package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
)

type RefreshTokenRepository struct{ db *sql.DB }

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, t *model.RefreshToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)`, t.ID, t.UserID, t.TokenHash, t.ExpiresAt, t.CreatedAt)
	return err
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var t model.RefreshToken
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`, hash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, autherrors.ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, now)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE refresh_tokens SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now)
	return err
}
