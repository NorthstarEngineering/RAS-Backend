package security

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	autherrors "restaurant-backend/internal/auth/errors"
	"restaurant-backend/internal/auth/model"
)

func TestQAJWTRejectsWrongSecretIssuerAndMalformedToken(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	user := &model.User{ID: uuid.New(), Role: model.RoleCEO}
	secret := []byte("0123456789abcdef0123456789abcdef")
	issuer := NewJWTManager(secret, "expected-issuer", time.Minute)
	token, _, err := issuer.Issue(user, now)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := NewJWTManager([]byte(strings.Repeat("x", 32)), "expected-issuer", time.Minute).Parse(token, now); err == nil {
		t.Fatal("Parse() accepted a token signed with another secret")
	}
	if _, err := NewJWTManager(secret, "another-issuer", time.Minute).Parse(token, now); err == nil {
		t.Fatal("Parse() accepted a token with another issuer")
	}

	branchID := uuid.New()
	branchUser := &model.User{ID: uuid.New(), Role: model.RoleBranchManager, BranchID: &branchID}
	branchToken, _, err := issuer.Issue(branchUser, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := issuer.Parse(branchToken, now)
	if err != nil || claims.BranchID == nil || *claims.BranchID != branchID {
		t.Fatalf("Parse() claims = %+v, error = %v; want branch %s", claims, err, branchID)
	}
	if _, err := issuer.Parse("not.a.jwt", now); err == nil {
		t.Fatal("Parse() accepted malformed token")
	}
}

func TestQAJWTRejectsClaimsThatImplementationValidates(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	secret := []byte("0123456789abcdef0123456789abcdef")
	manager := NewJWTManager(secret, "issuer", time.Minute)
	validSubject := uuid.NewString()

	tests := []struct {
		name   string
		claims jwtClaims
	}{
		{
			name: "unsupported role",
			claims: jwtClaims{Role: model.Role("UNSUPPORTED"), RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "issuer", Subject: validSubject, ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			}},
		},
		{
			name: "invalid subject UUID",
			claims: jwtClaims{Role: model.RoleCEO, RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "issuer", Subject: "not-a-uuid", ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			}},
		},
		{
			name: "malformed branch UUID",
			claims: jwtClaims{Role: model.RoleBranchManager, BranchID: "not-a-uuid", RegisteredClaims: jwt.RegisteredClaims{
				Issuer: "issuer", Subject: validSubject, ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tt.claims).SignedString(secret)
			if err != nil {
				t.Fatal(err)
			}
			claims, err := manager.Parse(raw, now)
			if claims != nil || !errors.Is(err, autherrors.ErrInvalidToken) {
				t.Fatalf("Parse() = (%+v, %v), want (nil, ErrInvalidToken)", claims, err)
			}
		})
	}
}

func TestQAJWTExpirationBoundaryAndMustChangePasswordClaim(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	manager := NewJWTManager([]byte("0123456789abcdef0123456789abcdef"), "issuer", time.Minute)
	token, expiresAt, err := manager.Issue(&model.User{ID: uuid.New(), Role: model.RoleCEO}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Parse(token, expiresAt); err == nil {
		t.Fatal("Parse() accepted token at its exact expiry time")
	}

	pending, _, err := manager.Issue(&model.User{ID: uuid.New(), Role: model.RoleCEO, MustChangePassword: true}, now)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(pending, now)
	if err != nil || !claims.MustChangePassword {
		t.Fatalf("Parse() pending-change claims = %+v, error = %v", claims, err)
	}
}

func TestQARefreshTokenIsOpaqueAndHashMatchesRawToken(t *testing.T) {
	raw, hash, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("refresh token decoded length = %d, error = %v; want 32 bytes", len(decoded), err)
	}
	if hash != HashToken(raw) || hash == raw {
		t.Fatalf("stored hash = %q; want hash of raw token, distinct from raw", hash)
	}
}

func TestQABcryptHasherComparesOnlyMatchingPassword(t *testing.T) {
	hasher := NewBcryptHasher(4)
	hash, err := hasher.Hash("CorrectPassword123")
	if err != nil {
		t.Fatal(err)
	}
	if !hasher.Compare(hash, "CorrectPassword123") {
		t.Fatal("Compare() rejected matching password")
	}
	if hasher.Compare(hash, "WrongPassword123") {
		t.Fatal("Compare() accepted a different password")
	}
	if hasher.Compare("not-a-bcrypt-hash", "CorrectPassword123") {
		t.Fatal("Compare() accepted malformed hash")
	}
}
