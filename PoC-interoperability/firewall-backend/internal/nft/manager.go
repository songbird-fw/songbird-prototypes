package nft

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
