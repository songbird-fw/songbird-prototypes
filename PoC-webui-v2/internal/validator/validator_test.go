package validator_test

import (
	"testing"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

func TestFactoryDefaultIsValid(t *testing.T) {
	if errs := validator.Validate(defaults.Tree()); len(errs) > 0 {
		for _, e := range errs {
			t.Error(e)
		}
	}
}

func expect(t *testing.T, tree *m.Tree, constraint, file string) {
	t.Helper()
	for _, e := range validator.Validate(tree) {
		if e.Constraint == constraint && e.File == file {
			return
		}
	}
	t.Errorf("expected %s in %s, got %v", constraint, file, validator.Validate(tree))
}

func TestConstraints(t *testing.T) {
	base := defaults.Tree()
	cases := []struct {
		name, dm, file string
		mutate         func(t *m.Tree)
	}{
		{"zone with same interfaces", "DM-06", "zones/LAN-WAN1.yaml", func(t *m.Tree) { t.Zones[0].Spec.DstInterface = "LAN" }},
		{"duplicate zone pair", "DM-07", "zones/GUEST-WAN1.yaml", func(t *m.Tree) { t.Zones[1].Spec.SrcInterface = "LAN" }},
		{"empty sources", "DM-08", "zones/LAN-WAN1.yaml", func(t *m.Tree) { t.Zones[0].Spec.Policies[0].Sources = nil }},
		{"dangling reference", "DM-02", "zones/LAN-WAN1.yaml", func(t *m.Tree) { t.Zones[0].Spec.Policies[1].Services = []string{"nope"} }},
		{"VIP as source", "DM-10", "zones/WAN1-DMZ.yaml", func(t *m.Tree) { t.Zones[2].Spec.Policies[0].Sources = []string{"vip-web-https"} }},
		{"address bound elsewhere", "DM-12", "zones/LAN-WAN1.yaml", func(t *m.Tree) { t.Zones[0].Spec.Policies[0].Sources = []string{"web-01"} }},
		{"nat pool missing", "DM-13", "zones/GUEST-WAN1.yaml", func(t *m.Tree) { t.Zones[1].Spec.Policies[0].Nat.Pool = "" }},
		{"inverted range", "DM-14", "addresses/admin-pcs.yaml", func(t *m.Tree) { t.Addresses[2].Spec.Value = "192.168.1.19-192.168.1.10" }},
		{"tcp without ports", "DM-17", "services/rsync.yaml", func(t *m.Tree) { t.Services[7].Spec.Ports = "" }},
		{"mixed-protocol group", "DM-18", "servicegroups/web.yaml", func(t *m.Tree) { t.ServiceGroups[0].Spec.Members = []string{"HTTP", "DNS"} }},
		{"overlapping subnets", "DM-04", "interfaces/DMZ.yaml", func(t *m.Tree) { t.Interfaces[3].Spec.Addresses = []string{"192.168.1.200/25"} }},
		{"dhcp on wan", "DM-24", "interfaces/WAN1.yaml", func(t *m.Tree) { t.Interfaces[0].Spec.Dhcp = t.Interfaces[1].Spec.Dhcp }},
		{"range contains interface IP", "DM-26", "interfaces/LAN.yaml", func(t *m.Tree) { t.Interfaces[1].Spec.Dhcp.RangeStart = "192.168.1.1" }},
		{"route gateway off-link", "DM-22", "staticroutes/default.yaml", func(t *m.Tree) { t.StaticRoutes[0].Spec.Gateway = "10.9.9.9" }},
		{"duplicate route preference", "DM-23", "staticroutes/default-backup.yaml", func(t *m.Tree) { t.StaticRoutes[1].Spec.Distance = 10 }},
		{"VIP on management port", "DM-35", "virtualips/vip-web-https.yaml", func(t *m.Tree) {
			t.Interfaces[0].Spec.AllowAccess = append(t.Interfaces[0].Spec.AllowAccess, "https")
		}},
		{"duplicate colour value", "DM-33", "colours/green.yaml", func(t *m.Tree) { t.Colours[1].Spec.Value = "#2A78D6" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := base.Clone()
			c.mutate(tree)
			expect(t, tree, c.dm, c.file)
		})
	}
}
