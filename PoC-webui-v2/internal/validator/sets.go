package validator

import (
	"net/netip"
	"sort"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// Interval sets over IPv4 addresses and ports, used to decide whether DNAT
// Policies overlap (DM-41), shadow each other, or hit the management port
// (DM-35). The model is IPv4-only; IPv6 objects are ignored here.

type iv struct{ lo, hi uint32 }

type ivs []iv

func u32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// addrInterval is the IPv4 span of an Address (subnet or range).
func addrInterval(a m.Address) (iv, bool) {
	switch {
	case a.Spec.Subnet != "":
		p, err := netip.ParsePrefix(a.Spec.Subnet)
		if err != nil || !p.Addr().Is4() {
			return iv{}, false
		}
		p = p.Masked()
		lo := u32(p.Addr())
		return iv{lo, lo | (1<<(32-p.Bits()) - 1)}, true
	case a.Spec.Range != nil:
		s, err1 := netip.ParseAddr(a.Spec.Range.Start)
		e, err2 := netip.ParseAddr(a.Spec.Range.End)
		if err1 != nil || err2 != nil || !s.Is4() || !e.Is4() || e.Less(s) {
			return iv{}, false
		}
		return iv{u32(s), u32(e)}, true
	}
	return iv{}, false
}

func (s ivs) norm() ivs {
	if len(s) == 0 {
		return s
	}
	c := append(ivs(nil), s...)
	sort.Slice(c, func(i, j int) bool { return c[i].lo < c[j].lo })
	out := ivs{c[0]}
	for _, x := range c[1:] {
		last := &out[len(out)-1]
		if uint64(x.lo) <= uint64(last.hi)+1 {
			last.hi = max(last.hi, x.hi)
		} else {
			out = append(out, x)
		}
	}
	return out
}

func (s ivs) overlaps(o ivs) bool {
	for _, a := range s {
		for _, b := range o {
			if a.lo <= b.hi && b.lo <= a.hi {
				return true
			}
		}
	}
	return false
}

// covers reports whether every element of o is inside s.
func (s ivs) covers(o ivs) bool {
	n := s.norm()
	for _, b := range o {
		ok := false
		for _, a := range n {
			if a.lo <= b.lo && b.hi <= a.hi {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (s ivs) size() uint64 {
	var n uint64
	for _, x := range s.norm() {
		n += uint64(x.hi-x.lo) + 1
	}
	return n
}

// addrSet is the IPv4 set of an address list; any = it holds a predefined Address.
type addrSet struct {
	any bool
	ivs ivs
}

func (x *v) addrSet(refs []string) addrSet {
	if x.t.AnyAddress(refs) {
		return addrSet{any: true}
	}
	var s addrSet
	for _, r := range refs {
		for _, a := range x.t.ExpandAddresses(r) {
			if i, ok := addrInterval(a); ok {
				s.ivs = append(s.ivs, i)
			}
		}
	}
	return s
}

func (a addrSet) overlaps(b addrSet) bool {
	switch {
	case a.any:
		return b.any || len(b.ivs) > 0
	case b.any:
		return len(a.ivs) > 0
	}
	return a.ivs.overlaps(b.ivs)
}

func (a addrSet) covers(b addrSet) bool {
	if a.any {
		return true
	}
	if b.any {
		return false
	}
	return a.ivs.covers(b.ivs)
}

// svcSet maps a protocol to its destination ports. Protocols without ports
// (icmp, ip) are recorded with the full range so that they compare equal.
type svcSet struct {
	any   bool
	ports map[string]ivs
}

func (x *v) svcSet(refs []string) svcSet {
	if x.t.AnyService(refs) {
		return svcSet{any: true}
	}
	s := svcSet{ports: map[string]ivs{}}
	for _, r := range refs {
		for _, sv := range x.t.ExpandServices(r) {
			for _, p := range sv.Spec.Protocols {
				i := iv{0, 65535}
				if p.Port != nil {
					i = iv{uint32(p.Port.Lo), uint32(p.Port.Hi)}
				}
				s.ports[p.Protocol] = append(s.ports[p.Protocol], i)
			}
		}
	}
	return s
}

func (a svcSet) overlaps(b svcSet) bool {
	if a.any || b.any {
		return true
	}
	for proto, pa := range a.ports {
		if pb, ok := b.ports[proto]; ok && pa.overlaps(pb) {
			return true
		}
	}
	return false
}

func (a svcSet) covers(b svcSet) bool {
	if a.any {
		return true
	}
	if b.any {
		return false
	}
	for proto, pb := range b.ports {
		pa, ok := a.ports[proto]
		if !ok || !pa.covers(pb) {
			return false
		}
	}
	return true
}

// has reports whether the set contains proto/port.
func (a svcSet) has(proto string, port int) bool {
	if a.any {
		return true
	}
	return a.ports[proto].overlaps(ivs{{uint32(port), uint32(port)}})
}

func (a svcSet) protocols() map[string]bool {
	out := map[string]bool{}
	for p := range a.ports {
		out[p] = true
	}
	return out
}
