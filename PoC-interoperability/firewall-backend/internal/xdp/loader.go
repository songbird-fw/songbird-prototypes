package xdp

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
