package observer

import (
	"testing"
	"time"
)

// Raw samples every 2 s are averaged into 30 s buckets aligned to the
// clock; the open bucket is returned too.
func TestTierAveragesBuckets(t *testing.T) {
	tr := newTier(30*time.Second, time.Hour)
	base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).UnixMilli()
	for i := range 20 { // 40 s: one full bucket (15 samples) and 5 in the next
		tr.add(Sample{T: base + int64(i)*2000, CPU: float64(i), RxBps: map[string]float64{"wan": 10}, TxBps: map[string]float64{}})
	}
	got := tr.samples()
	if len(got) != 2 {
		t.Fatalf("want 2 buckets, got %d", len(got))
	}
	if got[0].T != base || got[0].CPU != 7 || got[0].RxBps["wan"] != 10 {
		t.Errorf("first bucket: %+v", got[0])
	}
	if got[1].T != base+30000 || got[1].CPU != 17 {
		t.Errorf("open bucket: %+v", got[1])
	}
}

func TestHistoryRanges(t *testing.T) {
	o := &Observer{tiers: newTiers(), started: time.Now()}
	now := time.Now().UnixMilli()
	for i := range 40 {
		s := Sample{T: now - int64(39-i)*2000, CPU: 5, RxBps: map[string]float64{}, TxBps: map[string]float64{}}
		o.samples = append(o.samples, s)
		for _, tr := range o.tiers {
			tr.add(s)
		}
	}
	for _, r := range Ranges {
		h, err := o.History(r.Name)
		if err != nil || len(h.Samples) == 0 || h.To-h.From != r.Span.Milliseconds() {
			t.Errorf("%s: %v, %d samples, span %d", r.Name, err, len(h.Samples), h.To-h.From)
		}
	}
	if h, _ := o.History("rt"); len(h.Samples) > 32 {
		t.Errorf("real-time keeps about one minute, got %d samples", len(h.Samples))
	}
	if _, err := o.History("1y"); err == nil {
		t.Error("unknown range accepted")
	}
}
