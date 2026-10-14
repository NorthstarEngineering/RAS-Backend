package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	autherrors "restaurant-backend/internal/auth/errors"
)

type qaHandlerClock struct{ now time.Time }

func (c qaHandlerClock) Now() time.Time { return c.now }

func TestQARespondErrorMapsDomainAndInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		err      error
		want     int
		wantCode string
		wantMsg  string
		mustHide string
	}{
		{name: "invalid credentials", err: autherrors.ErrInvalidCredentials, want: http.StatusUnauthorized, wantCode: "INVALID_CREDENTIALS", wantMsg: autherrors.ErrInvalidCredentials.Message},
		{name: "forbidden", err: autherrors.ErrForbidden, want: http.StatusForbidden, wantCode: "FORBIDDEN", wantMsg: autherrors.ErrForbidden.Message},
		{name: "email conflict", err: autherrors.ErrEmailTaken, want: http.StatusConflict, wantCode: "EMAIL_ALREADY_EXISTS", wantMsg: autherrors.ErrEmailTaken.Message},
		{name: "validation", err: autherrors.ErrInvalidEmail, want: http.StatusBadRequest, wantCode: "INVALID_EMAIL", wantMsg: autherrors.ErrInvalidEmail.Message},
		{name: "unknown internal", err: errors.New("database unavailable"), want: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMsg: "Something went wrong", mustHide: "database unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", func(c *gin.Context) { respondError(c, tt.err) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tt.want, w.Body.String())
			}
			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("response body is not valid error JSON: %v; body=%s", err, w.Body.String())
			}
			if body.Error.Code != tt.wantCode || body.Error.Message != tt.wantMsg {
				t.Fatalf("error body = %+v, want code %q and message %q", body.Error, tt.wantCode, tt.wantMsg)
			}
			if tt.mustHide != "" && strings.Contains(w.Body.String(), tt.mustHide) {
				t.Fatalf("internal detail %q was exposed in response: %s", tt.mustHide, w.Body.String())
			}
		})
	}
}

func TestQARespondErrorLockedSetsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", func(c *gin.Context) {
		respondError(c, &autherrors.LockedError{RetryAfter: 61 * time.Second})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
	if got := w.Header().Get("Retry-After"); got != "61" {
		t.Fatalf("Retry-After = %q, want 61", got)
	}
}

func TestQARateLimiterFixedWindowAndPerKeyIsolation(t *testing.T) {
	clock := qaHandlerClock{now: time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)}
	limiter := NewRateLimiter(2, time.Minute, clock)
	if ok, _ := limiter.Allow("client-a"); !ok {
		t.Fatal("first request was rejected")
	}
	if ok, _ := limiter.Allow("client-a"); !ok {
		t.Fatal("second request was rejected")
	}
	if ok, retry := limiter.Allow("client-a"); ok || retry != time.Minute {
		t.Fatalf("third request = (%v, %v), want (false, 1m)", ok, retry)
	}
	if ok, _ := limiter.Allow("client-b"); !ok {
		t.Fatal("different client key was rejected")
	}

	clock.now = clock.now.Add(time.Minute)
	limiter.clock = clock
	if ok, _ := limiter.Allow("client-a"); !ok {
		t.Fatal("request at window boundary was rejected")
	}
}
