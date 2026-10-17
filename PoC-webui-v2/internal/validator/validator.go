// Package validator checks a configuration Tree against the domain-model
// constraints (DM-xx). It is pure: no I/O, no clock, no global state
// (components.md, dependency rule 3). Every error names file, path and
// reason (R-020.3).
package validator

import (
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// MaxPoliciesPerZone is the Data Plane per-Zone limit (ADR-0006, DM-36).
// The real value is still open; the PoC uses a small one.
const MaxPoliciesPerZone = 64

type Error struct {
	File       string `json:"file"`
	Path       string `json:"path"`
	Reason     string `json:"reason"`
	Constraint string `json:"constraint,omitempty"`
}

func (e Error) String() string {
	return fmt.Sprintf("%s: %s: %s (%s)", e.File, e.Path, e.Reason, e.Constraint)
}

type v struct {
	t    *m.Tree
	errs []Error
	file string
}

func (x *v) add(dm, path, format string, args ...any) {
	x.errs = append(x.errs, Error{File: x.file, Path: path, Reason: fmt.Sprintf(format, args...), Constraint: dm})
}

func (x *v) at(kind, name string) { x.file = m.FilePath(kind, name) }

// InterfaceAddressesSuffix makes the reserved name of the derived
// InterfaceAddresses object, e.g. "LAN addresses".
const InterfaceAddressesSuffix = " addresses"

// Validate returns every violation found; an empty list means valid.
func Validate(t *m.Tree) []Error {
	x := &v{t: t}
	x.names()
	x.colours()
	x.interfaces()
	x.objects()
	x.zones()
	x.routes()
	x.dhcp()
	x.users()
	return x.errs
}

// ---- helpers -----------------------------------------------------------------

func (x *v) iface(name string) (m.Interface, bool) {
	for _, i := range x.t.Interfaces {
		if i.Metadata.Name == name {
			return i, true
		}
	}
	return m.Interface{}, false
}

func ifacePrefixes(i m.Interface) []netip.Prefix {
	var out []netip.Prefix
	for _, a := range i.Spec.Addresses {
		if p, err := netip.ParsePrefix(a); err == nil {
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

func parseV4(s string) (netip.Addr, bool) {
	a, err := netip.ParseAddr(s)
	return a, err == nil && a.Is4()
}

var macRe = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)
var colourRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
var fqdnRe = regexp.MustCompile(`^([A-Za-z0-9-]{1,63}\.)+[A-Za-z]{2,63}$`)

// parsePorts parses "443" or "1000-2000".
func parsePorts(s string) (lo, hi int, ok bool) {
	a, b, isRange := strings.Cut(s, "-")
	lo, err1 := strconv.Atoi(strings.TrimSpace(a))
	hi = lo
	var err2 error
	if isRange {
		hi, err2 = strconv.Atoi(strings.TrimSpace(b))
	}
	if err1 != nil || err2 != nil || lo < 1 || hi > 65535 || lo > hi {
		return 0, 0, false
	}
	return lo, hi, true
}

// ---- DM-01 names -----------------------------------------------------------

func (x *v) names() {
	check := func(kind, name string, seen map[string]string) {
		x.at(kind, name)
		if !m.ValidName(name) {
			x.add("DM-01", "metadata.name", "invalid name %q (letters, digits, '.', '_', '-'; max 63)", name)
		}
		if other, ok := seen[name]; ok {
			x.add("DM-01", "metadata.name", "name %q already used by a %s", name, other)
		}
		seen[name] = kind
	}
	addrNS := map[string]string{}
	for _, o := range x.t.Addresses {
		check("Address", o.Metadata.Name, addrNS)
	}
	for _, o := range x.t.AddressGroups {
		check("AddressGroup", o.Metadata.Name, addrNS)
	}
	for _, o := range x.t.VirtualIPs {
		check("VirtualIP", o.Metadata.Name, addrNS)
	}
	svcNS := map[string]string{}
	for _, o := range x.t.Services {
		check("Service", o.Metadata.Name, svcNS)
	}
	for _, o := range x.t.ServiceGroups {
		check("ServiceGroup", o.Metadata.Name, svcNS)
	}
	for _, l := range []struct {
		kind  string
		names []string
	}{
		{"Interface", names(x.t.Interfaces)}, {"Zone", names(x.t.Zones)}, {"IpPool", names(x.t.IpPools)},
		{"StaticRoute", names(x.t.StaticRoutes)}, {"Colour", names(x.t.Colours)}, {"User", names(x.t.Users)},
	} {
		seen := map[string]string{}
		for _, n := range l.names {
			check(l.kind, n, seen)
		}
	}
	for _, z := range x.t.Zones {
		x.at("Zone", z.Metadata.Name)
		seen := map[string]bool{}
		for i, p := range z.Spec.Policies {
			path := fmt.Sprintf("spec.policies[%d].name", i)
			if strings.TrimSpace(p.Name) == "" {
				x.add("DM-01", path, "Policy name is required")
			} else if seen[p.Name] {
				x.add("DM-01", path, "Policy name %q is not unique in the Zone", p.Name)
			}
			seen[p.Name] = true
		}
	}
}

func names[S any](l m.List[S]) []string {
	out := make([]string, len(l))
	for i, o := range l {
		out[i] = o.Metadata.Name
	}
	return out
}

// ---- DM-33 colours ---------------------------------------------------------

func (x *v) colours() {
	seen := map[string]string{}
	known := map[string]bool{}
	for _, c := range x.t.Colours {
		x.at("Colour", c.Metadata.Name)
		known[c.Metadata.Name] = true
		if !colourRe.MatchString(c.Spec.Value) {
			x.add("DM-33", "spec.value", "%q is not a colour like #49F527", c.Spec.Value)
		}
		v := strings.ToUpper(c.Spec.Value)
		if other, ok := seen[v]; ok {
			x.add("DM-33", "spec.value", "value %s already used by Colour %q", c.Spec.Value, other)
		}
		seen[v] = c.Metadata.Name
		if c.Metadata.Colour != "" {
			x.add("DM-33", "metadata.colour", "a Colour cannot reference a Colour")
		}
	}
	ref := func(kind, name, colour, path string) {
		if colour != "" && !known[colour] {
			x.at(kind, name)
			x.add("DM-33", path, "Colour %q does not exist", colour)
		}
	}
	for _, o := range x.t.Interfaces {
		ref("Interface", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.Zones {
		ref("Zone", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
		for i, p := range o.Spec.Policies {
			ref("Zone", o.Metadata.Name, p.Colour, fmt.Sprintf("spec.policies[%d].colour", i))
		}
	}
	for _, o := range x.t.Addresses {
		ref("Address", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.AddressGroups {
		ref("AddressGroup", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.VirtualIPs {
		ref("VirtualIP", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.Services {
		ref("Service", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.ServiceGroups {
		ref("ServiceGroup", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.IpPools {
		ref("IpPool", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
	for _, o := range x.t.StaticRoutes {
		ref("StaticRoute", o.Metadata.Name, o.Metadata.Colour, "metadata.colour")
	}
}

// ---- Interfaces: DM-03, DM-04, DM-05 ----------------------------------------

func (x *v) interfaces() {
	vlans := map[string]string{}
	type owned struct {
		p     netip.Prefix
		iface string
	}
	var all []owned
	for _, i := range x.t.Interfaces {
		x.at("Interface", i.Metadata.Name)
		s := i.Spec
		if s.Role != "wan" && s.Role != "lan" && s.Role != "dmz" {
			x.add("DM-03", "spec.role", "role must be wan, lan or dmz")
		}
		switch s.Type {
		case "physical":
			if s.Parent != "" || s.VlanID != 0 {
				x.add("DM-03", "spec.parent", "a physical Interface has neither parent nor vlanId")
			}
		case "vlan":
			p, ok := x.iface(s.Parent)
			if !ok {
				x.add("DM-03", "spec.parent", "parent Interface %q does not exist", s.Parent)
			} else if p.Spec.Type != "physical" {
				x.add("DM-03", "spec.parent", "parent %q must be a physical Interface", s.Parent)
			}
			if s.VlanID < 1 || s.VlanID > 4094 {
				x.add("DM-03", "spec.vlanId", "vlanId must be in 1–4094")
			}
			key := fmt.Sprintf("%s/%d", s.Parent, s.VlanID)
			if other, ok := vlans[key]; ok {
				x.add("DM-03", "spec.vlanId", "VLAN %d on %s already used by %q", s.VlanID, s.Parent, other)
			}
			vlans[key] = i.Metadata.Name
		default:
			x.add("DM-03", "spec.type", "type must be physical or vlan")
		}
		if len(s.Addresses) == 0 {
			x.add("DM-04", "spec.addresses", "at least one IPv4 address/prefix is required")
		}
		for k, a := range s.Addresses {
			p, err := netip.ParsePrefix(a)
			if err != nil || !p.Addr().Is4() {
				x.add("DM-04", fmt.Sprintf("spec.addresses[%d]", k), "%q is not an IPv4 address/prefix (IPv6 is deferred)", a)
				continue
			}
			for _, o := range all {
				if o.p.Overlaps(p) {
					x.add("DM-04", fmt.Sprintf("spec.addresses[%d]", k), "%s overlaps %s of Interface %q", a, o.p, o.iface)
				}
			}
			all = append(all, owned{p.Masked(), i.Metadata.Name})
		}
		seen := map[string]bool{}
		for k, a := range s.AllowAccess {
			if a != "ping" && a != "https" {
				x.add("DM-05", fmt.Sprintf("spec.allowAccess[%d]", k), "only ping and https are allowed, got %q", a)
			}
			if seen[a] {
				x.add("DM-05", fmt.Sprintf("spec.allowAccess[%d]", k), "duplicate %q", a)
			}
			seen[a] = true
		}
	}
}

// ---- Objects: DM-14 … DM-21 -------------------------------------------------

func (x *v) objects() {
	for _, a := range x.t.Addresses {
		x.at("Address", a.Metadata.Name)
		s := a.Spec
		switch s.Type {
		case "ip":
			if _, ok := parseV4(s.Value); !ok {
				x.add("DM-14", "spec.value", "%q is not an IPv4 address", s.Value)
			}
		case "subnet":
			if p, err := netip.ParsePrefix(s.Value); err != nil || !p.Addr().Is4() {
				x.add("DM-14", "spec.value", "%q is not a valid IPv4 CIDR", s.Value)
			} else if p.Masked() != p {
				x.add("DM-14", "spec.value", "%q has host bits set (did you mean %s?)", s.Value, p.Masked())
			}
		case "range":
			lo, hi, ok := strings.Cut(s.Value, "-")
			a1, ok1 := parseV4(strings.TrimSpace(lo))
			a2, ok2 := parseV4(strings.TrimSpace(hi))
			if !ok || !ok1 || !ok2 {
				x.add("DM-14", "spec.value", "%q is not a range like 10.0.0.1-10.0.0.9", s.Value)
			} else if a2.Less(a1) {
				x.add("DM-14", "spec.value", "range start is after range end")
			}
		case "fqdn":
			if !fqdnRe.MatchString(s.Value) {
				x.add("DM-14", "spec.value", "%q is not a valid FQDN", s.Value)
			}
		case "mac":
			if !macRe.MatchString(s.Value) {
				x.add("DM-14", "spec.value", "%q is not a MAC address", s.Value)
			}
		default:
			x.add("DM-14", "spec.type", "type must be ip, range, subnet, fqdn or mac")
		}
		if s.Interface != "" {
			if _, ok := x.iface(s.Interface); !ok {
				x.add("DM-02", "spec.interface", "Interface %q does not exist", s.Interface)
			}
		}
	}

	addr := map[string]m.Address{}
	for _, a := range x.t.Addresses {
		addr[a.Metadata.Name] = a
	}
	for _, g := range x.t.AddressGroups {
		x.at("AddressGroup", g.Metadata.Name)
		if len(g.Spec.Members) == 0 {
			x.add("DM-15", "spec.members", "an AddressGroup needs at least one member")
		}
		seen := map[string]bool{}
		ifaces := map[string]bool{}
		for k, mem := range g.Spec.Members {
			path := fmt.Sprintf("spec.members[%d]", k)
			if seen[mem] {
				x.add("DM-15", path, "duplicate member %q", mem)
			}
			seen[mem] = true
			a, ok := addr[mem]
			if !ok {
				x.add("DM-15", path, "%q is not an Address (groups are flat; VIPs not allowed)", mem)
				continue
			}
			ifaces[a.Spec.Interface] = true
		}
		if len(ifaces) > 1 {
			x.add("DM-16", "spec.members", "members belong to different Interfaces")
		} else {
			for i := range ifaces {
				if g.Spec.Interface != i {
					x.add("DM-16", "spec.interface", "must be %q, the Interface of every member", i)
				}
			}
		}
	}

	for _, s := range x.t.Services {
		x.at("Service", s.Metadata.Name)
		switch s.Spec.Protocol {
		case "tcp", "udp":
			if _, _, ok := parsePorts(s.Spec.Ports); !ok {
				x.add("DM-17", "spec.ports", "tcp/udp need a port or range in 1–65535, got %q", s.Spec.Ports)
			}
		case "icmp", "any":
			if s.Spec.Ports != "" {
				x.add("DM-17", "spec.ports", "ports are not allowed for %s", s.Spec.Protocol)
			}
		default:
			x.add("DM-17", "spec.protocol", "protocol must be tcp, udp, icmp or any")
		}
	}
	svc := map[string]m.Service{}
	for _, s := range x.t.Services {
		svc[s.Metadata.Name] = s
	}
	for _, g := range x.t.ServiceGroups {
		x.at("ServiceGroup", g.Metadata.Name)
		if len(g.Spec.Members) == 0 {
			x.add("DM-18", "spec.members", "a ServiceGroup needs at least one member")
		}
		protos := map[string]bool{}
		seen := map[string]bool{}
		for k, mem := range g.Spec.Members {
			path := fmt.Sprintf("spec.members[%d]", k)
			if seen[mem] {
				x.add("DM-18", path, "duplicate member %q", mem)
			}
			seen[mem] = true
			s, ok := svc[mem]
			if !ok {
				x.add("DM-18", path, "%q is not a Service (groups are flat)", mem)
				continue
			}
			protos[s.Spec.Protocol] = true
		}
		if len(protos) > 1 {
			x.add("DM-18", "spec.members", "all members must have the same protocol")
		}
	}

	for _, vip := range x.t.VirtualIPs {
		x.at("VirtualIP", vip.Metadata.Name)
		s := vip.Spec
		if s.Protocol != "tcp" && s.Protocol != "udp" {
			x.add("DM-14", "spec.protocol", "protocol must be tcp or udp")
		}
		ext, ok := x.iface(s.External.Interface)
		if !ok {
			x.add("DM-02", "spec.external.interface", "Interface %q does not exist", s.External.Interface)
		}
		ip, ipok := parseV4(s.External.IP)
		if !ipok {
			x.add("DM-19", "spec.external.ip", "%q is not an IPv4 address", s.External.IP)
		} else if ok {
			if _, in := inAny(ip, ifacePrefixes(ext)); !in {
				x.add("DM-19", "spec.external.ip", "%s is not an address of Interface %q", ip, ext.Metadata.Name)
			}
		}
		if _, ok := parseV4(s.Mapped.IP); !ok {
			x.add("DM-14", "spec.mapped.ip", "%q is not an IPv4 address", s.Mapped.IP)
		}
		elo, ehi, ok1 := parsePorts(s.External.Port)
		mlo, mhi, ok2 := parsePorts(s.Mapped.Port)
		if !ok1 {
			x.add("DM-14", "spec.external.port", "invalid port %q", s.External.Port)
		}
		if !ok2 {
			x.add("DM-14", "spec.mapped.port", "invalid port %q", s.Mapped.Port)
		}
		if ok1 && ok2 && mlo != mhi && (mhi-mlo) != (ehi-elo) {
			x.add("DM-20", "spec.mapped.port", "mapped range must have the same length as the external range, or be a single port")
		}
		// DM-35: no VIP on the management port of an Interface with https.
		if ok && ok1 && s.Protocol == "tcp" && slices.Contains(ext.Spec.AllowAccess, "https") && elo <= 443 && 443 <= ehi {
			for _, p := range ifacePrefixes(ext) {
				if p.Addr() == ip {
					x.add("DM-35", "spec.external.port", "port 443 on %s is the management port (https in allowAccess of %q)", ip, ext.Metadata.Name)
				}
			}
		}
	}

	for _, p := range x.t.IpPools {
		x.at("IpPool", p.Metadata.Name)
		a, ok1 := parseV4(p.Spec.StartIP)
		b, ok2 := parseV4(p.Spec.EndIP)
		if !ok1 || !ok2 {
			x.add("DM-14", "spec", "startIp and endIp must be IPv4 addresses")
		} else if b.Less(a) {
			x.add("DM-14", "spec.endIp", "endIp is before startIp")
		}
	}
}

// ---- Zones and Policies: DM-06 … DM-13, DM-36 -------------------------------

// addrBinding returns the Interface an AddressRef is bound to ("" = none),
// whether it is a VIP, and whether it exists at all.
func (x *v) addrRef(name string) (iface string, vip *m.VirtualIP, ok bool) {
	if n, found := strings.CutSuffix(name, InterfaceAddressesSuffix); found {
		if _, exists := x.iface(n); exists {
			return n, nil, true
		}
		return "", nil, false
	}
	for _, a := range x.t.Addresses {
		if a.Metadata.Name == name {
			return a.Spec.Interface, nil, true
		}
	}
	for _, g := range x.t.AddressGroups {
		if g.Metadata.Name == name {
			return g.Spec.Interface, nil, true
		}
	}
	for i := range x.t.VirtualIPs {
		if x.t.VirtualIPs[i].Metadata.Name == name {
			return x.t.VirtualIPs[i].Spec.External.Interface, &x.t.VirtualIPs[i], true
		}
	}
	return "", nil, false
}

func (x *v) svcRef(name string) bool {
	for _, s := range x.t.Services {
		if s.Metadata.Name == name {
			return true
		}
	}
	for _, s := range x.t.ServiceGroups {
		if s.Metadata.Name == name {
			return true
		}
	}
	return false
}

func (x *v) zones() {
	pairs := map[string]string{}
	pools := map[string]bool{}
	for _, p := range x.t.IpPools {
		pools[p.Metadata.Name] = true
	}
	for _, z := range x.t.Zones {
		x.at("Zone", z.Metadata.Name)
		s := z.Spec
		if _, ok := x.iface(s.SrcInterface); !ok {
			x.add("DM-06", "spec.srcInterface", "Interface %q does not exist", s.SrcInterface)
		}
		if _, ok := x.iface(s.DstInterface); !ok {
			x.add("DM-06", "spec.dstInterface", "Interface %q does not exist", s.DstInterface)
		}
		if s.SrcInterface == s.DstInterface {
			x.add("DM-06", "spec.dstInterface", "source and destination Interface must differ")
		}
		key := s.SrcInterface + ">" + s.DstInterface
		if other, ok := pairs[key]; ok {
			x.add("DM-07", "spec", "Zone %q already covers %s → %s", other, s.SrcInterface, s.DstInterface)
		}
		pairs[key] = z.Metadata.Name

		enabled := 0
		for i, p := range s.Policies {
			base := fmt.Sprintf("spec.policies[%d]", i)
			if p.IsEnabled() {
				enabled++
			}
			if len(p.Sources) == 0 {
				x.add("DM-08", base+".sources", "at least one source is required")
			}
			if len(p.Destinations) == 0 {
				x.add("DM-08", base+".destinations", "at least one destination is required")
			}
			if len(p.Services) == 0 {
				x.add("DM-08", base+".services", "at least one service is required")
			}
			if p.Action != "accept" && p.Action != "drop" {
				x.add("DM-08", base+".action", "action must be accept or drop")
			}
			for k, src := range p.Sources {
				path := fmt.Sprintf("%s.sources[%d]", base, k)
				iface, vip, ok := x.addrRef(src)
				switch {
				case !ok:
					x.add("DM-02", path, "%q does not exist", src)
				case vip != nil:
					x.add("DM-10", path, "a VirtualIP may appear only in destinations")
				case iface != "" && iface != s.SrcInterface:
					x.add("DM-12", path, "%q is bound to Interface %q, but this Zone's source is %q", src, iface, s.SrcInterface)
				}
			}
			for k, dst := range p.Destinations {
				path := fmt.Sprintf("%s.destinations[%d]", base, k)
				iface, vip, ok := x.addrRef(dst)
				switch {
				case !ok:
					x.add("DM-02", path, "%q does not exist", dst)
				case vip != nil:
					if vip.Spec.External.Interface != s.SrcInterface {
						x.add("DM-11", path, "VirtualIP %q enters from %q, but this Zone's source is %q", dst, iface, s.SrcInterface)
					}
				case iface != "" && iface != s.DstInterface:
					x.add("DM-12", path, "%q is bound to Interface %q, but this Zone's destination is %q", dst, iface, s.DstInterface)
				}
			}
			for k, svc := range p.Services {
				if !x.svcRef(svc) {
					x.add("DM-02", fmt.Sprintf("%s.services[%d]", base, k), "Service %q does not exist", svc)
				}
			}
			if p.Nat != nil {
				switch p.Nat.Mode {
				case "none", "interface":
					if p.Nat.Pool != "" {
						x.add("DM-13", base+".nat.pool", "pool is allowed only with mode pool")
					}
				case "pool":
					if !pools[p.Nat.Pool] {
						x.add("DM-13", base+".nat.pool", "IpPool %q does not exist", p.Nat.Pool)
					}
				default:
					x.add("DM-13", base+".nat.mode", "mode must be none, interface or pool")
				}
			}
		}
		if enabled > MaxPoliciesPerZone {
			x.add("DM-36", "spec.policies", "%d enabled Policies exceed the per-Zone limit of %d", enabled, MaxPoliciesPerZone)
		}
	}
}

// ---- Routing: DM-22, DM-23 ----------------------------------------------------

func (x *v) routes() {
	seen := map[string]string{}
	for _, r := range x.t.StaticRoutes {
		x.at("StaticRoute", r.Metadata.Name)
		s := r.Spec
		dst, err := netip.ParsePrefix(s.Destination)
		if err != nil || !dst.Addr().Is4() {
			x.add("DM-14", "spec.destination", "%q is not an IPv4 prefix", s.Destination)
		}
		gw, gwok := parseV4(s.Gateway)
		if !gwok {
			x.add("DM-22", "spec.gateway", "%q is not an IPv4 address", s.Gateway)
		}
		out, ok := x.iface(s.OutInterface)
		if !ok {
			x.add("DM-02", "spec.outInterface", "Interface %q does not exist", s.OutInterface)
		} else if gwok {
			if _, in := inAny(gw, ifacePrefixes(out)); !in {
				x.add("DM-22", "spec.gateway", "%s is not in a subnet of %q", gw, s.OutInterface)
			}
		}
		if s.Distance < 0 || s.Distance > 255 {
			x.add("DM-23", "spec.distance", "distance must be in 1–255")
		}
		if s.Priority < 0 || s.Priority > 65535 {
			x.add("DM-23", "spec.priority", "priority must be in 1–65535")
		}
		d, p := s.Distance, s.Priority
		if d == 0 {
			d = 10
		}
		if p == 0 {
			p = 1
		}
		key := fmt.Sprintf("%s|%d|%d", dst.Masked(), d, p)
		if other, ok := seen[key]; ok {
			x.add("DM-23", "spec", "same destination, distance and priority as route %q", other)
		}
		seen[key] = r.Metadata.Name
	}
}

// ---- DHCPv4: DM-24 … DM-29 ---------------------------------------------------

func (x *v) dhcp() {
	type rng struct {
		a, b  netip.Addr
		iface string
	}
	var ranges []rng
	for _, i := range x.t.Interfaces {
		d := i.Spec.Dhcp
		if d == nil {
			continue
		}
		x.at("Interface", i.Metadata.Name)
		if i.Spec.Role != "lan" {
			x.add("DM-24", "spec.dhcp", "DHCP is allowed only on role lan")
		}
		a, ok1 := parseV4(d.RangeStart)
		b, ok2 := parseV4(d.RangeEnd)
		if !ok1 || !ok2 {
			x.add("DM-25", "spec.dhcp", "rangeStart and rangeEnd must be IPv4 addresses")
			continue
		}
		if b.Less(a) {
			x.add("DM-25", "spec.dhcp.rangeEnd", "rangeEnd is before rangeStart")
		}
		pfx, in := inAny(a, ifacePrefixes(i))
		if !in || !pfx.Contains(b) {
			x.add("DM-25", "spec.dhcp", "the range must lie in one subnet of the Interface")
			continue
		}
		within := func(ip netip.Addr) bool { return !ip.Less(a) && !b.Less(ip) }
		if within(pfx.Addr()) {
			x.add("DM-26", "spec.dhcp", "the range contains the Interface IP %s", pfx.Addr())
		}
		gw := pfx.Addr()
		if d.Gateway != "" {
			g, ok := parseV4(d.Gateway)
			if !ok || !pfx.Contains(g) {
				x.add("DM-27", "spec.dhcp.gateway", "gateway must lie in %s", pfx.Masked())
			} else {
				gw = g
				if within(g) {
					x.add("DM-26", "spec.dhcp.gateway", "the range contains the gateway")
				}
			}
		}
		for _, r := range ranges {
			if !b.Less(r.a) && !r.b.Less(a) {
				x.add("DM-28", "spec.dhcp", "range overlaps the DHCP range of %q", r.iface)
			}
		}
		ranges = append(ranges, rng{a, b, i.Metadata.Name})
		macs, ips := map[string]bool{}, map[string]bool{}
		for k, r := range d.Reservations {
			path := fmt.Sprintf("spec.dhcp.reservations[%d]", k)
			if !macRe.MatchString(r.Mac) {
				x.add("DM-29", path+".mac", "%q is not a MAC address", r.Mac)
			}
			ip, ok := parseV4(r.IP)
			if !ok || !pfx.Contains(ip) || ip == pfx.Addr() || ip == gw {
				x.add("DM-29", path+".ip", "%q must be in %s and differ from Interface IP and gateway", r.IP, pfx.Masked())
			}
			mac := strings.ToLower(r.Mac)
			if macs[mac] || ips[r.IP] {
				x.add("DM-29", path, "duplicate reservation")
			}
			macs[mac], ips[r.IP] = true, true
		}
	}
}

// ---- Users -----------------------------------------------------------------

func (x *v) users() {
	admins := 0
	for _, u := range x.t.Users {
		x.at("User", u.Metadata.Name)
		switch u.Spec.Role {
		case "admin":
			admins++
		case "read-only":
		default:
			x.add("R-043.9", "spec.role", "role must be admin or read-only")
		}
		if !strings.HasPrefix(u.Spec.PasswordHash, "$2") {
			x.add("R-043.1", "spec.passwordHash", "password must be a bcrypt hash")
		}
	}
	if admins == 0 {
		x.file = "users/"
		x.add("R-043.9", "", "at least one admin User is required")
	}
}

// Builtin reports whether an object is predefined and therefore immutable
// (DM-21). For Colours only `value` is immutable (R-055.1); the caller
// handles that case.
func Builtin(obj any) bool {
	switch o := obj.(type) {
	case m.Address:
		return o.Spec.Builtin
	case m.Service:
		return o.Spec.Builtin
	case m.Colour:
		return o.Spec.Builtin
	}
	return false
}
