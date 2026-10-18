// Package events simulates the Data Plane ring buffer and implements a
// minimal Event Collector: a bounded in-memory buffer plus a stream for the
// UI (components.md, "Event stream to the UI"; R-026, R-028).
//
// Events are generated from the *applied* configuration, so a Policy added
// in the UI starts producing hits once it is reconciled.
package events

import (
	"sync"
	"time"
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
	OutIf   string    `json:"outIf,omitempty"`
	RuleSet string    `json:"ruleSet,omitempty"`
	Policy  string    `json:"policy,omitempty"` // matched Policy: (RuleSet, Policy) identifies it (R-026)
	XDst    string    `json:"xdst,omitempty"`   // destination after DNAT, ip:port
	XSrc    string    `json:"xsrc,omitempty"`   // source after SNAT
	Packets uint64    `json:"packets,omitempty"`
	Bytes   uint64    `json:"bytes,omitempty"`
}

type Counters struct {
	EventsLost uint64            `json:"eventsLost"`
	Reasons    map[string]uint64 `json:"reasons"`
	Hits       map[string]uint64 `json:"hits"` // "ruleSet/policy"
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
	if e.Type != "FLOW_END" && e.Policy != "" {
		c.c.Hits[e.RuleSet+"/"+e.Policy]++ // per-Policy counters (dataplane.md §11)
	}
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
