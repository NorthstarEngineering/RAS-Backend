package branch

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"restaurant-backend/internal/authn"
)

func qaHandlerRouter(t *testing.T, repo *qaRepository, role, branchID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(authn.RoleKey, role)
		c.Set(authn.BranchIDKey, branchID)
		c.Next()
	})
	handler := NewHandler(NewService(repo, qaClock{now: time.Unix(500, 0)}))
	handler.RegisterRoutes(router.Group("/api"), authn.RequireRoles)
	return router
}

func qaPerform(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func assertQAError(t *testing.T, res *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, status, res.Body.String())
	}
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %v; body=%s", err, res.Body.String())
	}
	if body.Error.Code != code || body.Error.Message != message {
		t.Fatalf("error = (%q, %q), want (%q, %q)", body.Error.Code, body.Error.Message, code, message)
	}
}

func TestQAHandlerRoleAuthorization(t *testing.T) {
	branchID := uuid.New()
	for _, tc := range []struct {
		name string
		role string
		want int
	}{
		{name: "CEO can create", role: "CEO", want: http.StatusCreated},
		{name: "branch manager cannot create", role: "BRANCH_MANAGER", want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &qaRepository{}
			router := qaHandlerRouter(t, repo, tc.role, branchID.String())
			body := `{"name":"North","type":"branch","address":"A","official_start_time":"09:00","late_rate":1}`
			res := qaPerform(router, http.MethodPost, "/api/branches", body)
			if res.Code != tc.want {
				t.Fatalf("POST status = %d, want %d; body=%s", res.Code, tc.want, res.Body.String())
			}
			if tc.role == "BRANCH_MANAGER" && repo.createCalls != 0 {
				t.Fatal("unauthorized role reached service repository")
			}
		})
	}
}

func TestQAHandlerBranchManagerReadsOnlyOwnBranch(t *testing.T) {
	ownID := uuid.New()
	seed := qaExistingBranch()
	seed.ID = ownID
	repo := &qaRepository{getBranch: seed, listed: []Branch{seed}}
	router := qaHandlerRouter(t, repo, "BRANCH_MANAGER", ownID.String())

	list := qaPerform(router, http.MethodGet, "/api/branches", "")
	if list.Code != http.StatusOK || repo.getCalls != 1 || repo.listCalls != 0 {
		t.Fatalf("manager list status=%d getCalls=%d listCalls=%d body=%s", list.Code, repo.getCalls, repo.listCalls, list.Body.String())
	}
	var rows []BranchResponse
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatalf("manager list response is not JSON: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != ownID.String() {
		t.Fatalf("manager list rows = %+v, want only branch %s", rows, ownID)
	}

	before := repo.getCalls
	otherID := uuid.New()
	get := qaPerform(router, http.MethodGet, "/api/branches/"+otherID.String(), "")
	assertQAError(t, get, http.StatusForbidden, "FORBIDDEN", "operation is not permitted")
	if repo.getCalls != before {
		t.Fatal("cross-branch request reached repository")
	}
}

func TestQAHandlerRejectsInvalidBranchIDAndJSON(t *testing.T) {
	router := qaHandlerRouter(t, &qaRepository{}, "CEO", "")
	badID := qaPerform(router, http.MethodGet, "/api/branches/not-a-uuid", "")
	assertQAError(t, badID, http.StatusBadRequest, "INVALID_ID", "branch id is not a valid UUID")

	badJSON := qaPerform(router, http.MethodPost, "/api/branches", `{"name":`)
	assertQAError(t, badJSON, http.StatusBadRequest, "INVALID_REQUEST", "request body is not valid JSON")

	badUpdateJSON := qaPerform(router, http.MethodPatch, "/api/branches/"+uuid.NewString(), `{"is_active":"not-a-bool"}`)
	assertQAError(t, badUpdateJSON, http.StatusBadRequest, "INVALID_REQUEST", "request body is not valid JSON")
}

func TestQAHandlerMapsValidationAndNotFoundErrors(t *testing.T) {
	router := qaHandlerRouter(t, &qaRepository{getErr: ErrNotFound}, "CEO", "")
	get := qaPerform(router, http.MethodGet, "/api/branches/"+uuid.NewString(), "")
	assertQAError(t, get, http.StatusNotFound, "BRANCH_NOT_FOUND", ErrNotFound.Error())

	invalidCreate := `{"name":" ","type":"branch","official_start_time":"09:00","late_rate":1}`
	create := qaPerform(qaHandlerRouter(t, &qaRepository{}, "CEO", ""), http.MethodPost, "/api/branches", invalidCreate)
	assertQAError(t, create, http.StatusBadRequest, "BRANCH_NAME_REQUIRED", ErrNameRequired.Error())
}

func TestQAHandlerSanitizesUnexpectedRepositoryErrors(t *testing.T) {
	const detail = "database unavailable"
	repo := &qaRepository{listErr: errors.New(detail)}
	res := qaPerform(qaHandlerRouter(t, repo, "CEO", ""), http.MethodGet, "/api/branches", "")
	assertQAError(t, res, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
	if strings.Contains(res.Body.String(), detail) {
		t.Fatalf("internal error leaked to response: %s", res.Body.String())
	}
}
