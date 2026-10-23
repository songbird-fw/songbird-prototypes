// Package validator checks a configuration Tree in two layers, as the
// schema README prescribes for `songbird validate`:
//
//  1. structural: every object against the JSON Schema (package schema);
//  2. semantic: the cross-object rules JSON Schema cannot express — the
//     rule ids of songbird-fw/schema/validate.py, plus the constraints of
//     domain-model.md v0.7 (DM-xx) that the schema does not cover.
//
// It is pure: no I/O, no clock, no global state (components.md, rule 3).
// Every error names file, line, path and reason (R-020.3). Warnings are
// reported but never block a save or an Apply.
package validator

import (
	"fmt"
	"net/netip"
	"slices"
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
	// the predefined objects (DM-21) and an admin User. `songbird validate`
	// on a plain schema tree injects the predefined objects when they are
	// missing and does not ask for the others, like validate.py.
	Appliance bool
}

// Validate checks a configuration of the appliance. No entry with severity
// error means valid.
func Validate(t *m.Tree) []Error { return ValidateWith(t, Options{Appliance: true}) }

func ValidateWith(t *m.Tree, opt Options) []Error {
	if !opt.Appliance {
		t = withShipped(t)
	}
	x := &v{t: t, sys: t.Sys(), opt: opt}
	x.structural()
	x.names()
	x.system()
	x.colorsCheck()
	x.predefinedObjects()
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

// withShipped returns the tree with the predefined Addresses and Service
// injected when it does not contain them: they ship with the product, so a
// schema tree may omit them (validate.py, R-012.1). Colors are not injected:
// a tree without the palette simply does not use it.
func withShipped(t *m.Tree) *m.Tree {
	c := t
	for _, d := range predefined.Docs() {
		kind, meta := d.Header()
		if kind == "Color" {
			continue
		}
		if _, ok := t.Get(kind, meta.Name); ok {
			continue
		}
		if c == t {
			c = t.Clone()
		}
		_ = c.Put(d)
	}
	return c
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
		if ntp := s.NTP(); ntp != nil {
			x.listenOn("ntp", ntp.ListenOn)
		}
		if dns := s.DNS(); dns != nil {
			x.listenOn("dns", dns.ListenOn)
			if dns.DNS64 != nil && *dns.DNS64 && (s.NAT64 == nil || !s.NAT64.Enabled) {
				x.err("dns64-without-nat64", "R-018.6", "spec.systemServices.dns.dns64", "dns64 requires spec.nat64.enabled: true")
			}
			if dns.Mode == "recursive" && len(dns.Upstreams) > 0 {
				x.err("recursive-upstreams", "R-018.3", "spec.systemServices.dns.upstreams", "upstreams are meaningless in recursive mode")
			}
		}
		if s.Logging != nil && s.Logging.AnonymizeSourceIP != nil {
			for k, i := range s.Logging.AnonymizeSourceIP.Interfaces {
				x.ref(x.netNS, i, fmt.Sprintf("spec.logging.anonymizeSourceIp.interfaces[%d]", k), "Interface")
			}
		}
		if s.NAT64 != nil && s.NAT64.Enabled {
			x.warn("ipv6-deferred", "R-057", "spec.nat64.enabled", "NAT64 is deferred (domain model v0.7 is IPv4 only): stored, not applied")
		}
	}
}

// listenOn checks DM-45: the references exist (a Zone stands for its
// members) and an Interface appears at most once in the expanded list.
func (x *v) listenOn(service string, refs []string) {
	seen := map[string]string{}
	for k, r := range refs {
		path := fmt.Sprintf("spec.systemServices.%s.listenOn[%d]", service, k)
		if x.ref(x.netNS, r, path) == nil {
			continue
		}
		for _, i := range x.t.Side(r) {
			if other, dup := seen[i]; dup {
				x.err("listen-duplicate", "DM-45", path, "Interface %q is already in the list through %q", i, other)
			}
			seen[i] = r
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

// ---- predefined Addresses and Service (DM-21, R-012.1, R-010.1) ------------------------------

func (x *v) predefinedObjects() {
	for _, a := range x.t.Addresses {
		x.at(a)
		_, shipped := predefined.Address(a.Metadata.Name)
		switch {
		case shipped && !predefined.AddressMatches(a):
			want, _ := predefined.Address(a.Metadata.Name)
			x.err("predefined-immutable", "DM-21", "spec", "predefined Address %q must be exactly {subnet: %s, predefined: true}; only its metadata is editable", a.Metadata.Name, want.Spec.Subnet)
		case !shipped && a.Spec.Predefined:
			x.err("predefined-unknown", "DM-21", "spec.predefined", "%q is marked predefined but is not a shipped object (%v)", a.Metadata.Name, predefined.AddressNames())
		}
	}
	for _, s := range x.t.Services {
		x.at(s)
		_, shipped := predefined.Service(s.Metadata.Name)
		switch {
		case shipped && !predefined.ServiceMatches(s):
			x.err("predefined-immutable", "DM-21", "spec", "predefined Service %q must be exactly {protocols: [{protocol: any}], predefined: true}; only its metadata is editable", s.Metadata.Name)
		case !shipped && s.Spec.Predefined:
			x.err("predefined-unknown", "DM-21", "spec.predefined", "%q is marked predefined but is not a shipped object (%v)", s.Metadata.Name, predefined.ServiceNames())
		}
		if !s.Spec.Predefined {
			for k, p := range s.Spec.Protocols {
				if p.Protocol == "any" {
					x.err("any-protocol", "R-010.1", fmt.Sprintf("spec.protocols[%d].protocol", k), "protocol \"any\" is allowed only in the predefined Service any")
				}
			}
		}
	}
	if !x.opt.Appliance {
		return
	}
	for _, d := range predefined.Docs() { // DM-21: the shipped objects are present on an appliance
		kind, meta := d.Header()
		if kind == "Color" {
			continue // reported by colorsCheck
		}
		if _, ok := x.t.Get(kind, meta.Name); !ok {
			x.cur, x.file = nil, m.FilePath(kind, meta.Name)
			x.err("predefined-missing", "DM-21", "", "the predefined %s %q is missing", kind, meta.Name)
		}
	}
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
			x.warn("ipv6-deferred", "R-057", "spec.ipv6", "IPv6 is deferred (domain model v0.7 is IPv4 only): stored, not applied; IPv6 packets are dropped and counted")
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
				if !p.Addr().Is4() && !s.Predefined {
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
		}
	}
}

// ---- Groups (DM-15, DM-18): flat, members of the leaf kind only ---------------------------------

func (x *v) groups() {
	check := func(objs []m.Object[m.GroupSpec], ns map[string]m.Doc, leaf, dm string) {
		for _, g := range objs {
			x.at(g)
			seen := map[string]bool{}
			for k, mem := range g.Spec.Members {
				path := fmt.Sprintf("spec.members[%d]", k)
				if seen[mem] {
					x.err("duplicate-member", dm, path, "%q is listed twice", mem)
				}
				seen[mem] = true
				if o := x.ref(ns, mem, path, leaf); o == nil {
					continue
				}
			}
		}
	}
	check(x.t.AddressGroups, x.addrNS, "Address", "DM-15")
	check(x.t.ServiceGroups, x.svcNS, "Service", "DM-18")
	// DM-16 (all members bound to the same Interface) is not enforced: the
	// schema has no AddressGroup.interface, and with Zones a group may mix
	// members of several Interfaces of one Zone (the schema example grp-lan
	// does). DM-12 already applies to each expanded member. PoC finding.
}

// ---- Services (DM-17, DM-21) ----------------------------------------------------------------------------

func (x *v) services() {
	for _, s := range x.t.Services {
		x.at(s)
		seen := map[string]bool{}
		for k, p := range s.Spec.Protocols {
			if pr := p.Port; pr != nil && (pr.Lo < 1 || pr.Hi > 65535 || pr.Lo > pr.Hi) {
				x.err("port-range", "DM-17", fmt.Sprintf("spec.protocols[%d].port", k), "%s is not an ordered range within 1-65535", pr)
			}
			key := fmt.Sprintf("%s|%v|%v|%v|%v", p.Protocol, p.Port, deref(p.ICMPType), deref(p.ICMPCode), deref(p.ProtocolNumber))
			if seen[key] {
				x.err("duplicate-entry", "DM-17", fmt.Sprintf("spec.protocols[%d]", k), "this protocol entry is listed twice")
			}
			seen[key] = true
		}
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
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

// ---- DHCPv4: System.systemServices.dhcp (R-017 … R-017.6, DM-25 … DM-29) ----------------------------
// Rule ids are those of validate.py; DM-28 (no overlap between Interfaces)
// follows from DM-04 + dhcp-range-subnet and is checked for completeness.

func (x *v) dhcp() {
	if len(x.t.System) == 0 {
		return
	}
	o := x.t.System[0]
	x.at(o)
	sys := o.Spec
	listens := func(svc string, iface string) bool {
		var refs []string
		switch svc {
		case "dns":
			if d := sys.DNS(); d != nil {
				refs = d.ListenOn
			}
		case "ntp":
			if n := sys.NTP(); n != nil && n.Enabled {
				refs = n.ListenOn
			}
		}
		for _, r := range refs {
			if slices.Contains(x.t.Side(r), iface) {
				return true
			}
		}
		return false
	}
	type rng struct {
		a, b  netip.Addr
		iface string
	}
	var ranges []rng
	pools := sys.DHCP()
	names := make([]string, 0, len(pools))
	for n := range pools {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		pool := pools[name]
		fld := "spec.systemServices.dhcp." + name
		t := x.ref(x.netNS, name, fld, "Interface")
		if t == nil {
			continue
		}
		iface := t.(m.Interface)
		if v4 := iface.Spec.IPv4; v4 == nil || v4.Mode != "static" {
			x.err("dhcp-needs-static", "R-017.5", fld, "Interface %q has no static IPv4 address; a DHCP server needs one", name)
			continue
		}
		nets := prefixes(iface)
		own := map[netip.Addr]bool{}
		for _, p := range nets {
			own[p.Addr()] = true
		}
		inside := func(s string) bool {
			ip, err := netip.ParseAddr(s)
			if err != nil {
				return false
			}
			_, ok := inAny(ip, nets)
			return ok
		}
		if pool.LeaseTime != "" {
			if d, ok := m.ParseDuration(pool.LeaseTime); ok && d < 60e9 {
				x.err("dhcp-lease", "R-017.1", fld+".leaseTime", "lease time must be at least 1m")
			}
		}
		a, err1 := netip.ParseAddr(pool.Range.Start)
		b, err2 := netip.ParseAddr(pool.Range.End)
		if err1 != nil || err2 != nil {
			continue
		}
		if b.Less(a) {
			x.err("dhcp-range-order", "DM-25", fld+".range", "start must be <= end")
			continue
		}
		if !inside(pool.Range.Start) || !inside(pool.Range.End) {
			x.err("dhcp-range-subnet", "DM-25", fld+".range", "must lie inside the Interface subnet(s) %v", nets)
			continue
		}
		within := func(ip netip.Addr) bool { return !ip.Less(a) && !b.Less(ip) }
		for ip := range own {
			if within(ip) {
				x.err("dhcp-range-own-address", "DM-26", fld+".range", "contains the firewall's own address %s", ip)
			}
		}
		gw := netip.Addr{}
		if pool.Gateway != "" {
			if g, err := netip.ParseAddr(pool.Gateway); err == nil {
				gw = g
				if !inside(pool.Gateway) {
					x.err("dhcp-gateway-subnet", "DM-27", fld+".gateway", "not inside the Interface subnet")
				} else if within(g) {
					x.err("dhcp-range-own-address", "DM-26", fld+".range", "contains the gateway %s", g)
				}
			}
		}
		for _, r := range ranges {
			if !b.Less(r.a) && !r.b.Less(a) {
				x.err("dhcp-overlap", "DM-28", fld+".range", "overlaps the DHCP range of %q", r.iface)
			}
		}
		ranges = append(ranges, rng{a, b, name})
		macs, ips := map[string]bool{}, map[string]bool{}
		for k, r := range pool.Reservations {
			path := fmt.Sprintf("%s.reservations[%d]", fld, k)
			ip, err := netip.ParseAddr(r.Address)
			if err != nil {
				continue
			}
			if !inside(r.Address) || own[ip] || ip == gw {
				x.err("dhcp-reservation-subnet", "DM-29", path+".address", "%s must be inside the Interface subnet and differ from the Interface address and the gateway", r.Address)
			}
			mac := strings.ToLower(r.MAC)
			if macs[mac] {
				x.err("dhcp-reservation-dup", "DM-29", path+".mac", "duplicate MAC %s", r.MAC)
			}
			if ips[r.Address] {
				x.err("dhcp-reservation-dup", "DM-29", path+".address", "duplicate address %s", r.Address)
			}
			macs[mac], ips[r.Address] = true, true
		}
		if !pool.IsEnabled() {
			continue // a prepared pool advertises nothing yet
		}
		// R-017.6: advertising the firewall itself requires the service to listen there
		for _, c := range []struct {
			key, svc string
			list     []string
			implicit bool
		}{{"dnsServers", "dns", pool.DNSServers, pool.DNSServers == nil}, {"ntpServers", "ntp", pool.NTPServers, false}} {
			self := c.implicit
			for _, s := range c.list {
				if ip, err := netip.ParseAddr(s); err == nil && own[ip] {
					self = true
				}
			}
			if self && !listens(c.svc, name) {
				x.err("dhcp-advertises-silent-service", "R-017.6", fld+"."+c.key, "the pool advertises the firewall as %s server, but %q (or its Zone) is not in systemServices.%s.listenOn", strings.ToUpper(c.svc), name, c.svc)
			}
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
