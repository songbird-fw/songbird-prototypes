package main

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
