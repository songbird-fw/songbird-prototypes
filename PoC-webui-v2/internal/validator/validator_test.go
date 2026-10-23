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
		{"RuleSet from an Interface to itself", "same-interface", "DM-06", "rulesets/guest-to-untrusted.yaml", "",
			func(t *m.Tree) {
				rs(t, "guest-to-untrusted").Spec.From, rs(t, "guest-to-untrusted").Spec.To = "guest", "guest"
			}},
		{"RuleSet from a Zone to itself", "same-zone", "DM-06", "rulesets/internal-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-dmz").Spec.To = "internal" }},
		{"second RuleSet for a pair", "duplicate-pair", "DM-07", "rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-dmz").Spec.To = "untrusted" }},
		{"Address bound to another Interface", "address-binding", "DM-12", "rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[1].Source = []string{"web-01"} }},
		{"group member bound elsewhere", "address-binding", "DM-12", "rulesets/guest-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "guest-to-untrusted").Spec.Rules[0].Source = []string{"grp-backup"} }},
		{"member bound to the other member of the Zone", "address-binding", "DM-12", "rulesets/lan2-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "lan2-to-untrusted").Spec.Rules[0].Source = []string{"admin-pcs"} }},
		{"dangling service", "dangling-ref", "DM-02", "rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[1].Service = []string{"nope"} }},
		{"dnat to a subnet", "dnat-single-host", "DM-13", "rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Nat.TranslateIPTo = "offsite-backup" }},
		{"dnat to any destination", "dnat-any-destination", "DM-40", "rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Destination = []string{"any"} }},
		{"dnat port translation with service any", "dnat-any-service", "DM-40", "rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Service = []string{"any"} }},
		{"dnat destination not of the incoming side", "dnat-destination", "DM-40", "rulesets/untrusted-to-dmz.yaml", validator.SevWarning,
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Destination = []string{"offsite-backup"} }},
		{"dnat translated address on the wrong side", "address-binding", "DM-12", "rulesets/untrusted-to-dmz.yaml", "",
			func(t *m.Tree) { rs(t, "untrusted-to-dmz").Spec.Rules[0].Nat.TranslateIPTo = "nas" }},
		{"overlapping dnat on the same Interface", "dnat-overlap", "DM-41", "rulesets/wan-to-guest.yaml", "", func(t *m.Tree) {
			r := rs(t, "untrusted-to-dmz").Spec.Rules[0]
			r.Name = "dup"
			_ = t.Put(m.RuleSet{APIVersion: m.APIVersion, Kind: "RuleSet", Metadata: m.Meta{Name: "wan-to-guest"},
				Spec: m.RuleSetSpec{From: "wan", To: "guest", Rules: []m.Rule{r}}})
			t.Sort()
		}},
		{"dnat shadowed by an earlier rule", "dnat-shadowed", "DM-41", "rulesets/untrusted-to-dmz.yaml", validator.SevWarning, func(t *m.Tree) {
			s := &rs(t, "untrusted-to-dmz").Spec
			s.Rules = append([]m.Rule{{Name: "all", Source: []string{"any"}, Destination: []string{"any"}, Service: []string{"any"}, Action: "accept"}}, s.Rules...)
		}},
		{"dnat on the management port", "management-port", "DM-35", "rulesets/untrusted-to-dmz.yaml", "", func(t *m.Tree) {
			iface(t, "wan").Spec.AllowAccess = []string{"ping", "https"}
		}},
		{"FQDN Address in an enabled rule", "unsupported-address-kind", "DM-43", "rulesets/internal-to-untrusted.yaml", "", func(t *m.Tree) {
			_ = t.Put(m.Address{APIVersion: m.APIVersion, Kind: "Address", Metadata: m.Meta{Name: "example"}, Spec: m.AddressSpec{FQDN: "example.com"}})
			t.Sort()
			rs(t, "internal-to-untrusted").Spec.Rules[1].Destination = []string{"example"}
		}},
		{"MAC Address as destination", "mac-as-destination", "DM-44", "rulesets/internal-to-dmz.yaml", "", func(t *m.Tree) {
			_ = t.Put(m.Address{APIVersion: m.APIVersion, Kind: "Address", Metadata: m.Meta{Name: "laptop"}, Spec: m.AddressSpec{MAC: "34:29:8f:74:f4:6a"}})
			t.Sort()
			r := &rs(t, "internal-to-dmz").Spec.Rules[2] // disabled: DM-44 holds anyway
			r.Destination = []string{"laptop"}
		}},
		{"too many Policies in an effective list", "policy-limit", "DM-36", "rulesets/lan2-to-untrusted.yaml", "", func(t *m.Tree) {
			s := &rs(t, "internal-to-untrusted").Spec
			for i := range validator.MaxPoliciesPerRuleSet {
				s.Rules = append(s.Rules, m.Rule{Name: fmt.Sprintf("r%d", i), Source: []string{"any"}, Destination: []string{"any"}, Service: []string{"any"}, Action: "drop"})
			}
		}},
		{"Interface in two Zones", "zone-overlap", "DM-37", "zones/untrusted.yaml", "",
			func(t *m.Tree) { find(t.Zones, "untrusted").Spec.Interfaces = []string{"wan", "lan1"} }},
		{"overlapping subnets", "subnet-overlap", "DM-04", "interfaces/lan1.yaml", "",
			func(t *m.Tree) { iface(t, "dmz").Spec.IPv4.Addresses = []string{"192.168.10.200/25"} }},
		{"VLAN on a VLAN", "vlan-parent", "DM-03", "interfaces/guest.yaml", "",
			func(t *m.Tree) { iface(t, "guest").Spec.Parent = "lan1" }},
		{"inverted range", "range-order", "DM-14", "addresses/admin-pcs.yaml", "",
			func(t *m.Tree) { find(t.Addresses, "admin-pcs").Spec.Range.End = "192.168.10.1" }},
		{"inverted port range", "port-range", "DM-17", "services/veeam.yaml", "",
			func(t *m.Tree) { find(t.Services, "veeam").Spec.Protocols[0].Port = &m.PortRange{Lo: 3300, Hi: 2500} }},
		{"predefined Address edited", "predefined-immutable", "DM-21", "addresses/any.yaml", "",
			func(t *m.Tree) { find(t.Addresses, "any").Spec.Subnet = "10.0.0.0/8" }},
		{"predefined Service edited", "predefined-immutable", "DM-21", "services/any.yaml", "",
			func(t *m.Tree) {
				find(t.Services, "any").Spec.Protocols = []m.ProtocolEntry{{Protocol: "tcp", Port: &m.PortRange{Lo: 1, Hi: 65535}}}
			}},
		{"user Service marked predefined", "predefined-unknown", "DM-21", "services/https.yaml", "",
			func(t *m.Tree) { find(t.Services, "https").Spec.Predefined = true }},
		{"predefined Address deleted", "predefined-missing", "DM-21", "addresses/any6.yaml", "",
			func(t *m.Tree) { t.Delete("Address", "any6") }},
		{"nested group", "wrong-kind", "DM-02", "address-groups/grp-office.yaml", "", func(t *m.Tree) {
			g := find(t.AddressGroups, "grp-office")
			g.Spec.Members = append(g.Spec.Members, "grp-backup")
		}},
		{"route gateway off-link", "gateway-off-link", "DM-22", "static-routes/default.yaml", "",
			func(t *m.Tree) { find(t.StaticRoutes, "default").Spec.Gateway = "10.9.9.9" }},
		{"route tie", "route-tie", "DM-23", "static-routes/default-backup.yaml", "",
			func(t *m.Tree) { find(t.StaticRoutes, "default-backup").Spec.Distance = 10 }},
		{"DHCP on an Interface without static address", "dhcp-needs-static", "R-017.5", "system.yaml", "", func(t *m.Tree) {
			dhcp := t.System[0].Spec.SystemServices.DHCP
			dhcp["trunk"] = dhcp["lan2"]
		}},
		{"DHCP range contains the Interface address", "dhcp-range-own-address", "DM-26", "system.yaml", "", func(t *m.Tree) {
			p := t.System[0].Spec.SystemServices.DHCP["lan1"]
			p.Range.Start = "192.168.10.1"
			t.System[0].Spec.SystemServices.DHCP["lan1"] = p
		}},
		{"DHCP range outside the subnet", "dhcp-range-subnet", "DM-25", "system.yaml", "", func(t *m.Tree) {
			p := t.System[0].Spec.SystemServices.DHCP["guest"]
			p.Range.End = "192.168.31.10"
			t.System[0].Spec.SystemServices.DHCP["guest"] = p
		}},
		{"reservation outside the subnet", "dhcp-reservation-subnet", "DM-29", "system.yaml", "", func(t *m.Tree) {
			p := t.System[0].Spec.SystemServices.DHCP["lan1"]
			p.Reservations = []m.DhcpReservation{{MAC: "52:54:00:12:34:56", Address: "192.168.20.5"}}
			t.System[0].Spec.SystemServices.DHCP["lan1"] = p
		}},
		{"DHCP advertises a DNS server that does not listen", "dhcp-advertises-silent-service", "R-017.6", "system.yaml", "", func(t *m.Tree) {
			t.System[0].Spec.SystemServices.DNS.ListenOn = []string{"internal"} // guest pool omits dnsServers
		}},
		{"listenOn names an Interface twice", "listen-duplicate", "DM-45", "system.yaml", "", func(t *m.Tree) {
			t.System[0].Spec.SystemServices.NTP.ListenOn = []string{"internal", "lan1"}
		}},
		{"duplicate hex", "duplicate-hex", "DM-33", "colors/management.yaml", "",
			func(t *m.Tree) { find(t.Colors, "management").Spec.Hex = "#2e7d32" }},
		{"predefined hex repainted", "predefined-hex", "R-055.1", "colors/green.yaml", "",
			func(t *m.Tree) { find(t.Colors, "green").Spec.Hex = "#00FF00" }},
		{"dangling color", "dangling-ref", "DM-02", "addresses/nas.yaml", "",
			func(t *m.Tree) { find(t.Addresses, "nas").Metadata.Color = "magenta" }},
		{"no admin", "admin-required", "R-043.9", "users/", "",
			func(t *m.Tree) { find(t.Users, "admin").Spec.Role = "read-only" }},
		{"schema: drop with nat", "schema", "C-005", "rulesets/internal-to-untrusted.yaml", "",
			func(t *m.Tree) { rs(t, "internal-to-untrusted").Spec.Rules[0].Action = "drop" }},
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

// A member Interface may be used next to its Zone (DM-38, domain model
// 0.7): the factory default has lan2 -> untrusted next to internal ->
// untrusted, and lan1 -> lan2 between two members of internal.
func TestMemberNextToItsZoneIsAllowed(t *testing.T) {
	tree := defaults.Tree()
	if rs(tree, "lan2-to-untrusted").Spec.From != "lan2" || rs(tree, "lan1-to-lan2").Spec.From != "lan1" {
		t.Fatal("fixture changed")
	}
	for _, e := range validator.Errors(validator.Validate(tree)) {
		t.Error(e)
	}
}
