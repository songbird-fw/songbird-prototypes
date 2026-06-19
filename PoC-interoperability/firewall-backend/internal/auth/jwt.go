package auth

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret, issuer, accessTTL, refreshTTL string) *JWTManager {
	a, _ := time.ParseDuration(accessTTL)
	r, _ := time.ParseDuration(refreshTTL)
	return &JWTManager{secret: []byte(secret), issuer: issuer, accessTTL: a, refreshTTL: r}
}

func (m *JWTManager) GenerateAccessToken(userID, role string) (string, time.Time, error) {
	exp := time.Now().Add(m.accessTTL)
	claims := jwt.MapClaims{"sub": userID, "role": role, "iss": m.issuer, "exp": exp.Unix()}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(m.secret)
	return s, exp, err
}

func (m *JWTManager) GenerateRefreshToken() (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil { return "", time.Time{}, err }
	return base64.RawURLEncoding.EncodeToString(b), time.Now().Add(m.refreshTTL), nil
}

func (m *JWTManager) ParseAccessToken(raw string) (*jwt.Token, error) {
	return jwt.Parse(raw, func(token *jwt.Token) (interface{}, error) { return m.secret, nil })
}
