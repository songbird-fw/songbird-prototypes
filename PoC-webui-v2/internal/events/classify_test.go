package events

import (
	"net/netip"
	"testing"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

func flow(in, src, dst, proto string, port int) Flow {
	return Flow{InIf: in, Src: netip.MustParseAddr(src), Dst: netip.MustParseAddr(dst), Proto: proto, Dport: port}
}

func TestClassify(t *testing.T) {
	tree := defaults.Tree()
	cases := []struct {
		name               string
		f                  Flow
		action, reason, rs string
		policy, xdst, xsrc string
	}{
		{"office to the Internet, masqueraded", flow("lan1", "192.168.10.60", "8.8.8.8", "tcp", 443), "accept", "policy", "internal-to-untrusted", "office-web-dns", "", "203.0.113.10"},
		{"first match wins: printers are blocked", flow("lan2", "192.168.20.105", "8.8.8.8", "tcp", 443), "drop", "policy", "internal-to-untrusted", "block-printers-internet", "", ""},
		{"guest SNAT to the pool", flow("guest", "192.168.30.60", "8.8.8.8", "udp", 53), "accept", "policy", "guest-to-untrusted", "guest-web", "", "203.0.113.20"},
		{"DNAT matched before translation", flow("wan", "91.1.2.3", "203.0.113.10", "tcp", 443), "accept", "policy", "untrusted-to-dmz", "publish-web", "172.16.40.10:8443", ""},
		{"no RuleSet for guest -> lan1", flow("guest", "192.168.30.60", "192.168.10.5", "tcp", 445), "drop", "no-ruleset", "", "", "", ""},
		{"Default Deny inside a RuleSet", flow("lan1", "192.168.10.60", "192.168.20.30", "tcp", 23), "drop", "default-deny", "lan1-to-lan2", "", "", ""},
		{"empty RuleSet denies", flow("dmz", "172.16.40.10", "8.8.8.8", "tcp", 443), "drop", "default-deny", "dmz-to-untrusted", "", "", ""},
		{"intra-zone RuleSet", flow("lan1", "192.168.10.12", "192.168.20.77", "tcp", 3389), "accept", "policy", "lan1-to-lan2", "admin-full", "", ""},
	}
	for _, c := range cases {
		v := Classify(tree, c.f)
		if v.Action != c.action || v.Reason != c.reason || v.RuleSet != c.rs || v.Policy != c.policy || v.XDst != c.xdst || v.XSrc != c.xsrc {
			t.Errorf("%s: got %+v", c.name, v)
		}
	}
}

func TestIntraZoneAllowAndShadowedDNAT(t *testing.T) {
	tree := defaults.Tree()
	for i := range tree.Zones {
		if tree.Zones[i].Metadata.Name == "internal" {
			tree.Zones[i].Spec.IntraZoneTraffic = "allow"
		}
	}
	tree.Delete("RuleSet", "lan1-to-lan2")
	if v := Classify(tree, flow("lan1", "192.168.10.60", "192.168.20.30", "tcp", 23)); v.Action != "accept" || v.Reason != "intra-zone" {
		t.Errorf("DM-39 allow: got %+v", v)
	}
	for i := range tree.RuleSets {
		if tree.RuleSets[i].Metadata.Name == "untrusted-to-dmz" {
			s := &tree.RuleSets[i].Spec
			s.Rules = append([]m.Rule{{Name: "all", Source: []string{"any"}, Destination: []string{"any"}, Service: []string{"any"}, Action: "accept"}}, s.Rules...)
		}
	}
	if v := Classify(tree, flow("wan", "91.1.2.3", "203.0.113.10", "tcp", 443)); v.Action != "drop" || v.Reason != "dnat-shadowed" {
		t.Errorf("shadowed dnat: got %+v", v)
	}
}
