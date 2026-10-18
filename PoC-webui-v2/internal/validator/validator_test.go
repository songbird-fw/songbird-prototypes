package validator_test

import (
	"fmt"
	"testing"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

func rs(t *m.Tree, name string) *m.RuleSet {
	for i := range t.RuleSets {
		if t.RuleSets[i].Metadata.Name == name {
			return &t.RuleSets[i]
		}
	}
	panic("no RuleSet " + name)
}

func iface(t *m.Tree, name string) *m.Interface {
	for i := range t.Interfaces {
		if t.Interfaces[i].Metadata.Name == name {
			return &t.Interfaces[i]
		}
	}
	panic("no Interface " + name)
}

func find[S any](l m.List[S], name string) *m.Object[S] {
	for i := range l {
		if l[i].Metadata.Name == name {
			return &l[i]
		}
	}
	panic("no object " + name)
}

func TestConstraints(t *testing.T) {
	base := defaults.Tree()
	cases := []struct {
		name, rule, constraint, file, severity string
		mutate                                 func(t *m.Tree)
	}{
		{"RuleSet with equal sides", "same-side", "DM-06", "firewall/rulesets/guest-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "guest-to-untrusted").Spec.To = "guest" }},
		{"second RuleSet for a pair", "duplicate-pair", "DM-07", "firewall/rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-dmz").Spec.To = "untrusted" }},
		{"Zone member referenced directly", "zone-member", "DM-38", "firewall/rulesets/guest-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "guest-to-untrusted").Spec.To = "wan" }},
		{"Address bound to another Interface", "address-binding", "DM-12", "firewall/rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[2].Source = []string{"web-01"} }},
		{"group member bound elsewhere", "address-binding", "DM-12", "firewall/rulesets/guest-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "guest-to-untrusted").Spec.Rules[0].Source = []string{"grp-backup"} }},
		{"dangling service", "dangling-ref", "DM-02", "firewall/rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[2].Service = []string{"nope"} }},
		{"dnat to a subnet", "dnat-single-host", "DM-13", "firewall/rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Nat.TranslateIPTo = "offsite-backup" }},
		{"dnat to any destination", "dnat-any-destination", "DM-40", "firewall/rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Destination = []string{"any"} }},
		{"dnat destination not of the incoming side", "dnat-destination", "DM-40", "firewall/rulesets/untrusted-to-dmz.yaml", validator.SevWarning,
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Destination = []string{"offsite-backup"} }},
		{"dnat translated address on the wrong side", "address-binding", "DM-12", "firewall/rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Nat.TranslateIPTo = "nas" }},
		{"overlapping dnat on the same Interface", "dnat-overlap", "DM-41", "firewall/rulesets/untrusted-to-guest.yaml", "", func(t *m.Tree) {
			r := rs(t, "untrusted-to-dmz").Spec.Rules[0]
			r.Name = "dup"
			_ = t.Put(m.RuleSet{APIVersion: m.APIVersion, Kind: "RuleSet", Metadata: m.Meta{Name: "untrusted-to-guest"},
				Spec: m.RuleSetSpec{From: "untrusted", To: "guest", Rules: []m.Rule{r}}})
			t.Sort()
		}},
		{"dnat shadowed by an earlier rule", "dnat-shadowed", "DM-41", "firewall/rulesets/untrusted-to-dmz.yaml", validator.SevWarning, func(t *m.Tree) {
			s := &rs(t, "untrusted-to-dmz").Spec
			s.Rules = append([]m.Rule{{Name: "all", Source: []string{"any"}, Destination: []string{"any"}, Service: []string{"any"}, Action: "accept"}}, s.Rules...)
		}},
		{"dnat on the management port", "management-port", "DM-35", "firewall/rulesets/untrusted-to-dmz.yaml", "", func(t *m.Tree) {
			iface(t, "wan").Spec.AllowAccess = []string{"ping", "https"}
		}},
		{"too many Policies in a RuleSet", "policy-limit", "DM-36", "firewall/rulesets/dmz-to-untrusted.yaml", "", func(t *m.Tree) {
			s := &rs(t, "dmz-to-untrusted").Spec
			for i := range validator.MaxPoliciesPerRuleSet + 1 {
				s.Rules = append(s.Rules, m.Rule{Name: fmt.Sprintf("r%d", i), Source: []string{"any"}, Destination: []string{"any"}, Service: []string{"any"}, Action: "drop"})
			}
		}},
		{"Interface in two Zones", "zone-overlap", "DM-37", "network/zones/untrusted.yaml", "",
			func(t *m.Tree) { find(t.Zones, "untrusted").Spec.Interfaces = []string{"wan", "lan1"} }},
		{"overlapping subnets", "subnet-overlap", "DM-04", "network/interfaces/lan1.yaml", "",
			func(t *m.Tree) { iface(t, "dmz").Spec.IPv4.Addresses = []string{"192.168.10.200/25"} }},
		{"VLAN on a VLAN", "vlan-parent", "DM-03", "network/interfaces/guest.yaml", "",
			func(t *m.Tree) { iface(t, "guest").Spec.Parent = "lan1" }},
		{"inverted range", "range-order", "DM-14", "objects/addresses/admin-pcs.yaml", "",
			func(t *m.Tree) { find(t.Addresses, "admin-pcs").Spec.Range.End = "192.168.10.1" }},
		{"inverted port range", "port-range", "DM-17", "objects/services/veeam.yaml", "",
			func(t *m.Tree) { find(t.Services, "veeam").Spec.Protocols[0].Port = &m.PortRange{Lo: 3300, Hi: 2500} }},
		{"predefined Service edited", "predefined-service", "DM-21", "objects/services/https.yaml", "",
			func(t *m.Tree) { find(t.Services, "https").Spec.Protocols[0].Port = &m.PortRange{Lo: 444, Hi: 444} }},
		{"route gateway off-link", "gateway-off-link", "DM-22", "network/routes/default.yaml", "",
			func(t *m.Tree) { find(t.StaticRoutes, "default").Spec.Gateway = "10.9.9.9" }},
		{"route tie", "route-tie", "DM-23", "network/routes/default-backup.yaml", "",
			func(t *m.Tree) { find(t.StaticRoutes, "default-backup").Spec.Distance = 10 }},
		{"DHCP on an external Interface", "dhcp-role", "DM-24", "network/dhcp/lan1.yaml", "",
			func(t *m.Tree) { find(t.DhcpServers, "lan1").Spec.Interface = "wan" }},
		{"DHCP range contains the Interface address", "dhcp-range", "DM-26", "network/dhcp/lan1.yaml", "",
			func(t *m.Tree) { find(t.DhcpServers, "lan1").Spec.RangeStart = "192.168.10.1" }},
		// DM-28 is implied by DM-04 + DM-25 (ranges lie in subnets that cannot
		// overlap): it only fires together with a subnet overlap.
		{"DHCP ranges overlap", "dhcp-overlap", "DM-28", "network/dhcp/lan2.yaml", "", func(t *m.Tree) {
			iface(t, "lan2").Spec.IPv4.Addresses = []string{"192.168.20.1/24", "192.168.10.2/24"}
			d := find(t.DhcpServers, "lan2")
			d.Spec.RangeStart, d.Spec.RangeEnd = "192.168.10.150", "192.168.10.160"
		}},
		{"reservation outside the subnet", "dhcp-reservation", "DM-29", "network/dhcp/lan1.yaml", "",
			func(t *m.Tree) { find(t.DhcpServers, "lan1").Spec.Reservations[0].IP = "192.168.20.5" }},
		{"duplicate hex", "duplicate-hex", "DM-33", "objects/colors/management.yaml", "",
			func(t *m.Tree) { find(t.Colors, "management").Spec.Hex = "#2e7d32" }},
		{"predefined hex repainted", "predefined-hex", "R-055.1", "objects/colors/green.yaml", "",
			func(t *m.Tree) { find(t.Colors, "green").Spec.Hex = "#00FF00" }},
		{"dangling color", "dangling-ref", "DM-02", "objects/addresses/nas.yaml", "",
			func(t *m.Tree) { find(t.Addresses, "nas").Metadata.Color = "magenta" }},
		{"group cycle", "group-cycle", "R-013", "objects/addressgroups/grp-office.yaml", "", func(t *m.Tree) {
			find(t.AddressGroups, "grp-office").Spec.Members = append(find(t.AddressGroups, "grp-office").Spec.Members, "grp-backup")
			find(t.AddressGroups, "grp-backup").Spec.Members = append(find(t.AddressGroups, "grp-backup").Spec.Members, "grp-office")
		}},
		{"no admin", "admin-required", "R-043.9", "users/", "",
			func(t *m.Tree) { find(t.Users, "admin").Spec.Role = "read-only" }},
		{"schema: drop with nat", "schema", "C-005", "firewall/rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[1].Action = "drop" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tree := base.Clone()
			c.mutate(tree)
			sev := c.severity
			if sev == "" {
				sev = validator.SevError
			}
			all := validator.Validate(tree)
			for _, e := range all {
				if e.Rule == c.rule && e.Constraint == c.constraint && e.File == c.file && e.Severity == sev {
					if e.Path != "" && e.Line == 0 {
						t.Errorf("no line number for %v", e)
					}
					return
				}
			}
			t.Errorf("expected %s %s/%s in %s, got:", sev, c.rule, c.constraint, c.file)
			for _, e := range all {
				t.Log("  ", e)
			}
		})
	}
}

// Intra-zone traffic is the one case where Zone members are referenced
// directly (DM-38): the factory default has lan1 -> lan2 inside internal.
func TestIntraZoneRuleSetIsAllowed(t *testing.T) {
	tree := defaults.Tree()
	if rs(tree, "lan1-to-lan2").Spec.From != "lan1" {
		t.Fatal("fixture changed")
	}
	for _, e := range validator.Errors(validator.Validate(tree)) {
		t.Error(e)
	}
}
