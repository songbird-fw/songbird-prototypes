from pathlib import Path
import tarfile
root = Path('output/firewall-backend')
# overwrite/add files for extended scaffold
files = {
'go.mod': '''module github.com/agostino/firewall-backend

go 1.23.0

require (
	github.com/cilium/ebpf v0.16.0
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/nftables v0.3.0
	github.com/jackc/pgx/v5 v5.7.1
	github.com/spf13/viper v1.19.0
	golang.org/x/crypto v0.31.0
)
''',
'README.md': '''# Go + XDP + nftables Firewall Backend Scaffold

Questo scaffold estende la base iniziale con:
- JWT access + refresh token
- persistenza PostgreSQL per utenti, refresh token e audit trail
- pattern ibrido XDP + nftables
- REST API Gin + SSE per la UI

## Pattern ibrido

- **XDP**: fast-path per drop immediato su blocklist IP volumetrica
- **nftables**: policy più ricche, set gestiti da userspace, casi stateful/NAT
- **PostgreSQL**: utenti, refresh token rotabili, audit log, regole persistenti
- **JWT**: access token breve + refresh token persistito e revocabile

## Flusso consigliato

1. Login → emetti access token + refresh token
2. Salva hash del refresh token in PostgreSQL
3. POST block-ip → persisti richiesta, aggiorna XDP map, aggiorna nftables set
4. Pubblica evento SSE verso la UI
5. Audit trail di ogni operazione amministrativa

## Endpoint principali

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `GET /api/v1/rules`
- `POST /api/v1/rules/block-ip`
- `DELETE /api/v1/rules/block-ip/:ip`
- `GET /api/v1/stats`
- `GET /api/v1/stream/events`

## Note

- Il refresh token va persistito come hash, non in chiaro.
- L'aggiornamento XDP e nftables va fatto nello stesso service applicativo per mantenere coerenza.
- In caso di errore su nftables, il service deve effettuare rollback logico e segnalare drift operativo.
''',
'Makefile': '''APP=firewall-backend

.PHONY: deps generate build run clean

deps:
	go mod tidy

generate:
	go generate ./...

build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/server

run: build
	./bin/$(APP)

clean:
	rm -rf bin
''',
'configs/config.yaml': '''server:
  address: ":8080"
security:
  jwtIssuer: "firewall-backend"
  accessTTL: "15m"
  refreshTTL: "168h"
  jwtSecret: "change-me-in-production"
database:
  dsn: "postgres://firewall:firewall@localhost:5432/firewall?sslmode=disable"
xdp:
  interface: "eth0"
  attachMode: "driver"
  programName: "xdp_firewall"
nft:
  tableFamily: "ip"
  tableName: "fwctl"
  setName: "blocked_ipv4"
''',
'cmd/server/main.go': '''package main

import (
	"context"
	"log"

	"github.com/agostino/firewall-backend/internal/api"
	"github.com/agostino/firewall-backend/internal/auth"
	"github.com/agostino/firewall-backend/internal/broker"
	"github.com/agostino/firewall-backend/internal/config"
	"github.com/agostino/firewall-backend/internal/db"
	"github.com/agostino/firewall-backend/internal/nft"
	"github.com/agostino/firewall-backend/internal/service"
	"github.com/agostino/firewall-backend/internal/xdp"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	br := broker.New()
	xdpMgr := xdp.NewManager(cfg.XDP.Interface, br)
	if err := xdpMgr.LoadAndAttach(); err != nil {
		log.Printf("warning: XDP not attached: %v", err)
	}
	defer xdpMgr.Close()

	nftMgr := nft.NewManager(cfg.NFT.TableName, cfg.NFT.SetName)
	jwtMgr := auth.NewJWTManager(cfg.Security.JWTSecret, cfg.Security.JWTIssuer, cfg.Security.AccessTTL, cfg.Security.RefreshTTL)
	repos := db.NewRepositories(pool)
	ruleSvc := service.NewRuleService(repos, xdpMgr, nftMgr, br)
	authSvc := service.NewAuthService(repos, jwtMgr)

	r := api.NewRouter(cfg, ruleSvc, authSvc, jwtMgr, br)
	if err := r.Run(cfg.Server.Address); err != nil {
		log.Fatalf("server: %v", err)
	}
}
''',
'internal/config/config.go': '''package config

import "github.com/spf13/viper"

type Config struct {
	Server struct {
		Address string
	}
	Security struct {
		JWTIssuer  string
		AccessTTL  string
		RefreshTTL string
		JWTSecret  string
	}
	Database struct {
		DSN string
	}
	XDP struct {
		Interface   string
		AttachMode  string
		ProgramName string
	}
	NFT struct {
		TableFamily string
		TableName   string
		SetName     string
	}
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
''',
'internal/model/rule.go': '''package model

type BlockIPRequest struct {
	IP string `json:"ip" binding:"required,ip"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}
''',
'internal/model/entities.go': '''package model

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type AuditEvent struct {
	ID        string
	ActorID   string
	Action    string
	Target    string
	Payload   string
	CreatedAt time.Time
}
''',
'internal/broker/broker.go': '''package broker

import "sync"

type Broker struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

func New() *Broker { return &Broker{clients: make(map[chan string]struct{})} }
func (b *Broker) Subscribe() chan string {
	ch := make(chan string, 32)
	b.mu.Lock(); b.clients[ch] = struct{}{}; b.mu.Unlock()
	return ch
}
func (b *Broker) Unsubscribe(ch chan string) {
	b.mu.Lock(); delete(b.clients, ch); close(ch); b.mu.Unlock()
}
func (b *Broker) Publish(msg string) {
	b.mu.RLock(); defer b.mu.RUnlock()
	for ch := range b.clients {
		select { case ch <- msg: default: }
	}
}
''',
'internal/db/db.go': '''package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}
''',
'internal/db/repos.go': '''package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/agostino/firewall-backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct{ Pool *pgxpool.Pool }
func NewRepositories(pool *pgxpool.Pool) *Repositories { return &Repositories{Pool: pool} }
func HashToken(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func (r *Repositories) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	row := r.Pool.QueryRow(ctx, `select id, username, password_hash, role, created_at from users where username=$1`, username)
	var u model.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil { return nil, err }
	return &u, nil
}

func (r *Repositories) StoreRefreshToken(ctx context.Context, userID, raw string, expiresAt time.Time) error {
	_, err := r.Pool.Exec(ctx, `insert into refresh_tokens (id, user_id, token_hash, expires_at, created_at) values ($1,$2,$3,$4,now())`, uuid.NewString(), userID, HashToken(raw), expiresAt)
	return err
}

func (r *Repositories) ValidateRefreshToken(ctx context.Context, raw string) (*model.RefreshToken, error) {
	row := r.Pool.QueryRow(ctx, `select id, user_id, token_hash, expires_at, revoked_at, created_at from refresh_tokens where token_hash=$1 and revoked_at is null`, HashToken(raw))
	var t model.RefreshToken
	if err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil { return nil, err }
	return &t, nil
}

func (r *Repositories) RevokeRefreshToken(ctx context.Context, raw string) error {
	_, err := r.Pool.Exec(ctx, `update refresh_tokens set revoked_at=now() where token_hash=$1 and revoked_at is null`, HashToken(raw))
	return err
}

func (r *Repositories) Audit(ctx context.Context, actorID, action, target, payload string) error {
	_, err := r.Pool.Exec(ctx, `insert into audit_events (id, actor_id, action, target, payload, created_at) values ($1,$2,$3,$4,$5,now())`, uuid.NewString(), actorID, action, target, payload)
	return err
}
''',
'internal/db/migrations.sql': '''create extension if not exists pgcrypto;

create table if not exists users (
  id uuid primary key,
  username text not null unique,
  password_hash text not null,
  role text not null default 'admin',
  created_at timestamptz not null default now()
);

create table if not exists refresh_tokens (
  id uuid primary key,
  user_id uuid not null references users(id) on delete cascade,
  token_hash text not null,
  expires_at timestamptz not null,
  revoked_at timestamptz,
  created_at timestamptz not null default now()
);
create index if not exists idx_refresh_tokens_hash on refresh_tokens(token_hash);

create table if not exists audit_events (
  id uuid primary key,
  actor_id text not null,
  action text not null,
  target text not null,
  payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);
''',
'internal/auth/jwt.go': '''package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret, issuer, accessTTL, refreshTTL string) *JWTManager {
	a, _ := time.ParseDuration(accessTTL)
	r, _ := time.ParseDuration(refreshTTL)
	return &JWTManager{secret: []byte(secret), issuer: issuer, accessTTL: a, refreshTTL: r}
}

func (m *JWTManager) GenerateAccessToken(userID, role string) (string, time.Time, error) {
	exp := time.Now().Add(m.accessTTL)
	claims := jwt.MapClaims{"sub": userID, "role": role, "iss": m.issuer, "exp": exp.Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(m.secret)
	return s, exp, err
}

func (m *JWTManager) GenerateRefreshToken() (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", time.Time{}, err }
	return base64.RawURLEncoding.EncodeToString(b), time.Now().Add(m.refreshTTL), nil
}

func (m *JWTManager) ParseAccessToken(raw string) (*jwt.Token, error) {
	return jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) { return m.secret, nil })
}
''',
'internal/auth/middleware.go': '''package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireJWT(m *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(h, "Bearer ")
		tok, err := m.ParseAccessToken(raw)
		if err != nil || !tok.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := tok.Claims.(map[string]interface{})
		c.Set("userID", claims["sub"])
		c.Set("role", claims["role"])
		c.Next()
	}
}
''',
'internal/service/auth_service.go': '''package service

import (
	"context"
	"errors"

	"github.com/agostino/firewall-backend/internal/auth"
	"github.com/agostino/firewall-backend/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	repos *db.Repositories
	jwt   *auth.JWTManager
}

func NewAuthService(repos *db.Repositories, jwt *auth.JWTManager) *AuthService { return &AuthService{repos: repos, jwt: jwt} }

func (s *AuthService) Login(ctx context.Context, username, password string) (string, string, error) {
	u, err := s.repos.FindUserByUsername(ctx, username)
	if err != nil { return "", "", err }
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil { return "", "", errors.New("invalid credentials") }
	access, _, err := s.jwt.GenerateAccessToken(u.ID, u.Role)
	if err != nil { return "", "", err }
	refresh, exp, err := s.jwt.GenerateRefreshToken()
	if err != nil { return "", "", err }
	if err := s.repos.StoreRefreshToken(ctx, u.ID, refresh, exp); err != nil { return "", "", err }
	return access, refresh, nil
}

func (s *AuthService) Refresh(ctx context.Context, raw string) (string, string, error) {
	tok, err := s.repos.ValidateRefreshToken(ctx, raw)
	if err != nil { return "", "", err }
	if err := s.repos.RevokeRefreshToken(ctx, raw); err != nil { return "", "", err }
	access, _, err := s.jwt.GenerateAccessToken(tok.UserID, "admin")
	if err != nil { return "", "", err }
	refresh, exp, err := s.jwt.GenerateRefreshToken()
	if err != nil { return "", "", err }
	if err := s.repos.StoreRefreshToken(ctx, tok.UserID, refresh, exp); err != nil { return "", "", err }
	return access, refresh, nil
}

func (s *AuthService) Logout(ctx context.Context, raw string) error { return s.repos.RevokeRefreshToken(ctx, raw) }
''',
'internal/service/rule_service.go': '''package service

import (
	"context"
	"fmt"

	"github.com/agostino/firewall-backend/internal/broker"
	"github.com/agostino/firewall-backend/internal/db"
	"github.com/agostino/firewall-backend/internal/nft"
	"github.com/agostino/firewall-backend/internal/xdp"
)

type RuleService struct {
	repos *db.Repositories
	xdp   *xdp.Manager
	nft   *nft.Manager
	br    *broker.Broker
}

func NewRuleService(repos *db.Repositories, xdp *xdp.Manager, nft *nft.Manager, br *broker.Broker) *RuleService {
	return &RuleService{repos: repos, xdp: xdp, nft: nft, br: br}
}

func (s *RuleService) BlockIP(ctx context.Context, actorID, ip string) error {
	if err := s.xdp.BlockIP(ip); err != nil { return err }
	if err := s.nft.EnsureBlockedIP(ip); err != nil { return err }
	_ = s.repos.Audit(ctx, actorID, "block_ip", ip, fmt.Sprintf(`{"ip":"%s"}`, ip))
	s.br.Publish(fmt.Sprintf(`{"event":"block_ip","ip":"%s"}`, ip))
	return nil
}

func (s *RuleService) UnblockIP(ctx context.Context, actorID, ip string) error {
	if err := s.xdp.UnblockIP(ip); err != nil { return err }
	if err := s.nft.RemoveBlockedIP(ip); err != nil { return err }
	_ = s.repos.Audit(ctx, actorID, "unblock_ip", ip, fmt.Sprintf(`{"ip":"%s"}`, ip))
	s.br.Publish(fmt.Sprintf(`{"event":"unblock_ip","ip":"%s"}`, ip))
	return nil
}

func (s *RuleService) ListBlockedIPs(ctx context.Context) ([]string, error) { return s.xdp.ListBlockedIPs() }
func (s *RuleService) Stats(ctx context.Context) (map[string]uint64, error) { return s.xdp.ReadStats() }
''',
'internal/nft/manager.go': '''package nft

import (
	"net"

	"github.com/google/nftables"
)

type Manager struct {
	tableName string
	setName   string
}

func NewManager(tableName, setName string) *Manager { return &Manager{tableName: tableName, setName: setName} }

func (m *Manager) EnsureBlockedIP(ip string) error {
	conn := &nftables.Conn{}
	table := conn.AddTable(&nftables.Table{Family: nftables.TableFamilyIPv4, Name: m.tableName})
	set := &nftables.Set{Table: table, Name: m.setName, KeyType: nftables.TypeIPAddr}
	_ = conn.AddSet(set, []nftables.SetElement{{Key: net.ParseIP(ip).To4()}})
	return conn.Flush()
}

func (m *Manager) RemoveBlockedIP(ip string) error {
	conn := &nftables.Conn{}
	table := &nftables.Table{Family: nftables.TableFamilyIPv4, Name: m.tableName}
	set := &nftables.Set{Table: table, Name: m.setName, KeyType: nftables.TypeIPAddr}
	if err := conn.SetDeleteElements(set, []nftables.SetElement{{Key: net.ParseIP(ip).To4()}}); err != nil { return err }
	return conn.Flush()
}
''',
'internal/api/router.go': '''package api

import (
	"io"
	"net/http"

	"github.com/agostino/firewall-backend/internal/auth"
	"github.com/agostino/firewall-backend/internal/broker"
	"github.com/agostino/firewall-backend/internal/config"
	"github.com/agostino/firewall-backend/internal/model"
	"github.com/agostino/firewall-backend/internal/service"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, rules *service.RuleService, authSvc *service.AuthService, jwtMgr *auth.JWTManager, br *broker.Broker) *gin.Engine {
	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "iface": cfg.XDP.Interface}) })

	v1 := r.Group("/api/v1")
	{
		authg := v1.Group("/auth")
		authg.POST("/login", func(c *gin.Context) {
			var req model.LoginRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			access, refresh, err := authSvc.Login(c.Request.Context(), req.Username, req.Password)
			if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"accessToken": access, "refreshToken": refresh})
		})
		authg.POST("/refresh", func(c *gin.Context) {
			var req model.RefreshRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			access, refresh, err := authSvc.Refresh(c.Request.Context(), req.RefreshToken)
			if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"accessToken": access, "refreshToken": refresh})
		})
		authg.POST("/logout", func(c *gin.Context) {
			var req model.RefreshRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			if err := authSvc.Logout(c.Request.Context(), req.RefreshToken); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
		})

		protected := v1.Group("")
		protected.Use(auth.RequireJWT(jwtMgr))
		protected.GET("/rules", func(c *gin.Context) {
			ips, err := rules.ListBlockedIPs(c.Request.Context())
			if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"blocked_ips": ips})
		})
		protected.POST("/rules/block-ip", func(c *gin.Context) {
			var req model.BlockIPRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			actorID, _ := c.Get("userID")
			if err := rules.BlockIP(c.Request.Context(), actorID.(string), req.IP); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusCreated, gin.H{"status": "blocked", "ip": req.IP})
		})
		protected.DELETE("/rules/block-ip/:ip", func(c *gin.Context) {
			actorID, _ := c.Get("userID")
			ip := c.Param("ip")
			if err := rules.UnblockIP(c.Request.Context(), actorID.(string), ip); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"status": "unblocked", "ip": ip})
		})
		protected.GET("/stats", func(c *gin.Context) {
			stats, err := rules.Stats(c.Request.Context())
			if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, stats)
		})
		protected.GET("/stream/events", func(c *gin.Context) {
			ch := br.Subscribe(); defer br.Unsubscribe(ch)
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("X-Accel-Buffering", "no")
			c.Stream(func(w io.Writer) bool {
				select {
				case msg, ok := <-ch:
					if !ok { return false }
					c.SSEvent("message", msg)
					return true
				case <-c.Request.Context().Done():
					return false
				}
			})
		})
	}
	return r
}
''',
'internal/xdp/loader.go': '''package xdp

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/agostino/firewall-backend/internal/broker"
	"github.com/cilium/ebpf/link"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target amd64 Firewall ../../bpf/firewall.bpf.c -- -I../../bpf/headers

type Manager struct {
	ifaceName string
	broker    *broker.Broker
	objs      FirewallObjects
	link      link.Link
}

func NewManager(iface string, br *broker.Broker) *Manager { return &Manager{ifaceName: iface, broker: br} }
func (m *Manager) LoadAndAttach() error {
	if err := loadFirewallObjects(&m.objs, nil); err != nil { return fmt.Errorf("load objects: %w", err) }
	iface, err := net.InterfaceByName(m.ifaceName)
	if err != nil { return fmt.Errorf("interface: %w", err) }
	lnk, err := link.AttachXDP(link.XDPOptions{Program: m.objs.XdpFirewall, Interface: iface.Index})
	if err != nil { return fmt.Errorf("attach xdp: %w", err) }
	m.link = lnk
	if m.broker != nil { m.broker.Publish(fmt.Sprintf(`{"event":"xdp_attached","iface":"%s"}`, m.ifaceName)) }
	return nil
}
func (m *Manager) Close() { if m.link != nil { _ = m.link.Close() }; m.objs.Close() }
func ipToKey(ipstr string) (uint32, error) { ip := net.ParseIP(ipstr).To4(); if ip == nil { return 0, fmt.Errorf("invalid ipv4: %s", ipstr) }; return binary.BigEndian.Uint32(ip), nil }
func (m *Manager) BlockIP(ip string) error { key, err := ipToKey(ip); if err != nil { return err }; val := uint8(1); return m.objs.BlockedIps.Put(key, val) }
func (m *Manager) UnblockIP(ip string) error { key, err := ipToKey(ip); if err != nil { return err }; return m.objs.BlockedIps.Delete(key) }
func (m *Manager) ListBlockedIPs() ([]string, error) { var ips []string; var key uint32; var val uint8; it := m.objs.BlockedIps.Iterate(); for it.Next(&key, &val) { buf := make([]byte, 4); binary.BigEndian.PutUint32(buf, key); ips = append(ips, net.IP(buf).String()) }; return ips, it.Err() }
func (m *Manager) ReadStats() (map[string]uint64, error) { stats := map[string]uint64{"passed": 0, "dropped": 0}; var key uint32; var value uint64; it := m.objs.Stats.Iterate(); for it.Next(&key, &value) { switch key { case 0: stats["passed"] = value; case 1: stats["dropped"] = value } }; return stats, it.Err() }
''',
'bpf/firewall.bpf.c': '''#include "vmlinux.h"
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65535);
    __type(key, __u32);
    __type(value, __u8);
} blocked_ips SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 2);
    __type(key, __u32);
    __type(value, __u64);
} stats SEC(".maps");

static __always_inline void incr_stat(__u32 key) {
    __u64 *value = bpf_map_lookup_elem(&stats, &key);
    if (value) {
        __sync_fetch_and_add(value, 1);
    }
}

SEC("xdp")
int xdp_firewall(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_PASS;
    if (eth->h_proto != bpf_htons(ETH_P_IP)) { incr_stat(0); return XDP_PASS; }
    struct iphdr *iph = data + sizeof(*eth);
    if ((void *)(iph + 1) > data_end) return XDP_PASS;
    __u32 src = iph->saddr;
    __u8 *blocked = bpf_map_lookup_elem(&blocked_ips, &src);
    if (blocked) { incr_stat(1); return XDP_DROP; }
    incr_stat(0);
    return XDP_PASS;
}
''',
'deployments/docker-compose.yaml': '''services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: firewall
      POSTGRES_PASSWORD: firewall
      POSTGRES_DB: firewall
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata: {}
'''
}
for rel, content in files.items():
    p = root / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content)
archive = Path('output/firewall-backend-extended.tar.gz')
with tarfile.open(archive, 'w:gz') as tar:
    tar.add(root, arcname='firewall-backend')
print(str(archive))