package events

import (
	"net/netip"
	"slices"
	"strconv"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// A reference classifier of the forwarding path (dataplane.md §7), used by
// the simulated Data Plane so that the events it emits are the verdicts the
// applied configuration really gives: DNAT candidates on the original
// tuple, route lookup on the translated destination, then the first enabled
// Policy that matches across the applicable RuleSets of the (in, out)
// Interface pair, from the most specific (DM-42, R-014.4); then
// intraZoneTraffic (DM-39) or Default Deny.

type Flow struct {
	InIf  string
	Src   netip.Addr
	Dst   netip.Addr
	Proto string // tcp, udp, icmp
	Dport int
}

type Verdict struct {
	Action  string // accept, drop
	Reason  string // dataplane.md §11
	OutIf   string
	RuleSet string
	Policy  string
	Nat     *m.Nat
	XDst    string // ip:port after DNAT
	XSrc    string // ip after SNAT
}

func inAddresses(t *m.Tree, refs []string, ip netip.Addr) bool {
	for _, r := range refs {
		for _, a := range t.ExpandAddresses(r) {
			if addrContains(a, ip) {
				return true
			}
		}
	}
	return false
}

func addrContains(a m.Address, ip netip.Addr) bool {
	switch {
	case a.Spec.Subnet != "":
		p, err := netip.ParsePrefix(a.Spec.Subnet)
		return err == nil && p.Masked().Contains(ip)
	case a.Spec.Range != nil:
		s, err1 := netip.ParseAddr(a.Spec.Range.Start)
		e, err2 := netip.ParseAddr(a.Spec.Range.End)
		return err1 == nil && err2 == nil && !ip.Less(s) && !e.Less(ip)
	}
	return false // fqdn and mac never match (dataplane.md §7.3)
}

func inServices(t *m.Tree, refs []string, proto string, port int) bool {
	for _, r := range refs {
		for _, s := range t.ExpandServices(r) {
			for _, p := range s.Spec.Protocols {
				if p.Protocol == "any" {
					return true
				}
				if p.Protocol != proto {
					continue
				}
				if p.Port == nil || (p.Port.Lo <= port && port <= p.Port.Hi) {
					return true
				}
			}
		}
	}
	return false
}

func matches(t *m.Tree, r m.Rule, f Flow) bool {
	return r.IsEnabled() && inAddresses(t, r.Source, f.Src) && inAddresses(t, r.Destination, f.Dst) && inServices(t, r.Service, f.Proto, f.Dport)
}

// Route returns the outgoing Interface for a destination: the longest
// prefix among connected subnets and the active static routes (DM-23).
func Route(t *m.Tree, dst netip.Addr) string {
	best, bits := "", -1
	for _, i := range t.Interfaces {
		if !i.Spec.IsEnabled() {
			continue
		}
		for _, a := range i.Spec.StaticAddresses() {
			if p, err := netip.ParsePrefix(a); err == nil && p.Masked().Contains(dst) && p.Bits() > bits {
				best, bits = i.Metadata.Name, p.Bits()
			}
		}
	}
	type pref struct{ d, p int }
	var bestPref pref
	for _, r := range t.StaticRoutes {
		p, err := netip.ParsePrefix(r.Spec.Destination)
		if err != nil || !p.Masked().Contains(dst) {
			continue
		}
		d, pr := r.Spec.Preference()
		switch {
		case p.Bits() > bits:
			best, bits, bestPref = r.Spec.Interface, p.Bits(), pref{d, pr}
		case p.Bits() == bits && bits >= 0 && (d < bestPref.d || d == bestPref.d && pr < bestPref.p):
			best, bestPref = r.Spec.Interface, pref{d, pr}
		}
	}
	return best
}

// governed reports whether any RuleSet applies to the pair (DM-42).
func governed(t *m.Tree, in, out string) bool { return len(t.Applicable(in, out)) > 0 }

func firstIP(t *m.Tree, ref string) (netip.Addr, bool) {
	for _, a := range t.ExpandAddresses(ref) {
		switch {
		case a.Spec.Subnet != "":
			if p, err := netip.ParsePrefix(a.Spec.Subnet); err == nil {
				return p.Addr(), true
			}
		case a.Spec.Range != nil:
			if ip, err := netip.ParseAddr(a.Spec.Range.Start); err == nil {
				return ip, true
			}
		}
	}
	return netip.Addr{}, false
}

func interfaceIP(t *m.Tree, name string) (netip.Addr, bool) {
	if i, ok := t.Interface(name); ok {
		for _, a := range i.Spec.StaticAddresses() {
			if p, err := netip.ParsePrefix(a); err == nil {
				return p.Addr(), true
			}
		}
	}
	return netip.Addr{}, false
}

// Classify gives the verdict of a new forwarded Flow.
func Classify(t *m.Tree, f Flow) Verdict {
	// 1. DNAT candidates, on the original tuple (ADR-0010), from the most
	// specific incoming side: the Interface itself, then its Zone
	sides := []string{f.InIf}
	if z := t.ZoneOf(f.InIf); z != "" {
		sides = append(sides, z)
	}
	for _, from := range sides {
		for _, rs := range t.RuleSets {
			if rs.Spec.From != from {
				continue
			}
			for _, r := range rs.Spec.Rules {
				if r.Nat == nil || r.Nat.Type != "dnat" || !matches(t, r, f) {
					continue
				}
				to, ok := firstIP(t, r.Nat.TranslateIPTo)
				if !ok {
					continue
				}
				port := f.Dport
				if r.Nat.TranslateServiceTo != "" {
					if s, ok := t.Service(r.Nat.TranslateServiceTo); ok && len(s.Spec.Protocols) == 1 && s.Spec.Protocols[0].Port != nil {
						port = s.Spec.Protocols[0].Port.Lo
					}
				}
				v := Verdict{OutIf: Route(t, to), XDst: to.String() + ":" + strconv.Itoa(port)}
				seq := t.Applicable(f.InIf, v.OutIf)
				if !slices.ContainsFunc(seq, func(x m.RuleSet) bool { return x.Metadata.Name == rs.Metadata.Name }) {
					v.Action, v.Reason, v.RuleSet = "drop", "dnat-route-mismatch", rs.Metadata.Name
					return v
				}
				// the first enabled Policy that matches across the sequence decides
				for _, g := range seq {
					for _, e := range g.Spec.Rules {
						if !matches(t, e, f) {
							continue
						}
						v.RuleSet, v.Policy = g.Metadata.Name, e.Name
						switch {
						case g.Metadata.Name == rs.Metadata.Name && e.Name == r.Name:
							v.Action, v.Reason, v.Nat = "accept", "policy", r.Nat
						case e.Action == "drop":
							v.Action, v.Reason, v.XDst = "drop", "policy", ""
						default:
							v.Action, v.Reason, v.XDst = "drop", "dnat-shadowed", ""
						}
						return v
					}
				}
			}
		}
	}
	// 2. route lookup, applicable RuleSets in order, first match
	v := Verdict{OutIf: Route(t, f.Dst)}
	if v.OutIf == "" || v.OutIf == f.InIf {
		v.Action, v.Reason = "drop", "no-ruleset"
		return v
	}
	seq := t.Applicable(f.InIf, v.OutIf)
	for _, rs := range seq {
		for _, r := range rs.Spec.Rules {
			if r.Nat != nil && r.Nat.Type == "dnat" || !matches(t, r, f) {
				continue
			}
			v.RuleSet, v.Policy, v.Action, v.Reason = rs.Metadata.Name, r.Name, r.Action, "policy"
			if r.Action == "accept" && r.Nat != nil && r.Nat.Type == "snat" {
				v.Nat = r.Nat
				if r.Nat.TranslateIPTo != "" {
					if ip, ok := firstIP(t, r.Nat.TranslateIPTo); ok {
						v.XSrc = ip.String()
					}
				} else if ip, ok := interfaceIP(t, v.OutIf); ok {
					v.XSrc = ip.String() // masquerade (R-002)
				}
			}
			return v
		}
	}
	if z := t.ZoneOf(f.InIf); z != "" && z == t.ZoneOf(v.OutIf) {
		if zone, _ := t.Zone(z); zone.Spec.IntraZoneTraffic == "allow" {
			v.Action, v.Reason = "accept", "intra-zone" // DM-39
			return v
		}
	}
	if len(seq) == 0 {
		v.Action, v.Reason = "drop", "no-ruleset"
		return v
	}
	v.RuleSet = seq[len(seq)-1].Metadata.Name
	v.Action, v.Reason = "drop", "default-deny" // DM-09, R-033
	return v
}
