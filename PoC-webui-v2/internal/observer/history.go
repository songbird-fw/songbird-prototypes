package observer

import (
	"fmt"
	"time"
)

// Metric history for the dashboard ranges (R-027). The raw samples (every
// Interval) cover 5 minutes; three rollup tiers keep the average of each
// bucket, aligned to wall-clock time: 30 s for 1 hour, 10 min for 1 day,
// 1 h for 1 week. History lives in memory and starts with the process.

type tier struct {
	step time.Duration
	keep int
	done []Sample // closed buckets, oldest first
	acc  Sample   // running sums of the open bucket
	n    int      // samples in the open bucket
}

func newTier(step time.Duration, span time.Duration) *tier {
	return &tier{step: step, keep: int(span / step)}
}

func bucketStart(t int64, step time.Duration) int64 {
	ms := step.Milliseconds()
	return t - t%ms
}

// add folds a raw sample into the tier, closing the open bucket when the
// sample belongs to the next one.
func (t *tier) add(s Sample) {
	b := bucketStart(s.T, t.step)
	if t.n > 0 && b != t.acc.T {
		t.done = append(t.done, t.average())
		if len(t.done) > t.keep {
			t.done = t.done[len(t.done)-t.keep:]
		}
		t.n = 0
	}
	if t.n == 0 {
		t.acc = Sample{T: b, RxBps: map[string]float64{}, TxBps: map[string]float64{}}
	}
	t.acc.CPU += s.CPU
	t.acc.Mem += s.Mem
	for k, v := range s.RxBps {
		t.acc.RxBps[k] += v
	}
	for k, v := range s.TxBps {
		t.acc.TxBps[k] += v
	}
	t.n++
}

func (t *tier) average() Sample {
	n := float64(t.n)
	out := Sample{T: t.acc.T, CPU: t.acc.CPU / n, Mem: t.acc.Mem / n, RxBps: map[string]float64{}, TxBps: map[string]float64{}}
	for k, v := range t.acc.RxBps {
		out.RxBps[k] = v / n
	}
	for k, v := range t.acc.TxBps {
		out.TxBps[k] = v / n
	}
	return out
}

// samples returns the closed buckets plus the open one, so a range shows
// data as soon as the process has run for a few seconds.
func (t *tier) samples() []Sample {
	out := append([]Sample(nil), t.done...)
	if t.n > 0 {
		out = append(out, t.average())
	}
	return out
}

// Range is one dashboard time range.
type Range struct {
	Name   string        // rt, 5m, 1h, 1d, 1w
	Span   time.Duration // width of the x axis
	Step   time.Duration // distance between points
	Source int           // -1 = raw samples, else index into Observer.tiers
}

var Ranges = []Range{
	{"rt", time.Minute, Interval, -1},
	{"5m", 5 * time.Minute, Interval, -1},
	{"1h", time.Hour, 30 * time.Second, 0},
	{"1d", 24 * time.Hour, 10 * time.Minute, 1},
	{"1w", 7 * 24 * time.Hour, time.Hour, 2},
}

func newTiers() []*tier {
	return []*tier{
		newTier(30*time.Second, time.Hour),
		newTier(10*time.Minute, 24*time.Hour),
		newTier(time.Hour, 7*24*time.Hour),
	}
}

// Series is the answer of GET /api/v1/metrics?range=…
type Series struct {
	Range   string   `json:"range"`
	From    int64    `json:"from"`    // unix ms, left edge of the x axis
	To      int64    `json:"to"`      // unix ms, right edge
	StepMs  int64    `json:"stepMs"`  // distance between points
	Since   int64    `json:"since"`   // unix ms, start of the history (process start)
	Samples []Sample `json:"samples"` // oldest first, within [From, To]
}

// History returns the samples of one range.
func (o *Observer) History(name string) (Series, error) {
	var r *Range
	for i := range Ranges {
		if Ranges[i].Name == name {
			r = &Ranges[i]
		}
	}
	if r == nil {
		return Series{}, fmt.Errorf("unknown range %q (rt, 5m, 1h, 1d, 1w)", name)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	now := time.Now().UnixMilli()
	from := now - r.Span.Milliseconds()
	var src []Sample
	if r.Source < 0 {
		src = o.samples
	} else {
		src = o.tiers[r.Source].samples()
	}
	out := Series{Range: r.Name, From: from, To: now, StepMs: r.Step.Milliseconds(), Since: o.started.UnixMilli(), Samples: []Sample{}}
	for _, s := range src {
		if s.T >= from-r.Step.Milliseconds() {
			out.Samples = append(out.Samples, s)
		}
	}
	return out, nil
}
