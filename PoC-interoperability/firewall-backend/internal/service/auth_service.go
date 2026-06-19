package service

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
