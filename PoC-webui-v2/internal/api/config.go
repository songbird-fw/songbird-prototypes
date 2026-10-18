package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/auth"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/observer"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/predefined"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/schema"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

// ---- configuration ------------------------------------------------------------------

var nameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)

func redact(t *m.Tree) {
	for i := range t.Users {
		t.Users[i].Spec.PasswordHash = ""
	}
}

type kindInfo struct {
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Namespace string `json:"namespace"`
	Extension bool   `json:"extension"`
}

func (s *Server) findings(rev string, t *m.Tree) []validator.Error {
	if f, ok := s.findCache.Load(rev); ok {
		return f.([]validator.Error)
	}
	f := validator.Validate(t)
	if f == nil {
		f = []validator.Error{}
	}
	s.findCache.Store(rev, f)
	return f
}

func (s *Server) config(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	t, rev, err := s.Store.Read("HEAD")
	if err != nil {
		s.fail(w, err)
		return
	}
	findings := s.findings(rev, t)
	redact(t)
	var kinds []kindInfo
	for _, k := range m.Kinds {
		kinds = append(kinds, kindInfo{k.Name, k.Dir, k.Namespace, k.Extension})
	}
	w.Header().Set("ETag", `"`+rev+`"`)
	writeJSON(w, 200, map[string]any{
		"revision":     rev,
		"apiVersion":   m.APIVersion,
		"objects":      t,
		"findings":     findings,
		"kinds":        kinds,
		"activeRoutes": observer.ActiveRoutes(t),
		"predefined":   map[string][]string{"Color": predefined.PaletteNames(), "Service": predefinedServices()},
		"limits": map[string]int{"policiesPerRuleSet": validator.MaxPoliciesPerRuleSet, "ruleSets": validator.MaxRuleSets,
			"dnatEntries": validator.MaxDNATEntries},
	})
}

func predefinedServices() []string {
	var out []string
	for _, s := range predefined.Services() {
		out = append(out, s.Metadata.Name)
	}
	return out
}

func (s *Server) objectYAML(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	if _, ok := m.KindByName(kind); !ok || !nameRe.MatchString(name) {
		writeErr(w, 404, "unknown object")
		return
	}
	rev := r.URL.Query().Get("rev")
	if rev == "" {
		rev = "HEAD"
	}
	b, err := s.Store.File(rev, m.FilePath(kind, name))
	if err != nil {
		writeErr(w, 404, "not found at "+rev)
		return
	}
	if kind == "User" {
		var lines []string
		for _, l := range strings.Split(string(b), "\n") {
			if i := strings.Index(l, "passwordHash:"); i >= 0 {
				l = l[:i] + `passwordHash: "<redacted>"`
			}
			lines = append(lines, l)
		}
		b = []byte(strings.Join(lines, "\n"))
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) schemaFile(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	b, ok := schema.Files()[r.PathValue("file")]
	if !ok {
		writeErr(w, 404, "unknown schema file")
		return
	}
	w.Header().Set("Content-Type", "application/schema+json")
	_, _ = w.Write(b)
}

// key identifies a finding independently of its line, which moves when
// unrelated lines of the same file change.
func key(e validator.Error) string {
	return e.File + "\x00" + e.Path + "\x00" + e.Rule + "\x00" + e.Reason
}

// newErrors keeps only the errors that HEAD did not already have, so a
// change is judged on what it breaks, not on pre-existing problems (e.g.
// after a manual edit or an import made outside the UI).
func newErrors(before, after []validator.Error) []validator.Error {
	old := map[string]bool{}
	for _, e := range before {
		old[key(e)] = true
	}
	var out []validator.Error
	for _, e := range validator.Errors(after) {
		if !old[key(e)] {
			out = append(out, e)
		}
	}
	return out
}

func warningsFor(all []validator.Error, file string) []validator.Error {
	var out []validator.Error
	for _, e := range all {
		if e.Severity == validator.SevWarning && e.File == file {
			out = append(out, e)
		}
	}
	return out
}

// parseObject reads a request body (JSON from the forms, or YAML from the
// YAML editor) and checks it against the schema before it is typed, so that
// an unknown field or a wrong type is reported with its path (R-020.3).
func parseObject(r *http.Request, kind, name string) (m.Doc, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, &apiError{code: 400, msg: err.Error()}
	}
	file := m.FilePath(kind, name)
	var value any
	var doc *m.RawDoc
	if ct := r.Header.Get("Content-Type"); strings.Contains(ct, "yaml") {
		docs, err := m.SplitDocs(body)
		if err != nil {
			return nil, &apiError{code: 422, msg: "not valid YAML", validation: []validator.Error{{File: file, Reason: err.Error(), Rule: "yaml", Severity: validator.SevError}}}
		}
		if len(docs) != 1 {
			return nil, &apiError{code: 422, msg: fmt.Sprintf("expected one YAML document, got %d", len(docs))}
		}
		doc, value = &docs[0], docs[0].Value
	} else {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.UseNumber()
		if err := dec.Decode(&value); err != nil {
			return nil, &apiError{code: 400, msg: "invalid JSON: " + err.Error()}
		}
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, &apiError{code: 422, msg: "the object must be a mapping"}
	}
	if _, set := obj["apiVersion"]; !set {
		obj["apiVersion"] = m.APIVersion
	}
	if _, set := obj["kind"]; !set {
		obj["kind"] = kind
	}
	if obj["kind"] != kind {
		return nil, &apiError{code: 400, msg: fmt.Sprintf("the document declares kind %v, the URL %s", obj["kind"], kind)}
	}
	if issues := schema.Validate(obj); len(issues) > 0 {
		var errs []validator.Error
		for _, is := range issues {
			e := validator.Error{File: file, Path: is.Path, Reason: is.Reason, Rule: "schema", Constraint: "C-005", Severity: validator.SevError}
			if doc != nil {
				e.Line = m.LineOf(doc.Node, is.Path)
			}
			errs = append(errs, e)
		}
		return nil, &apiError{code: 422, msg: "the object does not match the schema (" + m.APIVersion + ")", validation: errs}
	}
	b, _ := json.Marshal(obj)
	t := &m.Tree{}
	n, err := t.PutJSON(kind, b)
	if err != nil {
		return nil, &apiError{code: 400, msg: err.Error()}
	}
	if n != name {
		return nil, &apiError{code: 400, msg: "metadata.name must match the URL (renaming is not supported: references are by name)"}
	}
	o, _ := t.Get(kind, name)
	return o, nil
}

// mutation builds the change for PUT/DELETE of one object: it protects the
// predefined objects (DM-21) and the System singleton, keeps uuids, and
// validates the resulting tree. warnings receives the findings of the file.
func mutation(kind, name string, obj m.Doc, create, del bool, before, after *any, warnings *[]validator.Error) func(t *m.Tree) ([]configstore.Change, error) {
	return func(t *m.Tree) ([]configstore.Change, error) {
		if _, ok := m.KindByName(kind); !ok {
			return nil, &apiError{code: 404, msg: "unknown kind " + kind}
		}
		baseline := validator.Validate(t)
		old, exists := t.Get(kind, name)
		if exists {
			*before = old
		}
		switch {
		case create && exists:
			return nil, &apiError{code: 409, msg: fmt.Sprintf("%s %q already exists", kind, name)}
		case !create && !exists:
			return nil, &apiError{code: 404, msg: fmt.Sprintf("%s %q does not exist", kind, name)}
		case kind == "System" && (create || del):
			return nil, &apiError{code: 422, msg: "System is a singleton: it can be edited, not created or deleted"}
		}
		var change configstore.Change
		if del {
			if _, shipped := predefined.ColorHex(name); kind == "Color" && shipped {
				return nil, &apiError{code: 422, msg: "predefined Colors cannot be deleted (DM-21); edit their comment instead"}
			}
			if kind == "Service" && predefined.IsService(name) {
				return nil, &apiError{code: 422, msg: "predefined Services cannot be deleted (DM-21)"}
			}
			t.Delete(kind, name)
			change = configstore.Change{Kind: kind, Name: name}
		} else {
			obj = m.AssignUUIDs(obj, old)
			if err := t.Put(obj); err != nil {
				return nil, &apiError{code: 400, msg: err.Error()}
			}
			*after = obj
			change = configstore.Change{Kind: kind, Name: name, Obj: obj}
		}
		all := validator.Validate(t)
		if errs := newErrors(baseline, all); len(errs) > 0 {
			return nil, &apiError{code: 422, msg: "the change is not valid", validation: errs, warnings: warningsFor(all, m.FilePath(kind, name))}
		}
		*warnings = warningsFor(all, m.FilePath(kind, name))
		return []configstore.Change{change}, nil
	}
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	if kind == "User" {
		writeErr(w, 400, "use PUT /api/v1/users/{name}")
		return
	}
	base := revision(r)
	if base == "" {
		writeErr(w, 428, "If-Match with the revision you edited is required")
		return
	}
	obj, err := parseObject(r, kind, name)
	if err != nil {
		s.fail(w, err)
		return
	}
	create := r.URL.Query().Get("create") == "1"
	verb := "update"
	if create {
		verb = "create"
	}
	var before, after any
	var warns []validator.Error
	commit, err := s.Store.Update(base, configstore.Author{Name: ss.User, Source: clientIP(r)},
		fmt.Sprintf("%s %s/%s", verb, kind, name), mutation(kind, name, obj, create, false, &before, &after, &warns))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Audit.Record(ss.User, clientIP(r), "config."+verb, kind+"/"+name, "commit "+commit[:8], before, after)
	s.respondApplied(w, commit, warns)
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	base := revision(r)
	if base == "" {
		writeErr(w, 428, "If-Match with the revision you edited is required")
		return
	}
	var before, after any
	var warns []validator.Error
	commit, err := s.Store.Update(base, configstore.Author{Name: ss.User, Source: clientIP(r)},
		fmt.Sprintf("delete %s/%s", kind, name), mutation(kind, name, nil, false, true, &before, &after, &warns))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u, ok := before.(m.User); ok {
		u.Spec.PasswordHash = ""
		before = u
	}
	s.Audit.Record(ss.User, clientIP(r), "config.delete", kind+"/"+name, "commit "+commit[:8], before, nil)
	s.respondApplied(w, commit, nil)
}

// validate is a dry run used by the forms and the YAML editor for immediate
// feedback (R-009.1, R-020.3). Nothing is committed.
func (s *Server) validate(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	reply := func(err error, warns []validator.Error) {
		var ae *apiError
		if errors.As(err, &ae) {
			writeJSON(w, 200, map[string]any{"valid": false, "error": ae.msg, "validation": ae.validation, "warnings": ae.warnings})
			return
		}
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"valid": true, "warnings": warns})
	}
	obj, err := parseObject(r, kind, name)
	if err != nil {
		reply(err, nil)
		return
	}
	t, _, err := s.Store.Read("HEAD")
	if err != nil {
		s.fail(w, err)
		return
	}
	var before, after any
	var warns []validator.Error
	_, err = mutation(kind, name, obj, r.URL.Query().Get("create") == "1", false, &before, &after, &warns)(t)
	reply(err, warns)
}

func (s *Server) putUser(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	name := r.PathValue("name")
	var req struct {
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, 400, "invalid request")
		return
	}
	if !nameRe.MatchString(name) {
		writeErr(w, 422, "user names follow the object name rules (lowercase, digits, '.', '_', '-')")
		return
	}
	if req.Password != "" && len(req.Password) < 8 {
		writeErr(w, 422, "password must have at least 8 characters")
		return
	}
	var hash string
	if req.Password != "" {
		var err error
		if hash, err = auth.HashPassword(req.Password); err != nil {
			s.fail(w, err)
			return
		}
	}
	verb := "config.update"
	commit, err := s.Store.Update(revision(r), configstore.Author{Name: ss.User, Source: clientIP(r)}, "update User/"+name, func(t *m.Tree) ([]configstore.Change, error) {
		old, exists := t.Get("User", name)
		u := m.User{APIVersion: m.APIVersion, Kind: "User", Metadata: m.Meta{Name: name}, Spec: m.UserSpec{Role: req.Role, PasswordHash: hash}}
		if exists {
			ou := old.(m.User)
			u.Metadata = ou.Metadata
			if hash == "" {
				u.Spec.PasswordHash = ou.Spec.PasswordHash
			}
		} else {
			verb = "config.create"
			if hash == "" {
				return nil, &apiError{code: 422, msg: "a password is required for a new User"}
			}
		}
		var before, after any
		var warns []validator.Error
		return mutation("User", name, u, !exists, false, &before, &after, &warns)(t)
	})
	if err != nil {
		s.fail(w, err)
		return
	}
	detail := "commit " + commit[:8]
	if req.Password != "" {
		detail += ", password set"
	}
	s.Audit.Record(ss.User, clientIP(r), verb, "User/"+name, detail, nil, map[string]string{"role": req.Role})
	s.respondApplied(w, commit, nil)
}

// respondApplied waits for the reconciliation of the commit, so the UI can
// say "saved and applied" or "saved, not applied: reason" (reconciliation.md §5).
func (s *Server) respondApplied(w http.ResponseWriter, commit string, warnings []validator.Error) {
	a, done := s.Rec.Wait(commit, s.ApplyWait)
	writeJSON(w, 200, map[string]any{
		"commit":   commit,
		"applied":  done && (a.Result == "applied" || a.Result == "no-op"),
		"pending":  !done,
		"attempt":  a,
		"warnings": warnings,
	})
}

// ---- import / export (R-020, R-020.1) ------------------------------------------------------

// export returns the repository at a revision as a tar.gz: the files are the
// Configuration, so the export is the repository content (ADR-0001).
func (s *Server) export(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	rev := r.URL.Query().Get("rev")
	if rev == "" {
		rev = "HEAD"
	}
	id, err := s.Store.Resolve(rev)
	if err != nil {
		writeErr(w, 404, "unknown revision")
		return
	}
	b, err := s.Store.Archive(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Audit.Record(ss.User, clientIP(r), "config.export", "", "revision "+id[:8], nil, nil)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="songbird-config-%s.tar.gz"`, id[:8]))
	_, _ = w.Write(b)
}

// importConfig replaces the whole Configuration with a set of YAML
// documents (one or many per file, any order), in one commit. Users are kept
// when the import has none, so the administrator does not lock themselves
// out. Nothing is committed unless the result is valid (R-020.2, R-020.3).
func (s *Server) importConfig(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	const file = "import.yaml"
	in, src := &m.Tree{}, validator.Sources{}
	if errs := validator.ParseDocs(in, file, body, src); len(errs) > 0 {
		writeJSON(w, 422, map[string]any{"error": "the import does not match the schema", "validation": errs})
		return
	}
	in.Sort()
	count := 0
	in.Each(func(m.Doc) { count++ })
	var warns []validator.Error
	commit, err := s.Store.Update(revision(r), configstore.Author{Name: ss.User, Source: clientIP(r)}, fmt.Sprintf("import %d objects", count),
		func(head *m.Tree) ([]configstore.Change, error) {
			if len(in.Users) == 0 {
				in.Users = head.Users
			}
			all := validator.Validate(in)
			if errs := validator.Errors(all); len(errs) > 0 {
				return nil, &apiError{code: 422, msg: fmt.Sprintf("the import is not valid: %d error(s), nothing was changed", len(errs)), validation: validator.Relocate(errs, in, src)}
			}
			for _, e := range all {
				if e.Severity == validator.SevWarning {
					warns = append(warns, e)
				}
			}
			warns = validator.Relocate(warns, in, src)
			var changes []configstore.Change
			head.Each(func(o m.Doc) {
				kind, meta := o.Header()
				if _, keep := in.Get(kind, meta.Name); !keep {
					changes = append(changes, configstore.Change{Kind: kind, Name: meta.Name})
				}
			})
			in.Each(func(o m.Doc) {
				kind, meta := o.Header()
				prev, _ := head.Get(kind, meta.Name)
				changes = append(changes, configstore.Change{Kind: kind, Name: meta.Name, Obj: m.AssignUUIDs(o, prev)})
			})
			return changes, nil
		})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Audit.Record(ss.User, clientIP(r), "config.import", "", fmt.Sprintf("%d objects, commit %s", count, commit[:8]), nil, nil)
	s.respondApplied(w, commit, warns)
}
