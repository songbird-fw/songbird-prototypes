// Package api is the API Server: it authenticates requests, turns edits into
// commits through the Config Manager and reads status through the Observer
// and the Reconciler. It never calls drivers (components.md, rule 2).
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/audit"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/auth"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/configstore"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/events"
	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/observer"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/reconciler"
	"github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/validator"
)

const cookieName = "songbird_session"

type Server struct {
	Store     *configstore.Store
	Rec       *reconciler.Reconciler
	Auth      *auth.Auth
	Audit     *audit.Log
	Ev        *events.Collector
	Obs       *observer.Observer
	Web       fs.FS
	ApplyWait time.Duration
}

type handler func(w http.ResponseWriter, r *http.Request, s auth.Session)

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("POST /api/v1/login", s.login)
	mux.HandleFunc("POST /api/v1/logout", s.authed(s.logout))
	mux.HandleFunc("GET /api/v1/me", s.authed(func(w http.ResponseWriter, r *http.Request, ss auth.Session) { writeJSON(w, 200, ss) }))
	mux.HandleFunc("GET /api/v1/config", s.authed(s.config))
	mux.HandleFunc("GET /api/v1/objects/{kind}/{name}/yaml", s.authed(s.objectYAML))
	mux.HandleFunc("PUT /api/v1/objects/{kind}/{name}", s.writer(s.putObject))
	mux.HandleFunc("DELETE /api/v1/objects/{kind}/{name}", s.writer(s.deleteObject))
	mux.HandleFunc("POST /api/v1/validate/{kind}/{name}", s.authed(s.validate))
	mux.HandleFunc("PUT /api/v1/users/{name}", s.writer(s.putUser))
	mux.HandleFunc("GET /api/v1/status", s.authed(func(w http.ResponseWriter, r *http.Request, _ auth.Session) { writeJSON(w, 200, s.Rec.Status()) }))
	mux.HandleFunc("GET /api/v1/history", s.authed(s.history))
	mux.HandleFunc("GET /api/v1/history/{rev}/diff", s.authed(s.diff))
	mux.HandleFunc("POST /api/v1/history/{rev}/revert", s.writer(s.revert))
	mux.HandleFunc("GET /api/v1/metrics", s.authed(func(w http.ResponseWriter, r *http.Request, _ auth.Session) { writeJSON(w, 200, s.Obs.Samples()) }))
	mux.HandleFunc("GET /api/v1/events", s.authed(s.recentEvents))
	mux.HandleFunc("GET /api/v1/events/stream", s.authed(s.stream))
	mux.HandleFunc("GET /api/v1/audit", s.authed(s.auditTail))
	mux.HandleFunc("GET /api/v1/audit/verify", s.authed(func(w http.ResponseWriter, r *http.Request, _ auth.Session) { writeJSON(w, 200, s.Audit.Verify()) }))
	mux.HandleFunc("POST /api/v1/diagnostics/faults", s.writer(s.fault))
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "not found") })
	mux.Handle("GET /", s.static())
	return secure(mux)
}

// ---- plumbing ---------------------------------------------------------------------

func secure(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hd := w.Header()
		hd.Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		hd.Set("X-Content-Type-Options", "nosniff")
		hd.Set("Referrer-Policy", "no-referrer")
		// CSRF: the session cookie is SameSite=Strict, and every write must
		// carry a header a cross-site form cannot set.
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Requested-With") != "songbird" {
			writeErr(w, 403, "missing X-Requested-With header")
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (s *Server) static() http.Handler {
	files := http.FileServerFS(s.Web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.Web, p); err != nil {
			r.URL.Path = "/" // SPA fallback
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{"error": msg})
}

func clientIP(r *http.Request) string {
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return h
}

func (s *Server) session(r *http.Request) (auth.Session, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return auth.Session{}, false
	}
	return s.Auth.Verify(c.Value)
}

// authed requires a valid session (R-043).
func (s *Server) authed(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ss, ok := s.session(r)
		if !ok {
			writeErr(w, 401, "authentication required")
			return
		}
		h(w, r, ss)
	}
}

// writer additionally requires the admin Role (R-043.9: read-only gets 403).
func (s *Server) writer(h handler) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request, ss auth.Session) {
		if ss.Role != "admin" {
			writeErr(w, 403, "the read-only Role cannot change anything")
			return
		}
		h(w, r, ss)
	})
}

type apiError struct {
	code       int
	msg        string
	validation []validator.Error
}

func (e *apiError) Error() string { return e.msg }

func (s *Server) fail(w http.ResponseWriter, err error) {
	var ae *apiError
	var ce *configstore.ConflictError
	switch {
	case errors.As(err, &ae):
		body := map[string]any{"error": ae.msg}
		if ae.validation != nil {
			body["validation"] = ae.validation
		}
		writeJSON(w, ae.code, body)
	case errors.As(err, &ce):
		writeJSON(w, 409, map[string]any{"error": ce.Error(), "conflict": map[string]string{"file": ce.File, "head": ce.Head}})
	case errors.Is(err, configstore.ErrNoChange):
		writeErr(w, 409, "nothing changed")
	default:
		writeErr(w, 500, err.Error())
	}
}

func revision(r *http.Request) string {
	return strings.Trim(r.Header.Get("If-Match"), `" `)
}

// ---- health, login --------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.Obs.Health()) // R-032: always 200, status in the body
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Username, Password string }
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, 400, "invalid request")
		return
	}
	ip := clientIP(r)
	token, ss, locked, err := s.Auth.Login(req.Username, req.Password, ip)
	switch {
	case errors.Is(err, auth.ErrLocked):
		s.Audit.Record(req.Username, ip, "auth.login-refused", "", "source address locked out", nil, nil)
		writeErr(w, 429, err.Error())
		return
	case err != nil:
		s.Audit.Record(req.Username, ip, "auth.login-failed", "", "", nil, nil)
		if locked {
			s.Audit.Record(req.Username, ip, "auth.lockout", "", fmt.Sprintf("%d failures, locked for %s", auth.MaxFailures, auth.LockoutFor), nil, nil)
		}
		writeErr(w, 401, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil,
		SameSite: http.SameSiteStrictMode, Expires: time.Unix(ss.Exp, 0)})
	s.Audit.Record(ss.User, ip, "auth.login", "", "", nil, nil)
	writeJSON(w, 200, ss)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	s.Audit.Record(ss.User, clientIP(r), "auth.logout", "", "", nil, nil)
	writeJSON(w, 200, map[string]string{"ok": "bye"})
}

// ---- configuration ---------------------------------------------------------------

func redact(t *m.Tree) {
	for i := range t.Users {
		t.Users[i].Spec.PasswordHash = ""
	}
}

func (s *Server) config(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	t, rev, err := s.Store.Read("HEAD")
	if err != nil {
		s.fail(w, err)
		return
	}
	redact(t)
	w.Header().Set("ETag", `"`+rev+`"`)
	writeJSON(w, 200, map[string]any{
		"revision":     rev,
		"objects":      t,
		"activeRoutes": observer.ActiveRoutes(t),
		"limits":       map[string]int{"policiesPerZone": validator.MaxPoliciesPerZone},
	})
}

func (s *Server) objectYAML(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	if _, ok := m.KindByName(kind); !ok || !m.ValidName(name) {
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
			if strings.Contains(l, "passwordHash:") {
				l = "  passwordHash: <redacted>"
			}
			lines = append(lines, l)
		}
		b = []byte(strings.Join(lines, "\n"))
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(b)
}

// newErrors keeps only errors that HEAD did not already have, so a change is
// judged on what it breaks, not on pre-existing problems (e.g. after a pull).
func newErrors(before, after []validator.Error) []validator.Error {
	old := map[string]bool{}
	for _, e := range before {
		old[e.String()] = true
	}
	var out []validator.Error
	for _, e := range after {
		if !old[e.String()] {
			out = append(out, e)
		}
	}
	return out
}

// mutation builds the change for PUT/DELETE of one object, enforcing DM-21
// and validating the resulting tree.
func mutation(kind, name string, body []byte, create, del bool, before, after *any) func(t *m.Tree) ([]configstore.Change, error) {
	return func(t *m.Tree) ([]configstore.Change, error) {
		if _, ok := m.KindByName(kind); !ok {
			return nil, &apiError{code: 404, msg: "unknown kind " + kind}
		}
		baseline := validator.Validate(t)
		old, exists := t.Get(kind, name)
		*before = old
		switch {
		case create && exists:
			return nil, &apiError{code: 409, msg: fmt.Sprintf("%s %q already exists", kind, name)}
		case !create && !exists:
			return nil, &apiError{code: 404, msg: fmt.Sprintf("%s %q does not exist", kind, name)}
		}
		var change configstore.Change
		if del {
			if validator.Builtin(old) {
				return nil, &apiError{code: 422, msg: "predefined objects cannot be deleted (DM-21)"}
			}
			t.Delete(kind, name)
			change = configstore.Change{Kind: kind, Name: name}
		} else {
			n, err := t.PutJSON(kind, body)
			if err != nil {
				return nil, &apiError{code: 400, msg: err.Error()}
			}
			if n != name {
				return nil, &apiError{code: 400, msg: "metadata.name must match the URL (renaming is not supported)"}
			}
			obj, _ := t.Get(kind, name)
			if exists && validator.Builtin(old) {
				if c, ok := old.(m.Colour); ok { // R-055.1: only meaning may change
					nc := obj.(m.Colour)
					if nc.Spec.Value != c.Spec.Value || !nc.Spec.Builtin {
						return nil, &apiError{code: 422, msg: "the value of a predefined Colour cannot change (R-055.1)"}
					}
				} else if !reflect.DeepEqual(old, obj) {
					return nil, &apiError{code: 422, msg: "predefined objects cannot be edited (DM-21)"}
				}
			}
			if validator.Builtin(obj) && (!exists || !validator.Builtin(old)) {
				return nil, &apiError{code: 422, msg: "builtin is reserved for predefined objects (DM-21)"}
			}
			*after = obj
			change = configstore.Change{Kind: kind, Name: name, Obj: obj}
		}
		if errs := newErrors(baseline, validator.Validate(t)); len(errs) > 0 {
			return nil, &apiError{code: 422, msg: "the change is not valid", validation: errs}
		}
		return []configstore.Change{change}, nil
	}
}

func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, 1<<20))
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	if kind == "User" {
		writeErr(w, 400, "use PUT /api/v1/users/{name}")
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	base := revision(r)
	if base == "" {
		writeErr(w, 428, "If-Match with the revision you edited is required")
		return
	}
	create := r.URL.Query().Get("create") == "1"
	verb := "update"
	if create {
		verb = "create"
	}
	var before, after any
	commit, err := s.Store.Update(base, configstore.Author{Name: ss.User, Source: clientIP(r)},
		fmt.Sprintf("%s %s/%s", verb, kind, name), mutation(kind, name, body, create, false, &before, &after))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Audit.Record(ss.User, clientIP(r), "config."+verb, kind+"/"+name, "commit "+commit[:8], before, after)
	s.respondApplied(w, commit)
}

func (s *Server) deleteObject(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	base := revision(r)
	if base == "" {
		writeErr(w, 428, "If-Match with the revision you edited is required")
		return
	}
	var before, after any
	commit, err := s.Store.Update(base, configstore.Author{Name: ss.User, Source: clientIP(r)},
		fmt.Sprintf("delete %s/%s", kind, name), mutation(kind, name, nil, false, true, &before, &after))
	if err != nil {
		s.fail(w, err)
		return
	}
	if u, ok := before.(m.User); ok {
		u.Spec.PasswordHash = ""
		before = u
	}
	s.Audit.Record(ss.User, clientIP(r), "config.delete", kind+"/"+name, "commit "+commit[:8], before, nil)
	s.respondApplied(w, commit)
}

// validate is a dry run used by the forms for immediate feedback (R-009.1).
func (s *Server) validate(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	kind, name := r.PathValue("kind"), r.PathValue("name")
	body, err := readBody(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	t, _, err := s.Store.Read("HEAD")
	if err != nil {
		s.fail(w, err)
		return
	}
	var before, after any
	_, err = mutation(kind, name, body, r.URL.Query().Get("create") == "1", false, &before, &after)(t)
	var ae *apiError
	if errors.As(err, &ae) {
		writeJSON(w, 200, map[string]any{"valid": false, "error": ae.msg, "validation": ae.validation})
		return
	}
	writeJSON(w, 200, map[string]any{"valid": err == nil})
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
	var verb string
	commit, err := s.Store.Update(revision(r), configstore.Author{Name: ss.User, Source: clientIP(r)}, "update User/"+name, func(t *m.Tree) ([]configstore.Change, error) {
		baseline := validator.Validate(t)
		old, exists := t.Get("User", name)
		u := m.User{APIVersion: m.APIVersion, Kind: "User", Metadata: m.Meta{Name: name}, Spec: m.UserSpec{Role: req.Role, PasswordHash: hash}}
		verb = "config.update"
		if exists {
			if hash == "" {
				u.Spec.PasswordHash = old.(m.User).Spec.PasswordHash
			}
		} else {
			verb = "config.create"
			if hash == "" {
				return nil, &apiError{code: 422, msg: "a password is required for a new User"}
			}
		}
		b, _ := json.Marshal(u)
		if _, err := t.PutJSON("User", b); err != nil {
			return nil, &apiError{code: 400, msg: err.Error()}
		}
		if errs := newErrors(baseline, validator.Validate(t)); len(errs) > 0 {
			return nil, &apiError{code: 422, msg: "the change is not valid", validation: errs}
		}
		return []configstore.Change{{Kind: "User", Name: name, Obj: u}}, nil
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
	s.respondApplied(w, commit)
}

// respondApplied waits for the reconciliation of the commit, so the UI can
// say "saved and applied" or "saved, not applied: reason" (reconciliation.md §5).
func (s *Server) respondApplied(w http.ResponseWriter, commit string) {
	a, done := s.Rec.Wait(commit, s.ApplyWait)
	writeJSON(w, 200, map[string]any{
		"commit":  commit,
		"applied": done && (a.Result == "applied" || a.Result == "no-op"),
		"pending": !done,
		"attempt": a,
	})
}

// ---- history -----------------------------------------------------------------------

func (s *Server) history(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n <= 0 || n > 500 {
		n = 100
	}
	h, err := s.Store.History(n)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, h)
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	out, err := s.Store.Diff(r.URL.Query().Get("from"), r.PathValue("rev"))
	if err != nil {
		writeErr(w, 404, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, out)
}

func (s *Server) revert(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	rev := r.PathValue("rev")
	commit, err := s.Store.Revert(rev, configstore.Author{Name: ss.User, Source: clientIP(r)})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.Audit.Record(ss.User, clientIP(r), "config.revert", "", "to "+rev[:min(8, len(rev))]+", commit "+commit[:8], nil, nil)
	s.respondApplied(w, commit)
}

// ---- events, audit, diagnostics ---------------------------------------------------------

func (s *Server) recentEvents(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 0 || n > 2000 {
		n = 500
	}
	writeJSON(w, 200, map[string]any{"events": s.Ev.Recent(n), "counters": s.Ev.Counters()})
}

// stream relays the Event Collector as server-sent events (R-028).
func (s *Server) stream(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, 500, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	ch, cancel := s.Ev.Subscribe()
	defer cancel()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	fmt.Fprint(w, ": connected\n\n")
	fl.Flush()
	for {
		select {
		case e := <-ch:
			b, _ := json.Marshal(e)
			fmt.Fprintf(w, "data: %s\n\n", b)
			// batch whatever is already queued before flushing
			for drained := false; !drained; {
				select {
				case e := <-ch:
					b, _ := json.Marshal(e)
					fmt.Fprintf(w, "data: %s\n\n", b)
				default:
					drained = true
				}
			}
			fl.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) auditTail(w http.ResponseWriter, r *http.Request, _ auth.Session) {
	e, err := s.Audit.Tail(300)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, e)
}

func (s *Server) fault(w http.ResponseWriter, r *http.Request, ss auth.Session) {
	var req struct {
		Driver  string `json:"driver"`
		Fail    bool   `json:"fail"`
		Stopped bool   `json:"stopped"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
		writeErr(w, 400, "invalid request")
		return
	}
	known := false
	for _, d := range reconciler.DriverNames {
		known = known || d == req.Driver
	}
	if !known {
		writeErr(w, 404, "unknown driver")
		return
	}
	s.Rec.SetFault(req.Driver, req.Fail, req.Stopped)
	s.Audit.Record(ss.User, clientIP(r), "diagnostics.fault", req.Driver, fmt.Sprintf("failNextApply=%v stopped=%v", req.Fail, req.Stopped), nil, nil)
	writeJSON(w, 200, s.Rec.Status())
}
