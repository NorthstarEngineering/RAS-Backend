package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
)

const userColumns = `id, name, email, password_hash, role, branch_id, is_active,
	must_change_password, failed_login_attempts, lockout_count, locked_until,
	last_login_at, created_at, updated_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.BranchID,
		&u.IsActive, &u.MustChangePassword, &u.FailedLoginAttempts, &u.LockoutCount,
		&u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, autherrors.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func branchArg(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func isEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_lower_uq"
}

type UserRepository struct{ db *sql.DB }

func NewUserRepository(db *sql.DB) *UserRepository { return &UserRepository{db: db} }

func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, name, email, password_hash, role, branch_id,
			is_active, must_change_password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
		u.ID, u.Name, u.Email, u.PasswordHash, string(u.Role), branchArg(u.BranchID),
		u.IsActive, u.MustChangePassword, u.CreatedAt)
	if isEmailConflict(err) {
		return autherrors.ErrEmailTaken
	}
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE LOWER(email) = LOWER($1)`, email))
}

func (r *UserRepository) List(ctx context.Context) ([]*model.User, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *UserRepository) ExistsByRole(ctx context.Context, role model.Role) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE role = $1)`, string(role)).Scan(&exists)
	return exists, err
}

// exec runs an UPDATE on a single user and reports ErrUserNotFound if nothing matched.
func (r *UserRepository) exec(ctx context.Context, query string, args ...any) error {
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return autherrors.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string, mustChange bool, now time.Time) error {
	return r.exec(ctx, `
		UPDATE users
		SET password_hash = $2, must_change_password = $3,
		    failed_login_attempts = 0, lockout_count = 0, locked_until = NULL, updated_at = $4
		WHERE id = $1`, id, hash, mustChange, now)
}

func (r *UserRepository) UpdateEmail(ctx context.Context, id uuid.UUID, email string, now time.Time) error {
	err := r.exec(ctx, `UPDATE users SET email = $2, updated_at = $3 WHERE id = $1`, id, email, now)
	if isEmailConflict(err) {
		return autherrors.ErrEmailTaken
	}
	return err
}

func (r *UserRepository) SetActive(ctx context.Context, id uuid.UUID, active bool, now time.Time) error {
	return r.exec(ctx, `UPDATE users SET is_active = $2, updated_at = $3 WHERE id = $1`, id, active, now)
}

// RegisterFailedAttempt increments the counter atomically (safe under concurrent guesses).
func (r *UserRepository) RegisterFailedAttempt(ctx context.Context, id uuid.UUID, now time.Time) (int, int, error) {
	var attempts, lockouts int
	err := r.db.QueryRowContext(ctx, `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1, updated_at = $2
		WHERE id = $1
		RETURNING failed_login_attempts, lockout_count`, id, now).Scan(&attempts, &lockouts)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, autherrors.ErrUserNotFound
	}
	return attempts, lockouts, err
}

func (r *UserRepository) Lock(ctx context.Context, id uuid.UUID, until, now time.Time) error {
	return r.exec(ctx, `
		UPDATE users
		SET locked_until = $2, failed_login_attempts = 0,
		    lockout_count = lockout_count + 1, updated_at = $3
		WHERE id = $1`, id, until, now)
}

func (r *UserRepository) RegisterSuccessfulLogin(ctx context.Context, id uuid.UUID, now time.Time) error {
	return r.exec(ctx, `
		UPDATE users
		SET failed_login_attempts = 0, lockout_count = 0, locked_until = NULL,
		    last_login_at = $2, updated_at = $2
		WHERE id = $1`, id, now)
}

func (r *UserRepository) Unlock(ctx context.Context, id uuid.UUID, now time.Time) error {
	return r.exec(ctx, `
		UPDATE users
		SET failed_login_attempts = 0, lockout_count = 0, locked_until = NULL, updated_at = $2
		WHERE id = $1`, id, now)
}
