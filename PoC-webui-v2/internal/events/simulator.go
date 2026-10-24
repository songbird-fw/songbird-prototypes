package events

import (
	"math/rand/v2"
	"net/netip"
	"slices"
	"strconv"
	"time"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// Simulator stands in for the Data Plane ring buffer. Flows are generated
// from the *applied* configuration and classified with Classify, so a Policy
// added in the UI starts producing hits once it is reconciled, and a Policy
// placed after a broader one never does (first match, R-014.4).
type Simulator struct {
	C       *Collector
	Applied func() *m.Tree
}

func (s *Simulator) Run() {
	tick := time.NewTicker(150 * time.Millisecond)
	burstUntil := time.Time{}
	for now := range tick.C {
		t := s.Applied()
		if t == nil {
			continue
		}
		// every ~60 s a short port scan floods drops, to exercise rate
		// limiting and events_lost (R-026.1)
		if burstUntil.IsZero() || now.After(burstUntil.Add(60*time.Second)) {
			burstUntil = now.Add(2 * time.Second)
		}
		if ext := external(t); now.Before(burstUntil) && ext != "" {
			scanner := "185.220.101." + strconv.Itoa(rand.IntN(250)+1)
			for range 20 {
				s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: "local-in-deny", Src: scanner, Dst: ifaceIP(t, ext),
					Sport: eph(), Dport: rand.IntN(1024) + 1, Proto: "tcp", InIf: ext})
			}
		}
		for range rand.IntN(3) + 1 {
			s.one(t, now)
		}
		s.endFlows(now)
	}
}

func (s *Simulator) endFlows(now time.Time) {
	s.C.mu.Lock()
	var keep, done []Event
	for _, e := range s.C.open {
		if now.Sub(e.TS) > time.Duration(2+rand.IntN(8))*time.Second {
			done = append(done, e)
		} else {
			keep = append(keep, e)
		}
	}
	s.C.open = keep
	s.C.mu.Unlock()
	for _, e := range done {
		e.Type, e.TS = "FLOW_END", now
		e.Packets = uint64(rand.IntN(400) + 4)
		e.Bytes = e.Packets * uint64(rand.IntN(1200)+60)
		s.C.emit(e)
	}
}

func (s *Simulator) one(t *m.Tree, now time.Time) {
	ext := external(t)
	r := rand.Float64()
	switch {
	case r < 0.08:
		s.localIn(t, now)
		return
	case r < 0.10 && ext != "":
		s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: pickS("malformed", "fragment", "ipv6-unsupported"),
			Src: publicIP(), Dst: ifaceIP(t, ext), Proto: pickS("tcp", "udp"), InIf: ext})
		return
	case r < 0.12 && ext != "" && t.Sys().ThreatFeedsOn():
		s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: "blocklist",
			Src: "45.155.205." + strconv.Itoa(rand.IntN(250)+1), Dst: ifaceIP(t, ext), Sport: eph(), Dport: pick(443, 22, 25), Proto: "tcp", InIf: ext})
		return
	case r < 0.16:
		s.unpaired(t, now) // a pair without RuleSet: no-ruleset or intra-zone
		return
	}
	if len(t.RuleSets) == 0 {
		return
	}
	rs := t.RuleSets[rand.IntN(len(t.RuleSets))]
	ins, outs := t.Side(rs.Spec.From), t.Side(rs.Spec.To)
	if len(ins) == 0 || len(outs) == 0 {
		return
	}
	in, out := ins[rand.IntN(len(ins))], outs[rand.IntN(len(outs))]
	var enabled []m.Rule
	for _, p := range rs.Spec.Rules {
		if p.IsEnabled() {
			enabled = append(enabled, p)
		}
	}
	var f Flow
	if len(enabled) > 0 && rand.Float64() < 0.8 {
		// a flow built to match one Policy; Classify decides which one
		// really matches first
		p := enabled[rand.IntN(len(enabled))]
		f = Flow{InIf: in, Src: s.addr(t, p.Source, in), Dst: s.addr(t, p.Destination, out)}
		f.InIf = arrival(t, ins, f.Src, in)
		f.Proto, f.Dport = s.service(t, p.Service)
	} else {
		f = Flow{InIf: in, Src: s.addr(t, nil, in), Dst: s.addr(t, nil, out), Proto: "tcp", Dport: pick(23, 445, 3389, 5900, 1433)}
	}
	s.emitFlow(t, f, now)
}

func (s *Simulator) unpaired(t *m.Tree, now time.Time) {
	var withAddr []string
	for _, i := range t.Interfaces {
		if len(i.Spec.StaticAddresses()) > 0 {
			withAddr = append(withAddr, i.Metadata.Name)
		}
	}
	if len(withAddr) < 2 {
		return
	}
	for range 10 {
		in, out := withAddr[rand.IntN(len(withAddr))], withAddr[rand.IntN(len(withAddr))]
		if governed(t, in, out) || in == out {
			continue
		}
		f := Flow{InIf: in, Src: s.addr(t, nil, in), Dst: s.addr(t, nil, out), Proto: "tcp", Dport: pick(22, 80, 443, 445, 3389)}
		s.emitFlow(t, f, now)
		return
	}
}

func (s *Simulator) emitFlow(t *m.Tree, f Flow, now time.Time) {
	if !f.Src.IsValid() || !f.Dst.IsValid() {
		return
	}
	v := Classify(t, f)
	e := Event{TS: now, Type: "FLOW_NEW", Verdict: v.Action, Reason: v.Reason, Src: f.Src.String(), Dst: f.Dst.String(),
		Sport: eph(), Dport: f.Dport, Proto: f.Proto, InIf: f.InIf, OutIf: v.OutIf, RuleSet: v.RuleSet, Policy: v.Policy, XDst: v.XDst, XSrc: v.XSrc}
	if f.Proto == "icmp" {
		e.Sport, e.Dport = 0, 0
	}
	if v.Action == "drop" {
		e.Type = "FLOW_DROP"
	}
	s.C.emit(e)
	if v.Action == "accept" {
		s.C.mu.Lock()
		if len(s.C.open) < 500 {
			s.C.open = append(s.C.open, e)
		}
		s.C.mu.Unlock()
	}
}

// localIn simulates traffic to the firewall itself: allowAccess (DM-05,
// owner rule), the implicit service ports (DM-34) and trustedHosts.
func (s *Simulator) localIn(t *m.Tree, now time.Time) {
	var cands []m.Interface
	for _, i := range t.Interfaces {
		if len(i.Spec.StaticAddresses()) > 0 {
			cands = append(cands, i)
		}
	}
	if len(cands) == 0 {
		return
	}
	i := cands[rand.IntN(len(cands))]
	n := i.Metadata.Name
	svc := pickS("https", "ping", "dns", "dhcp", "ntp", "ssh")
	if t.Uplink(n) && rand.IntN(2) == 0 {
		svc = "ssh"
	}
	proto, port := map[string]string{"https": "tcp", "ping": "icmp", "dns": "udp", "dhcp": "udp", "ntp": "udp", "ssh": "tcp"}[svc],
		map[string]int{"https": t.Sys().HTTPSPort(), "dns": 53, "dhcp": 67, "ntp": 123, "ssh": 22}[svc]
	src := s.addr(t, nil, n)
	e := Event{TS: now, Type: "FLOW_NEW", Verdict: "accept", Reason: "local-in", Src: src.String(), Dst: ifaceIP(t, n),
		Sport: eph(), Dport: port, Proto: proto, InIf: n}
	if proto == "icmp" {
		e.Sport, e.Dport = 0, 0
	}
	if !localAllowed(t, i, svc, src) {
		e.Type, e.Verdict, e.Reason = "FLOW_DROP", "drop", "local-in-deny"
	}
	s.C.emit(e)
}

func localAllowed(t *m.Tree, i m.Interface, svc string, src netip.Addr) bool {
	sys := t.Sys()
	n := i.Metadata.Name
	if slices.Contains(i.Spec.AllowAccess, svc) { // management services (DM-05)
		if svc == "https" && sys.Admin != nil && len(sys.Admin.TrustedHosts) > 0 {
			return inAddresses(t, sys.Admin.TrustedHosts, src)
		}
		return true
	}
	served := func(refs []string) bool {
		for _, r := range refs {
			if slices.Contains(t.Side(r), n) {
				return true
			}
		}
		return false
	}
	switch svc { // DM-34: a System Service opens its own ports where it listens
	case "dhcp":
		p, ok := sys.DHCP()[n]
		return ok && p.IsEnabled()
	case "dns":
		return sys.DNS() != nil && served(sys.DNS().ListenOn)
	case "ntp":
		return sys.NTP() != nil && sys.NTP().Enabled && served(sys.NTP().ListenOn)
	}
	return false
}

// addr picks an address of a source/destination list, on the given side.
func (s *Simulator) addr(t *m.Tree, refs []string, iface string) netip.Addr {
	if !t.AnyAddress(refs) && len(refs) > 0 {
		var all []m.Address
		for _, r := range refs {
			all = append(all, t.ExpandAddresses(r)...)
		}
		rand.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		for _, a := range all {
			if ip, ok := randomInAddress(a); ok {
				return ip
			}
		}
	}
	if i, ok := t.Interface(iface); ok {
		if t.Uplink(iface) {
			return netip.MustParseAddr(publicIP())
		}
		if as := i.Spec.StaticAddresses(); len(as) > 0 {
			if p, err := netip.ParsePrefix(as[rand.IntN(len(as))]); err == nil {
				return randomHost(p.Masked())
			}
		}
	}
	return netip.MustParseAddr(publicIP())
}

func (s *Simulator) service(t *m.Tree, refs []string) (string, int) {
	if !t.AnyService(refs) && len(refs) > 0 {
		var entries []m.ProtocolEntry
		for _, r := range refs {
			for _, sv := range t.ExpandServices(r) {
				for _, p := range sv.Spec.Protocols {
					if p.Protocol == "tcp" || p.Protocol == "udp" || p.Protocol == "icmp" {
						entries = append(entries, p)
					}
				}
			}
		}
		if len(entries) > 0 {
			p := entries[rand.IntN(len(entries))]
			if p.Port == nil {
				return p.Protocol, 0
			}
			lo, hi := p.Port.Lo, max(p.Port.Lo, p.Port.Hi)
			return p.Protocol, lo + rand.IntN(hi-lo+1)
		}
	}
	return pickS("tcp", "udp"), pick(443, 80, 53, 123, 8443)
}

// ---- helpers ---------------------------------------------------------------------------------

// arrival is the Interface of a Zone a packet from src really arrives on:
// the one whose subnet contains it, or the external one for public sources.
func arrival(t *m.Tree, ins []string, src netip.Addr, fallback string) string {
	for _, n := range ins {
		if i, ok := t.Interface(n); ok {
			for _, a := range i.Spec.StaticAddresses() {
				if p, err := netip.ParsePrefix(a); err == nil && p.Masked().Contains(src) {
					return n
				}
			}
		}
	}
	for _, n := range ins {
		if t.Uplink(n) {
			return n
		}
	}
	return fallback
}

// external is the uplink with a static address, where Internet scans land.
func external(t *m.Tree) string {
	for _, i := range t.Interfaces {
		if t.Uplink(i.Metadata.Name) && len(i.Spec.StaticAddresses()) > 0 {
			return i.Metadata.Name
		}
	}
	return ""
}

func ifaceIP(t *m.Tree, iface string) string {
	if ip, ok := interfaceIP(t, iface); ok {
		return ip.String()
	}
	return "0.0.0.0"
}

func randomInAddress(a m.Address) (netip.Addr, bool) {
	switch {
	case a.Spec.Subnet != "":
		p, err := netip.ParsePrefix(a.Spec.Subnet)
		if err != nil || !p.Addr().Is4() {
			return netip.Addr{}, false
		}
		if p.Bits() == 0 {
			return netip.MustParseAddr(publicIP()), true
		}
		return randomHost(p.Masked()), true
	case a.Spec.Range != nil:
		s, err1 := netip.ParseAddr(a.Spec.Range.Start)
		e, err2 := netip.ParseAddr(a.Spec.Range.End)
		if err1 != nil || err2 != nil || !s.Is4() || e.Less(s) {
			return netip.Addr{}, false
		}
		x, y := u32(s.As4()), u32(e.As4())
		return fromU32(x + uint32(rand.IntN(int(y-x)+1))), true
	}
	return netip.Addr{}, false
}

func randomHost(p netip.Prefix) netip.Addr {
	hostBits := 32 - p.Bits()
	if hostBits == 0 {
		return p.Addr()
	}
	base := u32(p.Addr().As4())
	n := uint32(1) << min(hostBits, 16)
	off := uint32(1)
	if n > 2 {
		off = 1 + uint32(rand.IntN(int(n-2)))
	}
	return fromU32(base + off)
}

func publicIP() string {
	return strconv.Itoa(pick(23, 31, 45, 52, 91, 104, 142, 151, 176, 185)) + "." +
		strconv.Itoa(rand.IntN(254)+1) + "." + strconv.Itoa(rand.IntN(254)+1) + "." + strconv.Itoa(rand.IntN(254)+1)
}

func u32(b [4]byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func fromU32(x uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x)})
}

func eph() int                 { return 32768 + rand.IntN(28000) }
func pick(v ...int) int        { return v[rand.IntN(len(v))] }
func pickS(v ...string) string { return v[rand.IntN(len(v))] }
