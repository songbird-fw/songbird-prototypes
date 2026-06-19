package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/agostino/firewall-backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repositories struct{ Pool *pgxpool.Pool }
func NewRepositories(pool *pgxpool.Pool) *Repositories { return &Repositories{Pool: pool} }
func HashToken(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func (r *Repositories) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	row := r.Pool.QueryRow(ctx, `select id, username, password_hash, role, created_at from users where username=$1`, username)
	var u model.User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil { return nil, err }
	return &u, nil
}

func (r *Repositories) StoreRefreshToken(ctx context.Context, userID, raw string, expiresAt time.Time) error {
	_, err := r.Pool.Exec(ctx, `insert into refresh_tokens (id, user_id, token_hash, expires_at, created_at) values ($1,$2,$3,$4,now())`, uuid.NewString(), userID, HashToken(raw), expiresAt)
	return err
}

func (r *Repositories) ValidateRefreshToken(ctx context.Context, raw string) (*model.RefreshToken, error) {
	row := r.Pool.QueryRow(ctx, `select id, user_id, token_hash, expires_at, revoked_at, created_at from refresh_tokens where token_hash=$1 and revoked_at is null`, HashToken(raw))
	var t model.RefreshToken
	if err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt); err != nil { return nil, err }
	return &t, nil
}

func (r *Repositories) RevokeRefreshToken(ctx context.Context, raw string) error {
	_, err := r.Pool.Exec(ctx, `update refresh_tokens set revoked_at=now() where token_hash=$1 and revoked_at is null`, HashToken(raw))
	return err
}

func (r *Repositories) Audit(ctx context.Context, actorID, action, target, payload string) error {
	_, err := r.Pool.Exec(ctx, `insert into audit_events (id, actor_id, action, target, payload, created_at) values ($1,$2,$3,$4,$5,now())`, uuid.NewString(), actorID, action, target, payload)
	return err
}
