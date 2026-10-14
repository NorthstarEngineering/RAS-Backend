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

func attendanceQASignedToken(t *testing.T, secret []byte, header tokenHeader, claims tokenClaims) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func attendanceQAClaims(role, branchID string, now time.Time) tokenClaims {
	return tokenClaims{Subject: uuid.NewString(), Role: role, BranchID: branchID, ExpiresAt: now.Add(time.Minute).Unix(), Issuer: Issuer}
}

func TestAttendanceQAVerifyRejectsMalformedRequiredClaims(t *testing.T) {
	secret := []byte(strings.Repeat("v", 32))
	now := time.Unix(1_800_000_000, 0)
	header := tokenHeader{Algorithm: "HS256", Type: "JWT"}
	tests := []struct {
		name   string
		claims tokenClaims
	}{
		{name: "invalid subject", claims: func() tokenClaims { c := attendanceQAClaims("CEO", "", now); c.Subject = "invalid"; return c }()},
		{name: "invalid issuer", claims: func() tokenClaims { c := attendanceQAClaims("CEO", "", now); c.Issuer = "other"; return c }()},
		{name: "missing expiration", claims: func() tokenClaims { c := attendanceQAClaims("CEO", "", now); c.ExpiresAt = 0; return c }()},
		{name: "future not-before", claims: func() tokenClaims {
			c := attendanceQAClaims("CEO", "", now)
			c.NotBefore = now.Add(time.Minute).Unix()
			return c
		}()},
		{name: "manager missing branch", claims: attendanceQAClaims("BRANCH_MANAGER", "", now)},
		{name: "manager malformed branch", claims: attendanceQAClaims("BRANCH_MANAGER", "not-a-uuid", now)},
		{name: "CEO carries branch", claims: attendanceQAClaims("CEO", uuid.NewString(), now)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Verify(attendanceQASignedToken(t, secret, header, tc.claims), secret, now); err == nil {
				t.Fatal("Verify() accepted invalid required claims")
			}
		})
	}
}

func TestAttendanceQAVerifyRejectsUnsupportedAlgorithmAndShortSecret(t *testing.T) {
	secret := []byte(strings.Repeat("v", 32))
	now := time.Unix(1_800_000_000, 0)
	claims := attendanceQAClaims("CEO", "", now)
	badAlgorithm := attendanceQASignedToken(t, secret, tokenHeader{Algorithm: "none", Type: "JWT"}, claims)
	if _, err := Verify(badAlgorithm, secret, now); err == nil {
		t.Fatal("Verify() accepted unsupported algorithm")
	}
	valid := attendanceQASignedToken(t, secret, tokenHeader{Algorithm: "HS256", Type: "JWT"}, claims)
	if _, err := Verify(valid, []byte("short"), now); err == nil {
		t.Fatal("Verify() accepted a secret shorter than 32 bytes")
	}
}

func TestAttendanceQAMiddlewareHeaderValidationAndVerifiedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := []byte(strings.Repeat("v", 32))
	now := time.Now()
	branchID := uuid.NewString()
	claims := attendanceQAClaims("BM", branchID, now)
	token := attendanceQASignedToken(t, secret, tokenHeader{Algorithm: "HS256", Type: "JWT"}, claims)
	tests := []struct {
		name       string
		authorize  string
		wantStatus int
	}{
		{name: "missing header", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", authorize: "Basic " + token, wantStatus: http.StatusUnauthorized},
		{name: "empty bearer", authorize: "Bearer   ", wantStatus: http.StatusUnauthorized},
		{name: "valid token", authorize: "Bearer " + token, wantStatus: http.StatusNoContent},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			router.Use(Middleware(secret))
			router.GET("/identity", func(c *gin.Context) {
				if c.GetString(UserIDKey) != claims.Subject || c.GetString(RoleKey) != "BRANCH_MANAGER" || c.GetString(BranchIDKey) != branchID {
					c.Status(http.StatusInternalServerError)
					return
				}
				value, ok := c.Get(ClaimsKey)
				verified, valid := value.(Claims)
				if !ok || !valid || verified.UserID.String() != claims.Subject || verified.Role != "BRANCH_MANAGER" {
					c.Status(http.StatusInternalServerError)
					return
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/identity", nil)
			if tc.authorize != "" {
				req.Header.Set("Authorization", tc.authorize)
			}
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", res.Code, tc.wantStatus, res.Body.String())
			}
		})
	}
}

func TestAttendanceQABranchScopeAllowsCEOAndRejectsMissingManagerScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(RoleKey, c.GetHeader("X-Test-Role"))
		c.Set(BranchIDKey, c.GetHeader("X-Test-Branch"))
		c.Next()
	})
	router.GET("/scope/", func(c *gin.Context) {
		if !BranchScope(c, "") {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})
	router.GET("/scope/:branch", func(c *gin.Context) {
		if !BranchScope(c, c.Param("branch")) {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})
	for _, tc := range []struct {
		name, role, actorBranch, requested string
		want                               int
	}{
		{name: "CEO can access branch", role: "CEO", requested: "any", want: http.StatusNoContent},
		{name: "manager without branch denied", role: "BRANCH_MANAGER", requested: "branch-a", want: http.StatusForbidden},
		{name: "manager empty request denied", role: "BRANCH_MANAGER", actorBranch: "branch-a", want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/scope/"+tc.requested, nil)
			req.Header.Set("X-Test-Role", tc.role)
			req.Header.Set("X-Test-Branch", tc.actorBranch)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("BranchScope status = %d, want %d", res.Code, tc.want)
			}
		})
	}
}
