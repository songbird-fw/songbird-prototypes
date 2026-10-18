// Package reconciler implements the reconciliation loop of reconciliation.md:
// HEAD → validate → compile → diff → apply (atomic) → status.
//
// Validator and the loop are real. Compiler output and drivers are
// simulated: the PoC has no kernel, so a "driver" only records the hash of
// the Plan section it was given and can be told to fail (fault injection),
// to demonstrate rollback and the Failed state.
package reconciler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

const (
	Starting    = "starting"
	Reconciling = "reconciling"
	InSync      = "in-sync"
	Failed      = "failed"
	Degraded    = "degraded"
)

var DriverNames = []string{"network", "dataplane", "dhcp", "dns"}

type ErrorInfo struct {
	Step       string            `json:"step"`
	Driver     string            `json:"driver,omitempty"`
	Message    string            `json:"message"`
	Validation []validator.Error `json:"validation,omitempty"`
}

type Step struct {
	Driver string `json:"driver"`
	Op     string `json:"op"` // prepare, commit, rollback
	OK     bool   `json:"ok"`
	Note   string `json:"note,omitempty"`
}

type Attempt struct {
	Time    time.Time  `json:"time"`
	Commit  string     `json:"commit"`
	Result  string     `json:"result"` // applied, no-op, rolled-back, invalid
	Changed []string   `json:"changed"`
	Steps   []Step     `json:"steps"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

type DriverStatus struct {
	Name          string `json:"name"`
	Status        string `json:"status"` // running, degraded, stopped
	AppliedHash   string `json:"appliedHash"`
	FailNextApply bool   `json:"failNextApply"`
	Stopped       bool   `json:"stopped"`
}

// PlanSummary is what the (simulated) Compiler produced for the Data Plane.
type PlanSummary struct {
	RuleSets     int                 `json:"ruleSets"`
	Pairs        int                 `json:"pairs"` // directed Interface pairs after Zone expansion
	Policies     int                 `json:"policies"`
	Disabled     int                 `json:"disabled"`
	MatchEntries int                 `json:"matchEntries"`
	DNATEntries  int                 `json:"dnatEntries"`
	LocalIn      map[string][]string `json:"localIn"`
	IntraZone    map[string]string   `json:"intraZone"`
	Sections     map[string]string   `json:"sections"`
}

type Status struct {
	State         string         `json:"state"`
	DesiredCommit string         `json:"desiredCommit"`
	AppliedCommit string         `json:"appliedCommit"`
	LastKnownGood string         `json:"lastKnownGood"`
	LastAttempt   *Attempt       `json:"lastAttempt,omitempty"`
	LastError     *ErrorInfo     `json:"lastError,omitempty"`
	Drivers       []DriverStatus `json:"drivers"`
	Plan          *PlanSummary   `json:"plan,omitempty"`
}

type Reconciler struct {
	store    *configstore.Store
	stateDir string
	OnResult func(a Attempt) // Audit Log hook

	mu          sync.Mutex
	st          Status
	applied     map[string]string // driver -> applied section hash
	appliedTree *m.Tree
	fail        map[string]bool
	stopped     map[string]bool
	attempts    map[string]Attempt
	changed     chan struct{} // closed and replaced on every status change
	trigger     chan struct{}
	stageDelay  time.Duration
}

func New(store *configstore.Store, stateDir string) *Reconciler {
	r := &Reconciler{
		store: store, stateDir: stateDir,
		st:         Status{State: Starting},
		applied:    map[string]string{},
		fail:       map[string]bool{},
		stopped:    map[string]bool{},
		attempts:   map[string]Attempt{},
		changed:    make(chan struct{}),
		trigger:    make(chan struct{}, 1),
		stageDelay: 150 * time.Millisecond,
	}
	if b, err := os.ReadFile(filepath.Join(stateDir, "last-known-good")); err == nil {
		r.st.LastKnownGood = strings.TrimSpace(string(b))
	}
	return r
}

// Trigger marks the state dirty; triggers are coalesced (§3).
func (r *Reconciler) Trigger() {
	select {
	case r.trigger <- struct{}{}:
	default:
	}
}

func (r *Reconciler) Run() {
	commits := r.store.Subscribe()
	go func() {
		for range commits {
			r.Trigger()
		}
	}()
	go func() { // periodic resync (§8), proposed 5 min
		for range time.Tick(5 * time.Minute) {
			r.Trigger()
		}
	}()
	r.boot()
	for range r.trigger {
		r.reconcile("HEAD")
	}
}

// boot applies HEAD; if it cannot be applied, falls back to the last known
// good commit without touching Git (§7, R-033.1).
func (r *Reconciler) boot() {
	a := r.reconcile("HEAD")
	if a.Result == "applied" || a.Result == "no-op" || r.st.LastKnownGood == "" || r.st.LastKnownGood == a.Commit {
		return
	}
	failed := a
	lkg := r.reconcile(r.st.LastKnownGood)
	r.mu.Lock()
	if lkg.Result == "applied" {
		r.st.State, r.st.LastError, r.st.LastAttempt = Failed, failed.Error, &failed
	}
	r.publish()
	r.mu.Unlock()
}

func (r *Reconciler) publish() { // caller holds mu
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *Reconciler) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.st
	s.Drivers = nil
	for _, d := range DriverNames {
		ds := DriverStatus{Name: d, Status: "running", AppliedHash: r.applied[d], FailNextApply: r.fail[d], Stopped: r.stopped[d]}
		if r.stopped[d] {
			ds.Status = "stopped"
		} else if r.applied[d] == "" {
			ds.Status = "stopped"
		}
		s.Drivers = append(s.Drivers, ds)
	}
	return s
}

// AppliedTree is the configuration currently running (used by the
// simulated Data Plane to generate traffic).
func (r *Reconciler) AppliedTree() *m.Tree {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.appliedTree
}

// SetFault makes the next commit of a driver fail (PoC fault injection).
// Clearing a fault is treated like a runtime fact change: reconcile again.
func (r *Reconciler) SetFault(driver string, fail, stopped bool) {
	r.mu.Lock()
	r.fail[driver] = fail
	if driver == "dhcp" || driver == "dns" {
		r.stopped[driver] = stopped
	}
	r.publish()
	r.mu.Unlock()
	if !fail {
		r.Trigger()
	}
}

// Wait blocks until the commit has been reconciled, or a newer commit
// superseded it, or the timeout expires (reconciliation.md §5).
func (r *Reconciler) Wait(commit string, timeout time.Duration) (Attempt, bool) {
	deadline := time.After(timeout)
	for {
		r.mu.Lock()
		if a, ok := r.attempts[commit]; ok {
			r.mu.Unlock()
			return a, true
		}
		ch := r.changed
		r.mu.Unlock()
		select {
		case <-ch:
		case <-deadline:
			return Attempt{Commit: commit, Result: "pending"}, false
		}
	}
}

func (r *Reconciler) reconcile(rev string) Attempt {
	r.mu.Lock()
	r.st.State = Reconciling
	r.publish()
	r.mu.Unlock()

	a := Attempt{Time: time.Now().UTC(), Changed: []string{}, Steps: []Step{}}
	finish := func(state string) Attempt {
		r.mu.Lock()
		r.st.State = state
		r.st.LastAttempt = &a
		r.st.LastError = a.Error
		if a.Commit != "" {
			r.attempts[a.Commit] = a
		}
		r.publish()
		r.mu.Unlock()
		if r.OnResult != nil {
			r.OnResult(a)
		}
		return a
	}

	// 1. Read
	tree, commit, err := r.store.Read(rev)
	if err != nil {
		a.Result, a.Error = "rolled-back", &ErrorInfo{Step: "read", Message: err.Error()}
		return finish(Degraded)
	}
	a.Commit = commit
	r.mu.Lock()
	if rev == "HEAD" {
		r.st.DesiredCommit = commit
	}
	r.mu.Unlock()

	// 2. Validate
	if errs := validator.Errors(validator.Validate(tree)); len(errs) > 0 { // warnings never block an Apply
		a.Result = "invalid"
		a.Error = &ErrorInfo{Step: "validate", Message: fmt.Sprintf("%d validation error(s)", len(errs)), Validation: errs}
		return finish(Failed)
	}

	// 3. Compile
	plan := compile(tree)

	// 4. Diff
	r.mu.Lock()
	for _, d := range DriverNames {
		if plan.Sections[d] != r.applied[d] {
			a.Changed = append(a.Changed, d)
		}
	}
	r.mu.Unlock()

	// 5. Apply
	if len(a.Changed) == 0 {
		a.Result = "no-op"
	} else if errInfo := r.apply(&a); errInfo != nil {
		a.Result, a.Error = "rolled-back", errInfo
		return finish(Failed)
	} else {
		a.Result = "applied"
	}

	r.mu.Lock()
	for _, d := range a.Changed {
		r.applied[d] = plan.Sections[d]
	}
	r.appliedTree = tree
	r.st.AppliedCommit = commit
	r.st.LastKnownGood = commit
	r.st.Plan = &plan
	r.mu.Unlock()
	_ = os.MkdirAll(r.stateDir, 0o750)
	_ = os.WriteFile(filepath.Join(r.stateDir, "last-known-good"), []byte(commit+"\n"), 0o640)

	r.mu.Lock()
	inSync := r.st.DesiredCommit == commit
	r.mu.Unlock()
	if inSync {
		return finish(InSync)
	}
	return finish(Failed) // fallback commit applied, HEAD is not
}

// apply runs one transaction across drivers in the order of §6.2 and rolls
// back the committed stages in reverse order on the first failure.
func (r *Reconciler) apply(a *Attempt) *ErrorInfo {
	changed := map[string]bool{}
	for _, d := range a.Changed {
		changed[d] = true
	}
	type stage struct{ driver, op, note string }
	var stages []stage
	for _, d := range []string{"network", "dhcp", "dns"} {
		if changed[d] {
			stages = append(stages, stage{d, "prepare", "pre-flight"})
		}
	}
	if changed["network"] {
		stages = append(stages, stage{"network", "commit", "additive: links, VLANs, addresses, routes"})
	}
	if changed["dataplane"] {
		stages = append(stages, stage{"dataplane", "prepare", "build inactive generation"}, stage{"dataplane", "commit", "switch active_gen"})
	}
	for _, d := range []string{"dhcp", "dns"} {
		if changed[d] {
			stages = append(stages, stage{d, "commit", "reload"})
		}
	}
	if changed["network"] {
		stages = append(stages, stage{"network", "commit", "removals"})
	}

	var committed []string
	for _, s := range stages {
		time.Sleep(r.stageDelay)
		r.mu.Lock()
		fail := r.fail[s.driver] && s.op == "commit"
		r.mu.Unlock()
		if fail {
			a.Steps = append(a.Steps, Step{s.driver, s.op, false, "simulated failure"})
			for i := len(committed) - 1; i >= 0; i-- {
				time.Sleep(r.stageDelay / 2)
				a.Steps = append(a.Steps, Step{committed[i], "rollback", true, "restored snapshot"})
			}
			return &ErrorInfo{Step: "apply", Driver: s.driver, Message: s.driver + " driver: simulated failure during " + s.note}
		}
		a.Steps = append(a.Steps, Step{s.driver, s.op, true, s.note})
		if s.op == "commit" {
			committed = append(committed, s.driver)
		}
	}
	return nil
}
