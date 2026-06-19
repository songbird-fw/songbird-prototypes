package model

import "time"

type User struct {
	ID           string
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
}

type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

type AuditEvent struct {
	ID        string
	ActorID   string
	Action    string
	Target    string
	Payload   string
	CreatedAt time.Time
}
