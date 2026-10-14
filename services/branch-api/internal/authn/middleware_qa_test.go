package authn

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func qaSignedToken(t *testing.T, secret []byte, header tokenHeader, claims tokenClaims) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal token header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal token claims: %v", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func qaClaims(role, branchID string, now time.Time) tokenClaims {
	return tokenClaims{
		Subject:   uuid.NewString(),
		Role:      role,
		BranchID:  branchID,
		ExpiresAt: now.Add(time.Minute).Unix(),
		Issuer:    Issuer,
	}
}

func TestQAVerifyRejectsInvalidSubjectAndBranchClaims(t *testing.T) {
	secret := []byte(strings.Repeat("q", 32))
	now := time.Unix(1_700_000_000, 0)
	validHeader := tokenHeader{Algorithm: "HS256", Type: "JWT"}
	tests := []struct {
		name   string
		claims tokenClaims
	}{
		{name: "invalid subject", claims: func() tokenClaims { c := qaClaims("CEO", "", now); c.Subject = "not-a-uuid"; return c }()},
		{name: "branch manager malformed branch id", claims: qaClaims("BRANCH_MANAGER", "not-a-uuid", now)},
		{name: "branch manager missing branch id", claims: qaClaims("BRANCH_MANAGER", "", now)},
		{name: "non-manager has branch claim", claims: qaClaims("CEO", uuid.NewString(), now)},
		{name: "not-before in future", claims: func() tokenClaims { c := qaClaims("CEO", "", now); c.NotBefore = now.Add(time.Minute).Unix(); return c }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := qaSignedToken(t, secret, validHeader, tt.claims)
			if _, err := Verify(token, secret, now); err == nil {
				t.Fatal("Verify() accepted invalid claims")
			}
		})
	}
}

func TestQAVerifyRejectsUnsupportedHeaderAndShortSecret(t *testing.T) {
	secret := []byte(strings.Repeat("q", 32))
	now := time.Unix(1_700_000_000, 0)
	claims := qaClaims("CEO", "", now)
	unsignedAlg := qaSignedToken(t, secret, tokenHeader{Algorithm: "HS384", Type: "JWT"}, claims)
	if _, err := Verify(unsignedAlg, secret, now); err == nil {
		t.Fatal("Verify() accepted unsupported signing algorithm")
	}
	valid := qaSignedToken(t, secret, tokenHeader{Algorithm: "HS256", Type: "JWT"}, claims)
	if _, err := Verify(valid, []byte("short"), now); err == nil {
		t.Fatal("Verify() accepted a secret shorter than 32 bytes")
	}
}

func TestQAMiddlewareRejectsMalformedAuthorizationAndSetsVerifiedIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte(strings.Repeat("q", 32))
	now := time.Now()
	branchID := uuid.NewString()
	claims := qaClaims("BM", branchID, now)
	token := qaSignedToken(t, secret, tokenHeader{Algorithm: "HS256", Type: "JWT"}, claims)

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "missing authorization", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic " + token, wantStatus: http.StatusUnauthorized},
		{name: "empty bearer", header: "Bearer   ", wantStatus: http.StatusUnauthorized},
		{name: "valid bearer", header: "Bearer " + token, wantStatus: http.StatusNoContent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Middleware(secret))
			router.GET("/identity", func(c *gin.Context) {
				if c.GetString(UserIDKey) != claims.Subject || c.GetString(RoleKey) != "BRANCH_MANAGER" || c.GetString(BranchIDKey) != branchID {
					c.Status(http.StatusInternalServerError)
					return
				}
				value, exists := c.Get(ClaimsKey)
				verified, ok := value.(Claims)
				if !exists || !ok || verified.UserID.String() != claims.Subject || verified.Role != "BRANCH_MANAGER" {
					c.Status(http.StatusInternalServerError)
					return
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/identity", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, tt.wantStatus, res.Body.String())
			}
		})
	}
}
