// Package validator checks a configuration Tree in two layers, as the
// schema README prescribes for `songbird validate`:
//
//  1. structural: every object against the JSON Schema (package schema);
//  2. semantic: the cross-object rules JSON Schema cannot express — the
//     rule ids of songbird-yaml-schema/validate.py, plus the constraints of
//     domain-model.md v0.6 (DM-xx) that the schema does not cover.
//
// It is pure: no I/O, no clock, no global state (components.md, rule 3).
// Every error names file, line, path and reason (R-020.3). Warnings are
// reported but never block a save or an Apply.
package validator

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/predefined"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/schema"
)

// Data Plane capacity limits (ADR-0006, proposed values).
const (
	MaxPoliciesPerRuleSet = 1024  // DM-36
	MaxRuleSets           = 256   // ruleset_id u16, map sizes
	MaxDNATEntries        = 16384 // after port-range expansion
)

const (
	SevError   = "error"
	SevWarning = "warning"
)

type Error struct {
	File       string `json:"file"`
	Line       int    `json:"line,omitempty"`
	Path       string `json:"path"`
	Reason     string `json:"reason"`
	Rule       string `json:"rule"`                 // validate.py rule id, or a PoC id
	Constraint string `json:"constraint,omitempty"` // DM-xx / R-xxx it enforces
	Severity   string `json:"severity"`
}

func (e Error) String() string {
	loc := e.File
	if e.Line > 0 {
		loc = fmt.Sprintf("%s:%d", e.File, e.Line)
	}
	c := e.Rule
	if e.Constraint != "" {
		c += ", " + e.Constraint
	}
	return fmt.Sprintf("%s %s %s: %s: %s (%s)", strings.ToUpper(e.Severity), loc, e.Path, e.Rule, e.Reason, c)
}

// Errors keeps the blocking entries only.
func Errors(l []Error) []Error {
	var out []Error
	for _, e := range l {
		if e.Severity == SevError {
			out = append(out, e)
		}
	}
	return out
}

type located struct {
	Error
	obj m.Doc
}

type v struct {
	t    *m.Tree
	sys  m.SystemSpec
	opt  Options
	out  []located
	cur  m.Doc
	file string

	addrNS, svcNS, netNS, colors map[string]m.Doc
}

func (x *v) at(o m.Doc) {
	kind, meta := o.Header()
	x.cur, x.file = o, m.FilePath(kind, meta.Name)
}

func (x *v) add(sev, rule, dm, path, format string, args ...any) {
	x.out = append(x.out, located{Error{File: x.file, Path: path, Reason: fmt.Sprintf(format, args...),
		Rule: rule, Constraint: dm, Severity: sev}, x.cur})
}

func (x *v) err(rule, dm, path, format string, args ...any) {
	x.add(SevError, rule, dm, path, format, args...)
}

func (x *v) warn(rule, dm, path, format string, args ...any) {
	x.add(SevWarning, rule, dm, path, format, args...)
}

// Options select the checks that only apply to a running appliance.
type Options struct {
	// Appliance requires what a running appliance has: the System object,
	// the predefined palette (DM-21) and an admin User. `songbird validate`
	// on a plain schema tree omits them, like validate.py.
	Appliance bool
}

// Validate checks a configuration of the appliance. No entry with severity
// error means valid.
func Validate(t *m.Tree) []Error { return ValidateWith(t, Options{Appliance: true}) }

func ValidateWith(t *m.Tree, opt Options) []Error {
	x := &v{t: t, sys: t.Sys(), opt: opt}
	x.structural()
	x.names()
	x.system()
	x.colorsCheck()
	x.interfaces()
	x.zones()
	x.addresses()
	x.groups()
	x.services()
	x.rulesets()
	x.routes()
	x.dhcp()
	x.users()
	out := make([]Error, len(x.out))
	for i, l := range x.out {
		if l.obj != nil && l.Path != "" {
			l.Line = m.LineOfObject(l.obj, l.Path)
		}
		out[i] = l.Error
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity == SevError
		}
		return false
	})
	return out
}

// ---- layer 1 ------------------------------------------------------------------------

func (x *v) structural() {
	x.t.Each(func(o m.Doc) {
		x.at(o)
		for _, is := range schema.ValidateObject(o) {
			x.err("schema", "C-005", is.Path, "%s", is.Reason)
		}
	})
}

// ---- DM-01 names ------------------------------------------------------------------------

func (x *v) names() {
	byNS := map[string]map[string]m.Doc{}
	x.t.Each(func(o m.Doc) {
		kind, meta := o.Header()
		k, _ := m.KindByName(kind)
		x.at(o)
		ns := byNS[k.Namespace]
		if ns == nil {
			ns = map[string]m.Doc{}
			byNS[k.Namespace] = ns
		}
		if other, ok := ns[meta.Name]; ok {
			ok, _ := other.Header()
			x.err("unique-name", "DM-01", "metadata.name", "name %q is already used by %s/%s (shared %s namespace)", meta.Name, ok, meta.Name, k.Namespace)
			return
		}
		ns[meta.Name] = o
	})
	get := func(ns string) map[string]m.Doc {
		if byNS[ns] == nil {
			return map[string]m.Doc{}
		}
		return byNS[ns]
	}
	x.addrNS, x.svcNS, x.netNS, x.colors = get("address"), get("service"), get("network"), get("color")
}

// ref checks a reference (DM-02): it must exist in the namespace and be of
// an allowed kind. It returns the target, or nil.
func (x *v) ref(ns map[string]m.Doc, name, path string, kinds ...string) m.Doc {
	o, ok := ns[name]
	if !ok {
		x.err("dangling-ref", "DM-02", path, "references unknown object %q", name)
		return nil
	}
	kind, _ := o.Header()
	if len(kinds) > 0 && !contains(kinds, kind) {
		x.err("wrong-kind", "DM-02", path, "%q is a %s, expected %s", name, kind, strings.Join(kinds, " or "))
		return nil
	}
	return o
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// ---- System -------------------------------------------------------------------------------

func (x *v) system() {
	if len(x.t.System) == 0 {
		if !x.opt.Appliance {
			return // the schema asks for at most one System, not for one
		}
		x.cur, x.file = nil, m.FilePath("System", "system")
		x.err("system-missing", "schema §11", "", "the System object (global settings) is required")
		return
	}
	for i, o := range x.t.System {
		x.at(o)
		if i > 0 {
			x.err("system-singleton", "schema §11", "metadata.name", "only one System object is allowed")
		}
		s := o.Spec
		if s.Admin != nil {
			for k, a := range s.Admin.TrustedHosts {
				x.ref(x.addrNS, a, fmt.Sprintf("spec.admin.trustedHosts[%d]", k), "Address", "AddressGroup")
			}
		}
		if s.NTP != nil {
			for k, i := range s.NTP.ServeToInterfaces {
				x.ref(x.netNS, i, fmt.Sprintf("spec.ntp.serveToInterfaces[%d]", k))
			}
		}
		if s.DNS != nil {
			for k, i := range s.DNS.ServeToInterfaces {
				x.ref(x.netNS, i, fmt.Sprintf("spec.dns.serveToInterfaces[%d]", k))
			}
			if s.DNS.DNS64 != nil && *s.DNS.DNS64 && (s.NAT64 == nil || !s.NAT64.Enabled) {
				x.err("dns64-without-nat64", "R-018.6", "spec.dns.dns64", "dns64 requires spec.nat64.enabled: true")
			}
			if s.DNS.Mode == "recursive" && len(s.DNS.Upstreams) > 0 {
				x.err("recursive-upstreams", "R-018.3", "spec.dns.upstreams", "upstreams are meaningless in recursive mode")
			}
		}
		if s.Logging != nil && s.Logging.AnonymizeSourceIP != nil {
			for k, i := range s.Logging.AnonymizeSourceIP.Interfaces {
				x.ref(x.netNS, i, fmt.Sprintf("spec.logging.anonymizeSourceIp.interfaces[%d]", k), "Interface")
			}
		}
		if s.NAT64 != nil && s.NAT64.Enabled {
			x.warn("ipv6-deferred", "R-057", "spec.nat64.enabled", "NAT64 is deferred (domain model v0.6 is IPv4 only): stored, not applied")
		}
	}
}

// ---- Color (DM-33, R-055) ---------------------------------------------------------------------

func (x *v) colorsCheck() {
	seenHex := map[string]string{}
	present := map[string]bool{}
	for _, c := range x.t.Colors {
		x.at(c)
		present[c.Metadata.Name] = true
		h := strings.ToLower(c.Spec.Hex)
		if other, ok := seenHex[h]; ok {
			x.err("duplicate-hex", "DM-33", "spec.hex", "hex %s is already used by Color %q (two names for one colour defeat the classification)", c.Spec.Hex, other)
		}
		seenHex[h] = c.Metadata.Name
		want, shipped := predefined.ColorHex(c.Metadata.Name)
		switch {
		case c.Spec.Predefined && !shipped:
			x.err("predefined-unknown", "DM-21", "spec.predefined", "%q is not in the shipped palette %v", c.Metadata.Name, predefined.PaletteNames())
		case c.Spec.Predefined && !strings.EqualFold(want, c.Spec.Hex):
			x.err("predefined-hex", "R-055.1", "spec.hex", "predefined colour %q must keep hex %s; change its comment, or define a new Color", c.Metadata.Name, want)
		case !c.Spec.Predefined && shipped:
			x.err("predefined-shadow", "DM-33", "spec.predefined", "%q is a predefined colour name: set predefined: true or choose another name", c.Metadata.Name)
		}
	}
	for _, n := range predefined.PaletteNames() {
		if !present[n] && x.opt.Appliance { // DM-21: shipped objects are present on an appliance
			x.cur, x.file = nil, m.FilePath("Color", n)
			x.err("predefined-missing", "DM-21", "", "the predefined Color %q is missing", n)
		}
	}
	x.t.Each(func(o m.Doc) {
		if _, meta := o.Header(); meta.Color != "" {
			x.at(o)
			x.ref(x.colors, meta.Color, "metadata.color", "Color")
		}
	})
}

// ---- Interfaces (DM-03, DM-04) ------------------------------------------------------------------

func prefixes(i m.Interface) []netip.Prefix {
	var out []netip.Prefix
	for _, a := range i.Spec.StaticAddresses() {
		if p, err := netip.ParsePrefix(a); err == nil && p.Addr().Is4() {
			out = append(out, p)
		}
	}
	return out
}

func inAny(ip netip.Addr, ps []netip.Prefix) (netip.Prefix, bool) {
	for _, p := range ps {
		if p.Contains(ip) {
			return p, true
		}
	}
	return netip.Prefix{}, false
}

func (x *v) interfaces() {
	vlans, devices := map[string]string{}, map[string]string{}
	type owned struct {
		p     netip.Prefix
		iface string
	}
	var all []owned
	for _, i := range x.t.Interfaces {
		x.at(i)
		s := i.Spec
		switch s.Type {
		case "vlan":
			if p := x.ref(x.netNS, s.Parent, "spec.parent", "Interface"); p != nil {
				if p.(m.Interface).Spec.Type != "physical" {
					x.err("vlan-parent", "DM-03", "spec.parent", "the parent of a VLAN must be a physical Interface")
				}
			}
			key := fmt.Sprintf("%s/%d", s.Parent, s.VlanID)
			if other, ok := vlans[key]; ok {
				x.err("duplicate-vlan", "DM-03", "spec.vlanId", "VLAN %d on %q is already defined by %q", s.VlanID, s.Parent, other)
			}
			vlans[key] = i.Metadata.Name
		case "physical":
			if other, ok := devices[s.Device]; ok && s.Device != "" {
				x.err("duplicate-device", "DM-03", "spec.device", "device %q is already used by %q", s.Device, other)
			}
			devices[s.Device] = i.Metadata.Name
		}
		for k, a := range s.StaticAddresses() {
			p, err := netip.ParsePrefix(a)
			if err != nil {
				continue // schema error already
			}
			path := fmt.Sprintf("spec.ipv4.addresses[%d]", k)
			if p.Masked() == p && p.Bits() < 31 {
				x.err("interface-address", "R-020.4", path, "%s is a network address: an Interface needs a host address, e.g. %s", a, p.Addr().Next())
			}
			for _, o := range all {
				if o.p.Overlaps(p) {
					x.err("subnet-overlap", "DM-04", path, "%s overlaps %s of Interface %q", a, o.p, o.iface)
				}
			}
			all = append(all, owned{p.Masked(), i.Metadata.Name})
		}
		if s.IPv4 != nil && s.IPv4.Gateway != "" {
			if gw, err := netip.ParseAddr(s.IPv4.Gateway); err == nil {
				if _, ok := inAny(gw, prefixes(i)); !ok {
					x.err("gateway-off-link", "DM-22", "spec.ipv4.gateway", "gateway %s is not in a subnet of this Interface", gw)
				}
			}
		}
		if s.IPv6 != nil && s.IPv6.Mode != "none" {
			x.warn("ipv6-deferred", "R-057", "spec.ipv6", "IPv6 is deferred (domain model v0.6 is IPv4 only): stored, not applied; IPv6 packets are dropped and counted")
			if ra := s.IPv6.RouterAdvertisement; ra != nil && ra.DelegatedPrefixFrom != "" {
				if src := x.ref(x.netNS, ra.DelegatedPrefixFrom, "spec.ipv6.routerAdvertisement.delegatedPrefixFrom", "Interface"); src != nil {
					if v6 := src.(m.Interface).Spec.IPv6; v6 == nil || v6.Mode != "dhcpv6-pd" {
						x.err("pd-source", "R-017.4", "spec.ipv6.routerAdvertisement.delegatedPrefixFrom", "%q is not in dhcpv6-pd mode", ra.DelegatedPrefixFrom)
					}
				}
			}
		}
	}
}

// ---- Zones (DM-37) -------------------------------------------------------------------------------

func (x *v) zones() {
	zoneOf := map[string]string{}
	for _, z := range x.t.Zones {
		x.at(z)
		for k, i := range z.Spec.Interfaces {
			if x.ref(x.netNS, i, fmt.Sprintf("spec.interfaces[%d]", k), "Interface") == nil {
				continue
			}
			if other, ok := zoneOf[i]; ok {
				x.err("zone-overlap", "DM-37", fmt.Sprintf("spec.interfaces[%d]", k), "Interface %q already belongs to Zone %q", i, other)
			}
			zoneOf[i] = z.Metadata.Name
		}
	}
}

// ---- Addresses (DM-14) ------------------------------------------------------------------------------

func (x *v) addresses() {
	for _, a := range x.t.Addresses {
		x.at(a)
		s := a.Spec
		if s.Interface != "" {
			x.ref(x.netNS, s.Interface, "spec.interface", "Interface")
		}
		switch s.Form() {
		case "subnet":
			if p, err := netip.ParsePrefix(s.Subnet); err == nil {
				if !p.Addr().Is4() {
					x.warn("ipv6-deferred", "R-057", "spec.subnet", "IPv6 Addresses are stored but never match: IPv6 is deferred")
				} else if p.Masked() != p {
					x.warn("cidr-host-bits", "DM-14", "spec.subnet", "%s has host bits set: it is used as %s", s.Subnet, p.Masked())
				}
			}
		case "range":
			a1, err1 := netip.ParseAddr(s.Range.Start)
			a2, err2 := netip.ParseAddr(s.Range.End)
			switch {
			case err1 != nil || err2 != nil:
			case a1.Is4() != a2.Is4():
				x.err("range-family", "DM-14", "spec.range", "range start and end must be the same IP family")
			case a2.Less(a1):
				x.err("range-order", "DM-14", "spec.range", "range start %s is after range end %s", a1, a2)
			}
		case "fqdn":
			x.warn("dataplane-unsupported", "R-012", "spec.fqdn", "FQDN Addresses are not supported by the Data Plane yet (dataplane.md §7.3): Policies using it never match")
		case "mac":
			x.warn("dataplane-unsupported", "R-012", "spec.mac", "MAC Addresses are not supported by the Data Plane yet (dataplane.md §7.3): Policies using it never match")
		}
	}
}

// ---- Groups (R-011, R-013) ----------------------------------------------------------------------------

func (x *v) groups() {
	check := func(objs []m.Object[m.GroupSpec], ns map[string]m.Doc, kinds []string, req string) {
		graph := map[string][]string{}
		for _, g := range objs {
			x.at(g)
			graph[g.Metadata.Name] = g.Spec.Members
			for k, mem := range g.Spec.Members {
				x.ref(ns, mem, fmt.Sprintf("spec.members[%d]", k), kinds...)
			}
		}
		const white, grey, black = 0, 1, 2
		color := map[string]int{}
		var dfs func(n string, path []string)
		dfs = func(n string, path []string) {
			color[n] = grey
			for _, mem := range graph[n] {
				if _, isGroup := graph[mem]; !isGroup {
					continue
				}
				switch color[mem] {
				case grey:
					x.at(ns[n])
					x.err("group-cycle", req, "spec.members", "cycle: %s", strings.Join(append(path, mem), " -> "))
				case white:
					dfs(mem, append(path, mem))
				}
			}
			color[n] = black
		}
		for _, g := range objs {
			if color[g.Metadata.Name] == white {
				dfs(g.Metadata.Name, []string{g.Metadata.Name})
			}
		}
	}
	check(x.t.AddressGroups, x.addrNS, []string{"Address", "AddressGroup"}, "R-013")
	check(x.t.ServiceGroups, x.svcNS, []string{"Service", "ServiceGroup"}, "R-011")
}

// ---- Services (DM-17, DM-21) ----------------------------------------------------------------------------

func (x *v) services() {
	for _, s := range x.t.Services {
		x.at(s)
		for k, p := range s.Spec.Protocols {
			for f, pr := range map[string]*m.PortRange{"port": p.Port, "sourcePort": p.SourcePort} {
				if pr != nil && (pr.Lo < 1 || pr.Hi > 65535 || pr.Lo > pr.Hi) {
					x.err("port-range", "DM-17", fmt.Sprintf("spec.protocols[%d].%s", k, f), "%s is not an ordered range within 1-65535", pr)
				}
			}
		}
		if !predefined.ServiceMatches(s) {
			x.err("predefined-service", "DM-21", "spec.protocols", "%q is a predefined Service: its protocols cannot change (only metadata is editable)", s.Metadata.Name)
		}
	}
}

// ---- Routing (DM-22, DM-23) ----------------------------------------------------------------------------------

func (x *v) routes() {
	seen := map[string]string{}
	for _, r := range x.t.StaticRoutes {
		x.at(r)
		s := r.Spec
		if o := x.ref(x.netNS, s.Interface, "spec.interface", "Interface"); o != nil {
			if gw, err := netip.ParseAddr(s.Gateway); err == nil {
				if _, ok := inAny(gw, prefixes(o.(m.Interface))); !ok {
					x.err("gateway-off-link", "DM-22", "spec.gateway", "%s is not in a subnet of Interface %q", gw, s.Interface)
				}
			}
		}
		dst, err := netip.ParsePrefix(s.Destination)
		if err != nil {
			continue
		}
		d, p := s.Preference()
		key := fmt.Sprintf("%s|%d|%d", dst.Masked(), d, p)
		if other, ok := seen[key]; ok {
			x.err("route-tie", "DM-23", "spec.priority", "same destination, distance and priority as route %q", other)
		}
		seen[key] = r.Metadata.Name
	}
}

// ---- DHCPv4 (DM-24 … DM-29) -----------------------------------------------------------------------------------

func (x *v) dhcp() {
	type rng struct {
		a, b  netip.Addr
		iface string
	}
	var ranges []rng
	served := map[string]string{}
	for _, d := range x.t.DhcpServers {
		x.at(d)
		s := d.Spec
		o := x.ref(x.netNS, s.Interface, "spec.interface", "Interface")
		if o == nil {
			continue
		}
		iface := o.(m.Interface)
		if other, ok := served[s.Interface]; ok {
			x.err("dhcp-duplicate", "R-017.3", "spec.interface", "Interface %q is already served by DhcpServer %q", s.Interface, other)
		}
		served[s.Interface] = d.Metadata.Name
		if iface.Spec.EffectiveRole() == "external" {
			x.err("dhcp-role", "DM-24", "spec.interface", "no DHCP server on an external Interface")
		}
		if s.LeaseTime != "" {
			if v, ok := m.ParseDuration(s.LeaseTime); ok && v < 60e9 {
				x.err("dhcp-lease", "R-017.1", "spec.leaseTime", "lease time must be at least 1m")
			}
		}
		a, err1 := netip.ParseAddr(s.RangeStart)
		b, err2 := netip.ParseAddr(s.RangeEnd)
		if err1 != nil || err2 != nil {
			continue
		}
		if b.Less(a) {
			x.err("dhcp-range", "DM-25", "spec.rangeEnd", "rangeEnd is before rangeStart")
			continue
		}
		pfx, in := inAny(a, prefixes(iface))
		if !in || !pfx.Contains(b) {
			x.err("dhcp-range", "DM-25", "spec.rangeStart", "the range must lie in one subnet of Interface %q", s.Interface)
			continue
		}
		within := func(ip netip.Addr) bool { return !ip.Less(a) && !b.Less(ip) }
		if within(pfx.Addr()) {
			x.err("dhcp-range", "DM-26", "spec.rangeStart", "the range contains the Interface address %s", pfx.Addr())
		}
		gw := pfx.Addr()
		if s.Gateway != "" {
			if g, err := netip.ParseAddr(s.Gateway); err == nil {
				if !pfx.Contains(g) {
					x.err("dhcp-gateway", "DM-27", "spec.gateway", "gateway must lie in %s", pfx.Masked())
				} else {
					gw = g
					if within(g) {
						x.err("dhcp-range", "DM-26", "spec.gateway", "the range contains the gateway")
					}
				}
			}
		}
		for _, r := range ranges {
			if !b.Less(r.a) && !r.b.Less(a) {
				x.err("dhcp-overlap", "DM-28", "spec.rangeStart", "the range overlaps the DHCP range of %q", r.iface)
			}
		}
		ranges = append(ranges, rng{a, b, s.Interface})
		macs, ips := map[string]bool{}, map[string]bool{}
		for k, r := range s.Reservations {
			path := fmt.Sprintf("spec.reservations[%d]", k)
			ip, err := netip.ParseAddr(r.IP)
			if err != nil {
				continue
			}
			if !pfx.Contains(ip) || ip == pfx.Addr() || ip == gw {
				x.err("dhcp-reservation", "DM-29", path+".ip", "%s must be in %s and differ from the Interface address and the gateway", r.IP, pfx.Masked())
			}
			mac := strings.ToLower(r.MAC)
			if macs[mac] {
				x.err("dhcp-reservation", "DM-29", path+".mac", "MAC %s is reserved twice", r.MAC)
			}
			if ips[r.IP] {
				x.err("dhcp-reservation", "DM-29", path+".ip", "%s is reserved twice", r.IP)
			}
			macs[mac], ips[r.IP] = true, true
		}
	}
}

// ---- Users (R-043.9) -------------------------------------------------------------------------------------------

func (x *v) users() {
	if !x.opt.Appliance {
		return
	}
	for _, u := range x.t.Users {
		if u.Spec.Role == "admin" {
			return
		}
	}
	x.cur, x.file = nil, "users/"
	x.err("admin-required", "R-043.9", "", "at least one User with the admin Role is required")
}
