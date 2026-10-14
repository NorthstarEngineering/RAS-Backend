package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
)

type jwtClaims struct {
	Role               model.Role `json:"role"`
	BranchID           string     `json:"branch_id,omitempty"`
	MustChangePassword bool       `json:"mcp,omitempty"`
	jwt.RegisteredClaims
}

// JWTManager issues and verifies short-lived HS256 access tokens.
type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewJWTManager(secret []byte, issuer string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: secret, issuer: issuer, ttl: ttl}
}

func (m *JWTManager) Issue(u *model.User, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(m.ttl)
	claims := jwtClaims{
		Role:               u.Role,
		MustChangePassword: u.MustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   u.ID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	if u.BranchID != nil {
		claims.BranchID = u.BranchID.String()
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

func (m *JWTManager) Parse(raw string, now time.Time) (*model.Claims, error) {
	parsed, err := jwt.ParseWithClaims(raw, &jwtClaims{},
		func(*jwt.Token) (any, error) { return m.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return now }),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid {
		return nil, autherrors.ErrInvalidToken
	}
	c, ok := parsed.Claims.(*jwtClaims)
	if !ok || !c.Role.Valid() {
		return nil, autherrors.ErrInvalidToken
	}
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return nil, autherrors.ErrInvalidToken
	}
	out := &model.Claims{UserID: userID, Role: c.Role, MustChangePassword: c.MustChangePassword}
	if c.BranchID != "" {
		b, err := uuid.Parse(c.BranchID)
		if err != nil {
			return nil, autherrors.ErrInvalidToken
		}
		out.BranchID = &b
	}
	return out, nil
}
