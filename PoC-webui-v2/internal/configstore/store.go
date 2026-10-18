// Package configstore is the Config Manager: the desired state is a set of
// YAML files in a local Git repository, one file per object, one commit per
// saved change (ADR-0001). It is the only writer of the repository.
package configstore

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

type Author struct {
	Name   string
	Source string // client address, recorded in the commit message
}

// Change replaces (Obj != nil) or deletes (Obj == nil) one object.
type Change struct {
	Kind string
	Name string
	Obj  m.Doc
}

// ConflictError is returned when the base revision is stale and another
// commit touched the same file (optimistic locking, components.md).
type ConflictError struct {
	File string
	Head string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s was changed by another commit since your revision (HEAD is %s)", e.File, short(e.Head))
}

var ErrNoChange = errors.New("nothing to commit")

type Commit struct {
	ID      string    `json:"id"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
	Subject string    `json:"subject"`
	Body    string    `json:"body"`
	Files   []string  `json:"files"`
}

type Store struct {
	dir   string
	mu    sync.Mutex // serialises commits: the Config Manager is the only writer
	subMu sync.Mutex
	subs  []chan string
	cache sync.Map // commit id -> *m.Tree
}

// Open opens the repository, creating it with the factory-default
// configuration on first boot (components.md: no bootstrap mode).
func Open(dir string, factory func() *m.Tree) (*Store, error) {
	s := &Store{dir: dir}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return s, nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	if _, err := s.git(nil, "init", "-q", "-b", "main"); err != nil {
		return nil, err
	}
	t := factory()
	var changes []Change
	t.Each(func(o m.Doc) {
		kind, meta := o.Header()
		changes = append(changes, Change{kind, meta.Name, m.AssignUUIDs(o, nil)})
	})
	if err := s.write(changes); err != nil {
		return nil, err
	}
	if _, err := s.commit(Author{Name: "songbird", Source: "setup"}, "factory-default configuration"); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) git(stdin io.Reader, args ...string) (string, error) {
	base := []string{"-C", s.dir, "-c", "user.name=songbird", "-c", "user.email=songbird@localhost", "-c", "commit.gpgsign=false"}
	cmd := exec.Command("git", append(base, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C")
	cmd.Stdin = stdin
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("git %s: %v: %s", args[0], err, strings.TrimSpace(errb.String()+out.String()))
	}
	return out.String(), nil
}

// Resolve returns the commit id of a revision.
func (s *Store) Resolve(rev string) (string, error) {
	out, err := s.git(nil, "rev-parse", "--verify", rev+"^{commit}")
	return strings.TrimSpace(out), err
}

func (s *Store) Head() (string, error) {
	out, err := s.git(nil, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

// Read returns the configuration tree at a revision.
func (s *Store) Read(rev string) (*m.Tree, string, error) {
	id, err := s.git(nil, "rev-parse", rev+"^{commit}")
	if err != nil {
		return nil, "", err
	}
	id = strings.TrimSpace(id)
	if t, ok := s.cache.Load(id); ok {
		return t.(*m.Tree).Clone(), id, nil
	}
	list, err := s.git(nil, "ls-tree", "-r", "--name-only", id)
	if err != nil {
		return nil, "", err
	}
	var paths []string
	var in bytes.Buffer
	for _, p := range strings.Split(strings.TrimSpace(list), "\n") {
		if strings.HasSuffix(p, ".yaml") {
			paths = append(paths, p)
			fmt.Fprintf(&in, "%s:%s\n", id, p)
		}
	}
	out, err := s.git(&in, "cat-file", "--batch")
	if err != nil {
		return nil, "", err
	}
	t := &m.Tree{}
	r := bufio.NewReader(strings.NewReader(out))
	for _, p := range paths {
		hdr, err := r.ReadString('\n')
		if err != nil {
			return nil, "", err
		}
		f := strings.Fields(hdr)
		if len(f) != 3 {
			return nil, "", fmt.Errorf("unexpected cat-file output %q", hdr)
		}
		size, _ := strconv.Atoi(f[2])
		data := make([]byte, size+1)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, "", err
		}
		docs, err := m.SplitDocs(data[:size])
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", p, err)
		}
		if len(docs) != 1 {
			return nil, "", fmt.Errorf("%s: expected one object per file, found %d", p, len(docs))
		}
		d := docs[0]
		if _, ok := m.KindByName(d.Kind); !ok {
			return nil, "", fmt.Errorf("%s: unknown kind %q", p, d.Kind)
		}
		if want := m.FilePath(d.Kind, d.Name); want != p {
			return nil, "", fmt.Errorf("%s: %s/%s must be stored in %s", p, d.Kind, d.Name, want)
		}
		if _, err := t.PutYAML(d.Kind, d.Data); err != nil {
			return nil, "", fmt.Errorf("%s: %w", p, err)
		}
	}
	t.Sort()
	s.cache.Store(id, t)
	return t.Clone(), id, nil
}

// File returns the raw content of one file at a revision.
func (s *Store) File(rev, path string) ([]byte, error) {
	out, err := s.git(nil, "show", rev+":"+path)
	return []byte(out), err
}

// Update runs fn on a copy of HEAD under the write lock. fn mutates the
// tree, validates it, and returns the changes to commit. base is the
// revision the client edited (If-Match); if it is stale, the commit still
// succeeds unless another commit touched one of the same files.
func (s *Store) Update(base string, author Author, msg string, fn func(t *m.Tree) ([]Change, error)) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, head, err := s.Read("HEAD")
	if err != nil {
		return "", err
	}
	changes, err := fn(t)
	if err != nil {
		return "", err
	}
	if base != "" && base != head {
		for _, c := range changes {
			f := m.FilePath(c.Kind, c.Name)
			out, err := s.git(nil, "diff", "--name-only", base, head, "--", f)
			if err != nil {
				return "", fmt.Errorf("unknown base revision %s", short(base))
			}
			if strings.TrimSpace(out) != "" {
				return "", &ConflictError{File: f, Head: head}
			}
		}
	}
	if err := s.write(changes); err != nil {
		_, _ = s.git(nil, "checkout", "-q", "--", ".")
		return "", err
	}
	return s.commit(author, msg)
}

func (s *Store) write(changes []Change) error {
	for _, c := range changes {
		p := filepath.Join(s.dir, filepath.FromSlash(m.FilePath(c.Kind, c.Name)))
		if c.Obj == nil {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		data, err := m.MarshalObject(c.Obj)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(p, data, 0o640); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) commit(a Author, msg string) (string, error) {
	if _, err := s.git(nil, "add", "-A"); err != nil {
		return "", err
	}
	if out, _ := s.git(nil, "status", "--porcelain"); strings.TrimSpace(out) == "" {
		return "", ErrNoChange
	}
	full := fmt.Sprintf("%s\n\nActor: %s\nSource: %s\n", msg, a.Name, a.Source)
	author := fmt.Sprintf("%s <%s@songbird.local>", a.Name, a.Name)
	if _, err := s.git(strings.NewReader(full), "commit", "-q", "--author", author, "-F", "-"); err != nil {
		return "", err
	}
	id, err := s.Head()
	if err != nil {
		return "", err
	}
	s.notify(id)
	return id, nil
}

// Revert creates a new commit whose tree equals rev (rollback is a commit,
// components.md).
func (s *Store) Revert(rev string, a Author) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, err := s.git(nil, "rev-parse", "--verify", rev+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("unknown revision %s", short(rev))
	}
	id = strings.TrimSpace(id)
	if _, err := s.git(nil, "read-tree", "-u", "--reset", id); err != nil {
		return "", err
	}
	return s.commit(a, "revert to "+short(id))
}

func (s *Store) History(n int) ([]Commit, error) {
	out, err := s.git(nil, "log", fmt.Sprintf("-n%d", n), "--name-only", "--format=%x1e%H%x1f%an%x1f%aI%x1f%s%x1f%b%x1f")
	if err != nil {
		return nil, err
	}
	var res []Commit
	for _, rec := range strings.Split(out, "\x1e") {
		f := strings.Split(rec, "\x1f")
		if len(f) < 6 {
			continue
		}
		t, _ := time.Parse(time.RFC3339, f[2])
		c := Commit{ID: f[0], Author: f[1], Time: t, Subject: f[3], Body: strings.TrimSpace(f[4]), Files: []string{}}
		for _, l := range strings.Split(f[5], "\n") {
			if l = strings.TrimSpace(l); l != "" {
				c.Files = append(c.Files, l)
			}
		}
		res = append(res, c)
	}
	return res, nil
}

// Diff returns the patch introduced by a commit, or between two revisions.
func (s *Store) Diff(from, to string) (string, error) {
	if from == "" {
		return s.git(nil, "show", "--format=", "--no-color", to)
	}
	return s.git(nil, "diff", "--no-color", from, to)
}

// Archive returns the repository content at a revision as a tar.gz: the
// export of the whole Configuration (R-020.1).
func (s *Store) Archive(rev string) ([]byte, error) {
	out, err := s.git(nil, "archive", "--format=tar.gz", "--prefix=songbird-config/", rev)
	return []byte(out), err
}

func (s *Store) Subscribe() <-chan string {
	ch := make(chan string, 1)
	s.subMu.Lock()
	s.subs = append(s.subs, ch)
	s.subMu.Unlock()
	return ch
}

func (s *Store) notify(id string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	for _, ch := range s.subs {
		select { // coalesced: subscribers only need "something changed"
		case ch <- id:
		default:
		}
	}
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
