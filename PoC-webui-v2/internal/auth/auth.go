// Package auth owns sessions and login lockout (components.md, Auth).
// Users and Roles are configuration and are read from the Config Manager.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	m "github.com/songbird-fw/songbird-prototypes/PoC-webui-v2/internal/model"
)

const (
	SessionTTL  = 8 * time.Hour    // R-043.5
	MaxFailures = 5                // R-043.2
	LockoutFor  = 10 * time.Minute // R-043.2
)

var (
	ErrLocked  = errors.New("too many failed attempts from this address, retry later")
	ErrInvalid = errors.New("invalid username or password")
)

type Session struct {
	User string `json:"u"`
	Role string `json:"r"`
	Exp  int64  `json:"e"`
}

type failures struct {
	n     int
	until time.Time
}

type Auth struct {
	key   []byte
	mu    sync.Mutex
	fails map[string]*failures
	dummy string // hash compared for unknown users, so timing does not reveal them
	Users func() m.List[m.UserSpec]
}

func New(users func() m.List[m.UserSpec]) *Auth {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	d, _ := bcrypt.GenerateFromPassword(k[:16], 12)
	return &Auth{key: k, fails: map[string]*failures{}, dummy: string(d), Users: users}
}

// Login checks credentials; the bool reports whether this attempt caused a
// lockout (so the caller can audit it).
func (a *Auth) Login(user, pass, source string) (string, Session, bool, error) {
	a.mu.Lock()
	f := a.fails[source]
	if f != nil && time.Now().Before(f.until) {
		a.mu.Unlock()
		return "", Session{}, false, ErrLocked
	}
	a.mu.Unlock()

	var found *m.User
	for _, u := range a.Users() {
		if u.Metadata.Name == user {
			found = &u
			break
		}
	}
	hash := a.dummy
	if found != nil {
		hash = found.Spec.PasswordHash
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) != nil || found == nil {
		a.mu.Lock()
		defer a.mu.Unlock()
		f := a.fails[source]
		if f == nil || !f.until.IsZero() { // first failure, or lockout expired
			f = &failures{}
			a.fails[source] = f
		}
		f.n++
		if f.n >= MaxFailures {
			f.until = time.Now().Add(LockoutFor)
			f.n = 0
			return "", Session{}, true, ErrInvalid
		}
		return "", Session{}, false, ErrInvalid
	}
	a.mu.Lock()
	delete(a.fails, source)
	a.mu.Unlock()
	s := Session{User: user, Role: found.Spec.Role, Exp: time.Now().Add(SessionTTL).Unix()}
	return a.sign(s), s, false, nil
}

func (a *Auth) sign(s Session) string {
	b, _ := json.Marshal(s)
	p := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(p))
	return p + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks signature and expiry, and that the User still exists with
// the same Role in the current configuration (a deleted User loses access).
func (a *Auth) Verify(token string) (Session, bool) {
	p, sig, ok := strings.Cut(token, ".")
	if !ok {
		return Session{}, false
	}
	mac := hmac.New(sha256.New, a.key)
	mac.Write([]byte(p))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return Session{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return Session{}, false
	}
	var s Session
	if json.Unmarshal(b, &s) != nil || time.Now().Unix() > s.Exp {
		return Session{}, false
	}
	for _, u := range a.Users() {
		if u.Metadata.Name == s.User {
			s.Role = u.Spec.Role
			return s, true
		}
	}
	return Session{}, false
}

func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 12)
	return string(h), err
}
