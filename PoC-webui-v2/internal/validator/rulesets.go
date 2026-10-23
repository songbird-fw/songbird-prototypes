package validator

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// RuleSets and their Policies: DM-06, DM-07, DM-08, DM-12, DM-13, DM-35,
// DM-36 (on the effective lists of DM-42), DM-40, DM-41, DM-43, DM-44, and
// the RuleSet rules of validate.py. Since domain model 0.7 a member
// Interface may be used next to its Zone (DM-38).

type dnatRule struct {
	rs      m.RuleSet
	index   int
	name    string
	in      []string // incoming Interfaces
	dst     addrSet
	svc     svcSet
	entries uint64
}

func (x *v) rulesets() {
	if len(x.t.RuleSets) > MaxRuleSets {
		x.at(x.t.RuleSets[MaxRuleSets])
		x.err("ruleset-limit", "ADR-0006", "metadata.name", "%d RuleSets exceed the Data Plane limit of %d", len(x.t.RuleSets), MaxRuleSets)
	}
	pairs := map[string]string{}
	var dnats []dnatRule
	var dnatEntries uint64
	for _, rs := range x.t.RuleSets {
		x.at(rs)
		s := rs.Spec
		from := x.ref(x.netNS, s.From, "spec.from")
		x.ref(x.netNS, s.To, "spec.to")
		if s.From == s.To {
			if k, _ := from.(m.Interface); from != nil && k.Kind == "Interface" {
				x.err("same-interface", "DM-06", "spec.to", "from and to cannot be the same Interface (use a Zone for intra-zone traffic)")
			} else {
				x.err("same-zone", "DM-06", "spec.to", "from and to cannot be the same Zone: traffic between its members is governed by Interface RuleSets and intraZoneTraffic (DM-39)")
			}
		}
		key := s.From + "\x00" + s.To
		if other, ok := pairs[key]; ok {
			x.err("duplicate-pair", "DM-07", "spec", "RuleSet %q already governs %s -> %s", other, s.From, s.To)
		}
		pairs[key] = rs.Metadata.Name
		fromIfs, toIfs := x.t.Side(s.From), x.t.Side(s.To)

		names := map[string]bool{}
		for i, r := range s.Rules {
			base := fmt.Sprintf("spec.rules[%d]", i)
			if names[r.Name] {
				x.err("duplicate-rule", "DM-01", base+".name", "rule name %q is repeated in this RuleSet", r.Name)
			}
			names[r.Name] = true
			dnat := r.Nat != nil && r.Nat.Type == "dnat"
			x.addressRefs(r.Source, base+".source", fromIfs, s.From, "enters from", false)
			if dnat {
				x.addressRefs(r.Destination, base+".destination", nil, "", "", true)
				x.dnatDestination(r.Destination, base+".destination", fromIfs, s.From)
			} else {
				x.addressRefs(r.Destination, base+".destination", toIfs, s.To, "leaves through", true)
			}
			for k, sv := range r.Service {
				x.ref(x.svcNS, sv, fmt.Sprintf("%s.service[%d]", base, k), "Service", "ServiceGroup")
			}
			if r.IsEnabled() {
				x.unsupported(r, base)
			}
			if r.Schedule != "" {
				x.warn("schedule-reserved", "schema", base+".schedule", "schedule is reserved for a future Schedule kind and is ignored")
			}
			if r.Nat != nil {
				x.nat(r, base, toIfs, s.To)
			}
			if dnat && r.IsEnabled() && r.Nat.TranslateIPTo != "" {
				d := dnatRule{rs: rs, index: i, name: r.Name, in: fromIfs, dst: x.addrSet(r.Destination), svc: x.svcSet(r.Service)}
				d.entries = max(1, d.dst.ivs.size()) * portCount(d.svc)
				dnatEntries += d.entries
				if dnatEntries > MaxDNATEntries {
					x.err("dnat-limit", "ADR-0006", base+".destination", "DNAT entries after port expansion reach %d, above the limit of %d", dnatEntries, MaxDNATEntries)
				}
				x.shadowed(rs, i, d)
				x.managementPort(d, base)
				dnats = append(dnats, d)
			}
		}
	}
	x.effectiveLimits()
	x.dnatOverlaps(dnats)
}

// effectiveLimits enforces DM-36 on the effective lists the Compiler builds
// (DM-42): for each ordered pair of distinct Interfaces, the enabled
// Policies of all its applicable RuleSets, from the most specific.
func (x *v) effectiveLimits() {
	reported := map[string]bool{}
	for _, in := range x.t.Interfaces {
		for _, out := range x.t.Interfaces {
			seq := x.t.Applicable(in.Metadata.Name, out.Metadata.Name)
			n := 0
			var names []string
			for _, rs := range seq {
				names = append(names, rs.Metadata.Name)
				for _, r := range rs.Spec.Rules {
					if r.IsEnabled() {
						n++
					}
				}
			}
			if n <= MaxPoliciesPerRuleSet || reported[strings.Join(names, ",")] {
				continue
			}
			reported[strings.Join(names, ",")] = true
			x.at(seq[0])
			x.err("policy-limit", "DM-36", "spec.rules", "%s -> %s is governed by %s: %d enabled Policies exceed the limit of %d per effective list (ADR-0006)",
				in.Metadata.Name, out.Metadata.Name, strings.Join(names, ", "), n, MaxPoliciesPerRuleSet)
		}
	}
}

// unsupported enforces DM-43 and DM-44 on an enabled Policy: no fqdn or mac
// Address (directly or through a group) until the Data Plane supports them
// (validate.py: unsupported-address-kind); mac only as source.
func (x *v) unsupported(r m.Rule, base string) {
	for _, side := range []struct {
		name string
		refs []string
	}{{"source", r.Source}, {"destination", r.Destination}} {
		for k, ref := range side.refs {
			for _, a := range x.t.ExpandAddresses(ref) {
				kind := map[bool]string{true: "fqdn", false: "mac"}[a.Spec.FQDN != ""]
				if a.Spec.FQDN == "" && a.Spec.MAC == "" {
					continue
				}
				x.err("unsupported-address-kind", "DM-43", fmt.Sprintf("%s.%s[%d]", base, side.name, k), "Address %q (%s) is not supported by the Data Plane yet (R-012.4): disable the rule or remove the Address", a.Metadata.Name, kind)
			}
		}
	}
}

// addressRefs checks the references of a source or destination list and the
// Interface binding of DM-12: an Address bound to an Interface is usable
// only on the side of the RuleSet that contains that Interface. side == nil
// skips the binding check.
func (x *v) addressRefs(refs []string, path string, side []string, sideName, verb string, destination bool) {
	for k, r := range refs {
		p := fmt.Sprintf("%s[%d]", path, k)
		if x.ref(x.addrNS, r, p, "Address", "AddressGroup") == nil {
			continue
		}
		for _, a := range x.t.ExpandAddresses(r) {
			if destination && a.Spec.MAC != "" {
				x.err("mac-as-destination", "DM-44", p, "MAC Address %q can only be used as source", a.Metadata.Name)
			}
			if side != nil && a.Spec.Interface != "" && !slices.Contains(side, a.Spec.Interface) {
				via := ""
				if a.Metadata.Name != r {
					via = fmt.Sprintf(" (member %q)", a.Metadata.Name)
				}
				x.err("address-binding", "DM-12", p, "%q%s is bound to Interface %q, but this RuleSet's traffic %s %s", r, via, a.Spec.Interface, verb, sideName)
			}
		}
	}
}

// dnatDestination: a dnat Policy matches the public address, before
// translation, which should be an address of the incoming side (DM-40).
func (x *v) dnatDestination(refs []string, path string, fromIfs []string, fromName string) {
	if x.t.AnyAddress(refs) {
		return
	}
	var own []netip.Prefix
	for _, n := range fromIfs {
		if i, ok := x.t.Interface(n); ok {
			own = append(own, prefixes(i)...)
		}
	}
	for k, r := range refs {
		for _, a := range x.t.ExpandAddresses(r) {
			if a.Spec.Interface != "" {
				if !slices.Contains(fromIfs, a.Spec.Interface) {
					x.warn("dnat-destination", "DM-40", fmt.Sprintf("%s[%d]", path, k), "%q is bound to %q, not to %s: traffic to it is not normally received from there", a.Metadata.Name, a.Spec.Interface, fromName)
				}
				continue
			}
			span, ok := addrInterval(a)
			if !ok {
				continue
			}
			inside := false
			for _, p := range own {
				pi, _ := addrInterval(m.Address{Spec: m.AddressSpec{Subnet: p.Masked().String()}})
				if pi.lo <= span.lo && span.hi <= pi.hi {
					inside = true
				}
			}
			if !inside && len(own) > 0 {
				x.warn("dnat-destination", "DM-40", fmt.Sprintf("%s[%d]", path, k), "%q is not an address of %s: traffic to it is not normally received by the firewall", a.Metadata.Name, fromName)
			}
		}
	}
}

func singleHostOrRange(a m.Address) bool {
	if a.Spec.Range != nil {
		return true
	}
	if p, err := netip.ParsePrefix(a.Spec.Subnet); err == nil {
		return p.Bits() == p.Addr().BitLen()
	}
	return false
}

// nat checks DM-13 and the NAT rules of validate.py.
func (x *v) nat(r m.Rule, base string, toIfs []string, toName string) {
	n := r.Nat
	var target *m.Address
	if n.TranslateIPTo != "" {
		if o := x.ref(x.addrNS, n.TranslateIPTo, base+".nat.translateIpTo", "Address"); o != nil {
			a := o.(m.Address)
			target = &a
			if a.Spec.FQDN != "" || a.Spec.MAC != "" {
				x.err("nat-address-kind", "DM-13", base+".nat.translateIpTo", "%q must be a subnet or range Address, not fqdn/mac", a.Metadata.Name)
				target = nil
			}
		}
	}
	if n.Type != "dnat" {
		return
	}
	if x.t.AnyAddress(r.Destination) {
		x.err("dnat-any-destination", "DM-40", base+".destination", "a dnat rule needs an explicit destination (the public address), not \"any\"")
	}
	if target != nil {
		if !singleHostOrRange(*target) {
			x.err("dnat-single-host", "DM-13", base+".nat.translateIpTo", "%q must be a single host (/32) or a range", target.Metadata.Name)
		}
		if target.Spec.Interface != "" && !slices.Contains(toIfs, target.Spec.Interface) {
			x.err("address-binding", "DM-12", base+".nat.translateIpTo", "%q is bound to Interface %q, but this RuleSet's traffic leaves through %s", target.Metadata.Name, target.Spec.Interface, toName)
		}
	}
	if n.TranslateServiceTo == "" {
		return
	}
	if x.t.AnyService(r.Service) {
		x.err("dnat-any-service", "DM-40", base+".service", "translateServiceTo requires an explicit service, not \"any\"")
	}
	o := x.ref(x.svcNS, n.TranslateServiceTo, base+".nat.translateServiceTo", "Service")
	if o == nil {
		return
	}
	ps := o.(m.Service).Spec.Protocols
	if len(ps) != 1 || !slices.Contains([]string{"tcp", "udp", "sctp"}, ps[0].Protocol) || ps[0].Port == nil || !ps[0].Port.Single() {
		x.err("dnat-service-shape", "DM-13", base+".nat.translateServiceTo", "%q must have exactly one tcp/udp/sctp entry with a single port", n.TranslateServiceTo)
		return
	}
	if protos := x.svcSet(r.Service).protocols(); len(protos) > 0 && !protos[ps[0].Protocol] {
		var have []string
		for p := range protos {
			have = append(have, p)
		}
		slices.Sort(have)
		x.err("dnat-protocol-mismatch", "DM-13", base+".nat.translateServiceTo", "translateServiceTo uses %s but the rule's service has %s", ps[0].Protocol, strings.Join(have, ", "))
	}
}

func portCount(s svcSet) uint64 {
	if s.any {
		return 65535
	}
	var n uint64
	for proto, p := range s.ports {
		if proto == "tcp" || proto == "udp" || proto == "sctp" {
			n += p.size()
		}
	}
	return max(n, 1)
}

// shadowed warns when an earlier enabled Policy of the same RuleSet matches
// all the traffic of a dnat Policy (DM-41): the Data Plane would drop it
// with reason dnat-shadowed.
func (x *v) shadowed(rs m.RuleSet, i int, d dnatRule) {
	r := rs.Spec.Rules[i]
	src := x.addrSet(r.Source)
	for j := 0; j < i; j++ {
		e := rs.Spec.Rules[j]
		if !e.IsEnabled() {
			continue
		}
		if x.addrSet(e.Source).covers(src) && x.addrSet(e.Destination).covers(d.dst) && x.svcSet(e.Service).covers(d.svc) {
			x.warn("dnat-shadowed", "DM-41", fmt.Sprintf("spec.rules[%d]", i), "rule %q (position %d) matches all the traffic of this dnat rule first: it would be dropped as dnat-shadowed", e.Name, j+1)
			return
		}
	}
}

// managementPort enforces DM-35: no dnat Policy translates the management
// HTTPS port of an Interface that allows https.
func (x *v) managementPort(d dnatRule, base string) {
	port := x.sys.HTTPSPort()
	if !d.svc.has("tcp", port) {
		return
	}
	for _, n := range d.in {
		i, ok := x.t.Interface(n)
		if !ok || !slices.Contains(i.Spec.AllowAccess, "https") {
			continue
		}
		var own ivs
		for _, p := range prefixes(i) {
			a := u32(p.Addr())
			own = append(own, iv{a, a})
		}
		if d.dst.any || d.dst.ivs.overlaps(own) {
			x.err("management-port", "DM-35", base+".service", "tcp/%d on an address of %q is the management port (https in its allowAccess, System.admin.httpsPort)", port, n)
		}
	}
}

// dnatOverlaps enforces DM-41: two dnat Policies whose RuleSets share an
// incoming Interface do not overlap on (protocol, destination, service).
func (x *v) dnatOverlaps(ds []dnatRule) {
	for j := range ds {
		for i := 0; i < j; i++ {
			a, b := ds[i], ds[j]
			shared := ""
			for _, n := range a.in {
				if slices.Contains(b.in, n) {
					shared = n
					break
				}
			}
			if shared == "" || !a.dst.overlaps(b.dst) || !a.svc.overlaps(b.svc) {
				continue
			}
			x.at(b.rs)
			x.err("dnat-overlap", "DM-41", fmt.Sprintf("spec.rules[%d]", b.index), "overlaps dnat rule %s/%s on Interface %q: the translation of a packet would be ambiguous", a.rs.Metadata.Name, a.name, shared)
		}
	}
}
