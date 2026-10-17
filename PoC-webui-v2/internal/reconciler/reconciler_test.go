package reconciler

import (
	"errors"
	"testing"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/defaults"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

func setup(t *testing.T) (*configstore.Store, *Reconciler) {
	t.Helper()
	dir := t.TempDir()
	s, err := configstore.Open(dir+"/config", defaults.Tree)
	if err != nil {
		t.Fatal(err)
	}
	r := New(s, dir+"/state")
	r.stageDelay = time.Millisecond
	go r.Run()
	head, _ := s.Head()
	if a, ok := r.Wait(head, 5*time.Second); !ok || a.Result != "applied" {
		t.Fatalf("factory default not applied: %+v", a)
	}
	return s, r
}

func edit(t *testing.T, s *configstore.Store, base string, fn func(t *m.Tree) any, kind, name string) (string, error) {
	t.Helper()
	return s.Update(base, configstore.Author{Name: "test"}, "test", func(tr *m.Tree) ([]configstore.Change, error) {
		return []configstore.Change{{Kind: kind, Name: name, Obj: fn(tr)}}, nil
	})
}

func get[T any](tr *m.Tree, kind, name string) T {
	o, _ := tr.Get(kind, name)
	return o.(T)
}

func TestColourOnlyChangeIsNoOp(t *testing.T) {
	s, r := setup(t)
	head, _ := s.Head()
	c, err := edit(t, s, head, func(tr *m.Tree) any { o := get[m.Colour](tr, "Colour", "green"); o.Spec.Meaning = "Veeam"; return o }, "Colour", "green")
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := r.Wait(c, 5*time.Second); a.Result != "no-op" {
		t.Fatalf("expected no-op, got %+v", a)
	}
}

func TestFailedApplyRollsBackAndKeepsAppliedCommit(t *testing.T) {
	s, r := setup(t)
	good, _ := s.Head()
	r.SetFault("dataplane", true, false)
	// an address change touches network, dataplane and dns
	c, err := edit(t, s, good, func(tr *m.Tree) any {
		i := get[m.Interface](tr, "Interface", "DMZ")
		i.Spec.Addresses = []string{"172.16.30.1/24"}
		return i
	}, "Interface", "DMZ")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Wait(c, 5*time.Second)
	if a.Result != "rolled-back" || a.Error == nil || a.Error.Driver != "dataplane" {
		t.Fatalf("expected rollback by dataplane, got %+v", a)
	}
	last := a.Steps[len(a.Steps)-1]
	if last.Op != "rollback" || last.Driver != "network" {
		t.Errorf("network commit should be rolled back last, steps: %+v", a.Steps)
	}
	st := r.Status()
	if st.State != Failed || st.AppliedCommit != good || st.DesiredCommit != c {
		t.Errorf("status: %+v", st)
	}
	// clearing the fault is a fact change: the loop retries HEAD
	r.SetFault("dataplane", false, false)
	deadline := time.Now().Add(5 * time.Second)
	for r.Status().State != InSync && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if st := r.Status(); st.AppliedCommit != c {
		t.Errorf("expected HEAD applied after fault cleared, got %+v", st)
	}
}

func TestStaleRevisionConflictsOnlyOnSameFile(t *testing.T) {
	s, _ := setup(t)
	base, _ := s.Head()
	if _, err := edit(t, s, base, func(tr *m.Tree) any {
		o := get[m.Address](tr, "Address", "nas")
		o.Spec.Value = "192.168.1.21"
		return o
	}, "Address", "nas"); err != nil {
		t.Fatal(err)
	}
	// different file, same stale base: merged
	if _, err := edit(t, s, base, func(tr *m.Tree) any {
		o := get[m.Address](tr, "Address", "web-01")
		o.Spec.Value = "172.16.20.12"
		return o
	}, "Address", "web-01"); err != nil {
		t.Fatalf("different file should merge: %v", err)
	}
	// same file, stale base: conflict
	_, err := edit(t, s, base, func(tr *m.Tree) any {
		o := get[m.Address](tr, "Address", "nas")
		o.Spec.Value = "192.168.1.22"
		return o
	}, "Address", "nas")
	var ce *configstore.ConflictError
	if !errors.As(err, &ce) || ce.File != "addresses/nas.yaml" {
		t.Fatalf("expected conflict on addresses/nas.yaml, got %v", err)
	}
}

func TestRevertIsANewCommit(t *testing.T) {
	s, r := setup(t)
	first, _ := s.Head()
	c, _ := edit(t, s, first, func(tr *m.Tree) any {
		o := get[m.Address](tr, "Address", "nas")
		o.Spec.Value = "192.168.1.21"
		return o
	}, "Address", "nas")
	r.Wait(c, 5*time.Second)
	rev, err := s.Revert(first, configstore.Author{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if rev == first {
		t.Fatal("revert must create a new commit")
	}
	tr, _, _ := s.Read(rev)
	if v := get[m.Address](tr, "Address", "nas").Spec.Value; v != "192.168.1.20" {
		t.Errorf("revert did not restore the tree: %s", v)
	}
	if a, _ := r.Wait(rev, 5*time.Second); a.Result != "applied" {
		t.Errorf("revert not applied: %+v", a)
	}
}
