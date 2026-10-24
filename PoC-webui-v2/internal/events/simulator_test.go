package events

import (
	"net/netip"
	"testing"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

// Simulated flows are consistent: a source on an internal Interface lies in
// its subnet, and the emitted verdict is the classifier's.
func TestSimulatedFlowsAreConsistent(t *testing.T) {
	tree := defaults.Tree()
	c := NewCollector()
	c.bucket = 1e9 // no rate limiting in the test
	s := &Simulator{C: c, Applied: func() *m.Tree { return tree }}
	for range 3000 {
		s.one(tree, time.Now())
		c.bucket = 1e9
	}
	n := 0
	for _, e := range c.Recent(bufSize) {
		if e.Reason != "policy" && e.Reason != "default-deny" {
			continue
		}
		i, _ := tree.Interface(e.InIf)
		src := netip.MustParseAddr(e.Src)
		if !tree.Uplink(i.Metadata.Name) {
			in := false
			for _, a := range i.Spec.StaticAddresses() {
				if p := netip.MustParsePrefix(a); p.Masked().Contains(src) {
					in = true
				}
			}
			if !in {
				t.Errorf("%s arrived on %s, outside its subnets: %+v", e.Src, e.InIf, e)
			}
		}
		v := Classify(tree, Flow{InIf: e.InIf, Src: src, Dst: netip.MustParseAddr(e.Dst), Proto: e.Proto, Dport: e.Dport})
		if v.Action != e.Verdict || v.Policy != e.Policy {
			t.Errorf("event %+v disagrees with the classifier %+v", e, v)
		}
		n++
	}
	if n < 500 {
		t.Fatalf("only %d forwarded flows generated", n)
	}
}
