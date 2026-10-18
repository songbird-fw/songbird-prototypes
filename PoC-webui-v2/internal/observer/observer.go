// Package observer computes health (R-032) and metrics (R-027). CPU and
// memory are read from the host's /proc; per-Interface throughput is
// simulated because the PoC has no Data Plane counters.
package observer

import (
	"bufio"
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/events"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/reconciler"
)

const (
	Interval = 2 * time.Second
	Keep     = 150 // 5 minutes of samples
)

type Sample struct {
	T     int64              `json:"t"` // unix ms
	CPU   float64            `json:"cpu"`
	Mem   float64            `json:"mem"`
	RxBps map[string]float64 `json:"rx"`
	TxBps map[string]float64 `json:"tx"`
}

type Observer struct {
	Rec *reconciler.Reconciler
	Ev  *events.Collector

	mu                  sync.Mutex
	samples             []Sample
	prevIdle, prevTotal uint64
	walk                map[string]float64
	started             time.Time
}

func New(rec *reconciler.Reconciler, ev *events.Collector) *Observer {
	return &Observer{Rec: rec, Ev: ev, walk: map[string]float64{}, started: time.Now()}
}

func (o *Observer) Run() {
	o.sample()
	for range time.Tick(Interval) {
		o.sample()
	}
}

func (o *Observer) sample() {
	s := Sample{T: time.Now().UnixMilli(), RxBps: map[string]float64{}, TxBps: map[string]float64{}}
	s.CPU = o.cpu()
	s.Mem = mem()
	if t := o.Rec.AppliedTree(); t != nil {
		for _, i := range t.Interfaces {
			n := i.Metadata.Name
			if len(i.Spec.StaticAddresses()) == 0 && i.Spec.EffectiveRole() != "external" {
				continue // e.g. an 802.1Q trunk: its traffic is counted on the VLANs
			}
			base := map[string]float64{"external": 40e6, "internal": 25e6, "dmz": 8e6, "transit": 15e6}[i.Spec.EffectiveRole()]
			if i.Spec.Type == "vlan" {
				base /= 2
			}
			if _, ok := o.walk[n]; !ok {
				o.walk[n] = 1
			}
			o.walk[n] = math.Max(0.2, math.Min(1.8, o.walk[n]+rand.NormFloat64()*0.08+(1-o.walk[n])*0.1))
			wave := 1 + 0.25*math.Sin(float64(s.T)/45000+float64(len(n)))
			s.RxBps[n] = base * o.walk[n] * wave
			s.TxBps[n] = base * 0.35 * o.walk[n] * (2 - wave)
		}
	}
	o.mu.Lock()
	o.samples = append(o.samples, s)
	if len(o.samples) > Keep {
		o.samples = o.samples[len(o.samples)-Keep:]
	}
	o.mu.Unlock()
}

func (o *Observer) Samples() []Sample {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Sample(nil), o.samples...)
}

func (o *Observer) cpu() float64 {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Scan()
	fields := strings.Fields(sc.Text())
	var total, idle uint64
	for i, v := range fields[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 || i == 4 { // idle + iowait
			idle += n
		}
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	first := o.prevTotal == 0
	dt, di := total-o.prevTotal, idle-o.prevIdle
	o.prevTotal, o.prevIdle = total, idle
	if first || dt == 0 {
		return 0
	}
	return 100 * float64(dt-di) / float64(dt)
}

func mem() float64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	vals := map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), ":")
		n, _ := strconv.ParseFloat(strings.Fields(v + " 0")[0], 64)
		vals[k] = n
	}
	if vals["MemTotal"] == 0 {
		return 0
	}
	return 100 * (1 - vals["MemAvailable"]/vals["MemTotal"])
}

// ---- health --------------------------------------------------------------------

type Subsystem struct {
	Status string `json:"status"` // running, degraded, stopped
	Detail string `json:"detail,omitempty"`
}

type Health struct {
	Status     string               `json:"status"`
	Uptime     int64                `json:"uptimeSeconds"`
	Subsystems map[string]Subsystem `json:"subsystems"`
}

// Health builds the /health document (R-032). Subsystem names follow the
// RA glossary: Data Plane, DHCP, DNS, GitOps, API/UI, Alerting, plus the
// Reconciler and the network driver.
func (o *Observer) Health() Health {
	st := o.Rec.Status()
	h := Health{Status: "running", Uptime: int64(time.Since(o.started).Seconds()), Subsystems: map[string]Subsystem{}}
	for _, d := range st.Drivers {
		sub := Subsystem{Status: d.Status}
		if d.Stopped {
			sub.Detail = "stopped (fault injected)"
		} else if d.FailNextApply {
			sub.Status, sub.Detail = "degraded", "next apply will fail (fault injected)"
		} else if d.AppliedHash != "" {
			sub.Detail = "plan section " + d.AppliedHash
		}
		h.Subsystems[d.Name] = sub
	}
	if dp, ok := h.Subsystems["dataplane"]; ok && dp.Status == "running" {
		c := o.Ev.Counters()
		if c.EventsLost > 0 {
			dp.Detail += ", events_lost=" + strconv.FormatUint(c.EventsLost, 10)
		}
		h.Subsystems["dataplane"] = dp
	}
	rec := Subsystem{Status: "running", Detail: st.State}
	switch st.State {
	case reconciler.Failed, reconciler.Degraded:
		rec.Status = "degraded"
		if st.LastError != nil {
			rec.Detail = st.State + ": " + st.LastError.Message
		}
	}
	h.Subsystems["reconciler"] = rec
	h.Subsystems["api"] = Subsystem{Status: "running"}
	gitops := Subsystem{Status: "stopped", Detail: "System.git.enabled is false"}
	if t := o.Rec.AppliedTree(); t != nil {
		if g := t.Sys().Git; g != nil && g.Enabled {
			gitops = Subsystem{Status: "stopped", Detail: "remote " + g.Remote + " configured; push/pull not implemented in the PoC"}
		}
	}
	h.Subsystems["gitops"] = gitops
	h.Subsystems["alerting"] = Subsystem{Status: "stopped", Detail: "not in phase 0.1"}
	for k, s := range h.Subsystems {
		if k == "gitops" || k == "alerting" {
			continue // not configured is not a fault
		}
		if s.Status != "running" {
			h.Status = "degraded"
		}
	}
	return h
}

// ActiveRoutes applies the DM-23 preference rule.
func ActiveRoutes(t *m.Tree) map[string]bool {
	best := map[string]m.StaticRoute{}
	for _, r := range t.StaticRoutes {
		b, ok := best[r.Spec.Destination]
		d, p := r.Spec.Preference()
		bd, bp := b.Spec.Preference()
		if !ok || d < bd || d == bd && p < bp {
			best[r.Spec.Destination] = r
		}
	}
	out := map[string]bool{}
	for _, r := range best {
		out[r.Metadata.Name] = true
	}
	return out
}
