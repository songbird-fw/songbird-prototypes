// Package events simulates the Data Plane ring buffer and implements a
// minimal Event Collector: a bounded in-memory buffer plus a stream for the
// UI (components.md, "Event stream to the UI"; R-026, R-028).
//
// Events are generated from the *applied* configuration, so a Policy added
// in the UI starts producing hits once it is reconciled.
package events

import (
	"math/rand/v2"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

type Event struct {
	ID      uint64    `json:"id"`
	TS      time.Time `json:"ts"`
	Type    string    `json:"type"`    // FLOW_NEW, FLOW_DROP, FLOW_END
	Verdict string    `json:"verdict"` // accept, drop
	Reason  string    `json:"reason"`  // dataplane.md §11
	Src     string    `json:"src"`
	Dst     string    `json:"dst"`
	Sport   int       `json:"sport"`
	Dport   int       `json:"dport"`
	Proto   string    `json:"proto"`
	InIf    string    `json:"inIf"`
	Zone    string    `json:"zone,omitempty"`
	Policy  string    `json:"policy,omitempty"`
	Packets uint64    `json:"packets,omitempty"`
	Bytes   uint64    `json:"bytes,omitempty"`
}

type Counters struct {
	EventsLost uint64            `json:"eventsLost"`
	Reasons    map[string]uint64 `json:"reasons"`
	Hits       map[string]uint64 `json:"hits"` // "zone/policy"
	Accepted   uint64            `json:"accepted"`
	Dropped    uint64            `json:"dropped"`
}

type Collector struct {
	mu      sync.Mutex
	buf     []Event
	next    uint64
	subs    map[chan Event]struct{}
	c       Counters
	bucket  float64 // drop-event token bucket (R-026.1)
	lastTok time.Time
	open    []Event // accepted flows waiting for FLOW_END
}

const bufSize = 2000

func NewCollector() *Collector {
	return &Collector{subs: map[chan Event]struct{}{}, c: Counters{Reasons: map[string]uint64{}, Hits: map[string]uint64{}}, bucket: 30, lastTok: time.Now()}
}

func (c *Collector) emit(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.Verdict == "drop" {
		c.c.Dropped++
		c.c.Reasons[e.Reason]++
		// token bucket: 30 drop events/s; drops are always counted
		now := time.Now()
		c.bucket = min(30, c.bucket+now.Sub(c.lastTok).Seconds()*30)
		c.lastTok = now
		if c.bucket < 1 {
			c.c.EventsLost++
			return
		}
		c.bucket--
	} else if e.Type == "FLOW_NEW" {
		c.c.Accepted++
		c.c.Reasons[e.Reason]++
		if e.Zone != "" {
			c.c.Hits[e.Zone+"/"+e.Policy]++
		}
	}
	c.next++
	e.ID = c.next
	if len(c.buf) >= bufSize {
		c.buf = c.buf[1:]
	}
	c.buf = append(c.buf, e)
	for ch := range c.subs {
		select {
		case ch <- e:
		default: // slow consumer: drop for it, never block the collector
		}
	}
}

func (c *Collector) Recent(n int) []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > len(c.buf) {
		n = len(c.buf)
	}
	return append([]Event(nil), c.buf[len(c.buf)-n:]...)
}

func (c *Collector) Counters() Counters {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := Counters{EventsLost: c.c.EventsLost, Accepted: c.c.Accepted, Dropped: c.c.Dropped, Reasons: map[string]uint64{}, Hits: map[string]uint64{}}
	for k, v := range c.c.Reasons {
		out.Reasons[k] = v
	}
	for k, v := range c.c.Hits {
		out.Hits[k] = v
	}
	return out
}

func (c *Collector) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 256)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch, func() {
		c.mu.Lock()
		delete(c.subs, ch)
		c.mu.Unlock()
	}
}

// ---- simulated Data Plane ------------------------------------------------------

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
		// every ~60 s a short port scan from one address floods drops,
		// to exercise rate limiting and events_lost (R-026.1)
		if burstUntil.IsZero() || now.After(burstUntil.Add(60*time.Second)) {
			burstUntil = now.Add(2 * time.Second)
		}
		if now.Before(burstUntil) {
			scanner := "185.220.101." + strconv.Itoa(rand.IntN(250)+1)
			wan := firstOf(t, "wan")
			for range 20 {
				s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: "local-in-deny", Src: scanner, Dst: ifaceIP(t, wan),
					Sport: rand.IntN(30000) + 30000, Dport: rand.IntN(1024) + 1, Proto: "tcp", InIf: wan})
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
	r := rand.Float64()
	switch {
	case r < 0.07: // local-in
		lan, wan := firstOf(t, "lan"), firstOf(t, "wan")
		if rand.IntN(2) == 0 && lan != "" {
			ok := hasAccess(t, lan, "https")
			e := Event{TS: now, Type: "FLOW_NEW", Verdict: "accept", Reason: "local-in", Src: randomIn(t, lan), Dst: ifaceIP(t, lan), Sport: eph(), Dport: 443, Proto: "tcp", InIf: lan}
			if !ok {
				e.Type, e.Verdict, e.Reason = "FLOW_DROP", "drop", "local-in-deny"
			}
			s.C.emit(e)
		} else if wan != "" {
			s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: "local-in-deny", Src: publicIP(), Dst: ifaceIP(t, wan), Sport: eph(), Dport: pick(22, 23, 3389, 8080), Proto: "tcp", InIf: wan})
		}
		return
	case r < 0.09:
		wan := firstOf(t, "wan")
		s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: pickS("malformed", "fragment", "ipv6-unsupported"), Src: publicIP(), Dst: ifaceIP(t, wan), Proto: pickS("tcp", "udp"), InIf: wan})
		return
	}
	if len(t.Zones) == 0 {
		return
	}
	z := t.Zones[rand.IntN(len(t.Zones))]
	var enabled []m.Policy
	for _, p := range z.Spec.Policies {
		if p.IsEnabled() {
			enabled = append(enabled, p)
		}
	}
	if len(enabled) > 0 && rand.Float64() < 0.8 {
		// a flow built to match a Policy (the simulator does not re-check
		// first-match against earlier Policies)
		p := enabled[rand.IntN(len(enabled))]
		src := s.addr(t, p.Sources[rand.IntN(len(p.Sources))], z.Spec.SrcInterface)
		dstRef := p.Destinations[rand.IntN(len(p.Destinations))]
		proto, dport := s.service(t, p.Services[rand.IntN(len(p.Services))])
		dst := s.addr(t, dstRef, z.Spec.DstInterface)
		if vip := findVIP(t, dstRef); vip != nil {
			dst = vip.Spec.External.IP
			lo, hi := ports(vip.Spec.External.Port)
			dport, proto = lo+rand.IntN(hi-lo+1), vip.Spec.Protocol
		}
		e := Event{TS: now, Type: "FLOW_NEW", Verdict: p.Action, Reason: "policy", Src: src, Dst: dst, Sport: eph(), Dport: dport, Proto: proto, InIf: z.Spec.SrcInterface, Zone: z.Metadata.Name, Policy: p.Name}
		if p.Action == "drop" {
			e.Type = "FLOW_DROP"
		}
		if proto == "icmp" {
			e.Sport, e.Dport = 0, 0
		}
		s.C.emit(e)
		if e.Verdict == "accept" {
			s.C.mu.Lock()
			if len(s.C.open) < 500 {
				s.C.open = append(s.C.open, e)
			}
			s.C.mu.Unlock()
		}
		return
	}
	// no Policy matches: Default Deny (DM-09, R-033)
	s.C.emit(Event{TS: now, Type: "FLOW_DROP", Verdict: "drop", Reason: "default-deny", Src: randomIn(t, z.Spec.SrcInterface),
		Dst: randomIn(t, z.Spec.DstInterface), Sport: eph(), Dport: pick(23, 445, 3389, 5900, 1433), Proto: "tcp", InIf: z.Spec.SrcInterface, Zone: z.Metadata.Name})
}

func (s *Simulator) addr(t *m.Tree, ref, iface string) string {
	if n, ok := strings.CutSuffix(ref, " addresses"); ok {
		return randomIn(t, n)
	}
	for _, a := range t.Addresses {
		if a.Metadata.Name != ref {
			continue
		}
		switch a.Spec.Type {
		case "ip":
			return a.Spec.Value
		case "subnet":
			p, err := netip.ParsePrefix(a.Spec.Value)
			if err != nil || p.Bits() == 0 {
				if iface != "" && roleOf(t, iface) != "wan" {
					return randomIn(t, iface)
				}
				return publicIP()
			}
			return randomHost(p)
		case "range":
			lo, hi, _ := strings.Cut(a.Spec.Value, "-")
			a1, _ := netip.ParseAddr(strings.TrimSpace(lo))
			a2, _ := netip.ParseAddr(strings.TrimSpace(hi))
			b1, b2 := a1.As4(), a2.As4()
			x, y := u32(b1), u32(b2)
			if y < x {
				return a1.String()
			}
			return fromU32(x + uint32(rand.IntN(int(y-x)+1)))
		}
		return a.Spec.Value
	}
	for _, g := range t.AddressGroups {
		if g.Metadata.Name == ref && len(g.Spec.Members) > 0 {
			return s.addr(t, g.Spec.Members[rand.IntN(len(g.Spec.Members))], iface)
		}
	}
	return randomIn(t, iface)
}

func (s *Simulator) service(t *m.Tree, ref string) (string, int) {
	for _, g := range t.ServiceGroups {
		if g.Metadata.Name == ref && len(g.Spec.Members) > 0 {
			return s.service(t, g.Spec.Members[rand.IntN(len(g.Spec.Members))])
		}
	}
	for _, sv := range t.Services {
		if sv.Metadata.Name != ref {
			continue
		}
		switch sv.Spec.Protocol {
		case "icmp":
			return "icmp", 0
		case "any":
			return pickS("tcp", "udp"), pick(443, 80, 53, 123, 8443)
		default:
			lo, hi := ports(sv.Spec.Ports)
			return sv.Spec.Protocol, lo + rand.IntN(hi-lo+1)
		}
	}
	return "tcp", 443
}

// ---- helpers -------------------------------------------------------------------------

func findVIP(t *m.Tree, name string) *m.VirtualIP {
	for i := range t.VirtualIPs {
		if t.VirtualIPs[i].Metadata.Name == name {
			return &t.VirtualIPs[i]
		}
	}
	return nil
}

func roleOf(t *m.Tree, iface string) string {
	for _, i := range t.Interfaces {
		if i.Metadata.Name == iface {
			return i.Spec.Role
		}
	}
	return ""
}

func firstOf(t *m.Tree, role string) string {
	for _, i := range t.Interfaces {
		if i.Spec.Role == role {
			return i.Metadata.Name
		}
	}
	return ""
}

func hasAccess(t *m.Tree, iface, what string) bool {
	for _, i := range t.Interfaces {
		if i.Metadata.Name == iface {
			for _, a := range i.Spec.AllowAccess {
				if a == what {
					return true
				}
			}
		}
	}
	return false
}

func ifaceIP(t *m.Tree, iface string) string {
	for _, i := range t.Interfaces {
		if i.Metadata.Name == iface && len(i.Spec.Addresses) > 0 {
			p, err := netip.ParsePrefix(i.Spec.Addresses[0])
			if err == nil {
				return p.Addr().String()
			}
		}
	}
	return "0.0.0.0"
}

func randomIn(t *m.Tree, iface string) string {
	if roleOf(t, iface) == "wan" {
		return publicIP()
	}
	for _, i := range t.Interfaces {
		if i.Metadata.Name == iface && len(i.Spec.Addresses) > 0 {
			if p, err := netip.ParsePrefix(i.Spec.Addresses[rand.IntN(len(i.Spec.Addresses))]); err == nil {
				return randomHost(p.Masked())
			}
		}
	}
	return publicIP()
}

func randomHost(p netip.Prefix) string {
	if !p.Addr().Is4() {
		return p.Addr().String()
	}
	hostBits := 32 - p.Bits()
	if hostBits == 0 {
		return p.Addr().String()
	}
	base := u32(p.Masked().Addr().As4())
	n := uint32(1) << min(hostBits, 16)
	off := uint32(1)
	if n > 2 {
		off = 1 + uint32(rand.IntN(int(n-2)))
	}
	return fromU32(base + off)
}

func publicIP() string {
	return strconv.Itoa(pick(23, 31, 45, 52, 91, 104, 142, 151, 172, 185, 203)) + "." +
		strconv.Itoa(rand.IntN(254)+1) + "." + strconv.Itoa(rand.IntN(254)+1) + "." + strconv.Itoa(rand.IntN(254)+1)
}

func ports(s string) (int, int) {
	a, b, isRange := strings.Cut(s, "-")
	lo, _ := strconv.Atoi(strings.TrimSpace(a))
	hi := lo
	if isRange {
		hi, _ = strconv.Atoi(strings.TrimSpace(b))
	}
	if lo <= 0 {
		lo, hi = 443, 443
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

func u32(b [4]byte) uint32 {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}
func fromU32(x uint32) string {
	return netip.AddrFrom4([4]byte{byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x)}).String()
}
func eph() int                 { return 32768 + rand.IntN(28000) }
func pick(v ...int) int        { return v[rand.IntN(len(v))] }
func pickS(v ...string) string { return v[rand.IntN(len(v))] }
