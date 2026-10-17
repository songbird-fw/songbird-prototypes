// Package audit is the Audit Log: append-only JSON lines, each entry
// chained to the previous one by SHA-256 (R-035, R-035.1, R-043.10).
// Encryption at rest (also R-035.1) is out of scope for the PoC.
package audit

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Entry struct {
	Seq    int    `json:"seq"`
	Time   string `json:"time"` // ISO 8601
	Actor  string `json:"actor"`
	Source string `json:"source"`
	Action string `json:"action"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail,omitempty"`
	Before any    `json:"before,omitempty"`
	After  any    `json:"after,omitempty"`
	Prev   string `json:"prev"`
	Hash   string `json:"hash"`
}

type Log struct {
	path string
	mu   sync.Mutex
	last string
	seq  int
}

func Open(path string) (*Log, error) {
	l := &Log{path: path}
	entries, err := l.read()
	if err != nil {
		return nil, err
	}
	if n := len(entries); n > 0 {
		l.last, l.seq = entries[n-1].Hash, entries[n-1].Seq
	}
	return l, nil
}

func digest(e Entry) string {
	e.Hash = ""
	b, _ := json.Marshal(e)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// Record appends one entry (AuditSink.Record in components.md).
func (l *Log) Record(actor, source, action, target, detail string, before, after any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	e := Entry{Seq: l.seq, Time: time.Now().UTC().Format(time.RFC3339), Actor: actor, Source: source, Action: action,
		Target: target, Detail: detail, Before: before, After: after, Prev: l.last}
	// normalise before/after so the hash is stable after a JSON round trip
	if b, err := json.Marshal(e); err == nil {
		_ = json.Unmarshal(b, &e)
	}
	e.Hash = digest(e)
	l.last = e.Hash
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(e)
	_, _ = f.Write(append(b, '\n'))
}

func (l *Log) read() ([]Entry, error) {
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("audit log line %d: %w", len(out)+1, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

// Tail returns the last n entries, newest first.
func (l *Log) Tail(n int) ([]Entry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	all, err := l.read()
	if err != nil {
		return nil, err
	}
	var out []Entry
	for i := len(all) - 1; i >= 0 && len(out) < n; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

type Verification struct {
	OK       bool   `json:"ok"`
	Entries  int    `json:"entries"`
	BrokenAt int    `json:"brokenAt,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// Verify recomputes the chain: any modified, removed or reordered entry
// breaks it.
func (l *Log) Verify() Verification {
	l.mu.Lock()
	defer l.mu.Unlock()
	all, err := l.read()
	if err != nil {
		return Verification{Reason: err.Error()}
	}
	prev := ""
	for i, e := range all {
		if e.Prev != prev {
			return Verification{Entries: len(all), BrokenAt: e.Seq, Reason: fmt.Sprintf("entry %d does not chain to entry %d", e.Seq, i)}
		}
		if digest(e) != e.Hash {
			return Verification{Entries: len(all), BrokenAt: e.Seq, Reason: fmt.Sprintf("entry %d was modified", e.Seq)}
		}
		prev = e.Hash
	}
	return Verification{OK: true, Entries: len(all)}
}
