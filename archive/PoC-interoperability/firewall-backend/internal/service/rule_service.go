package service

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
