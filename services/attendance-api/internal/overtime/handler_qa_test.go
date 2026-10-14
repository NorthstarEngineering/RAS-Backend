package overtime

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"restaurant-api/internal/authn"
)

func overtimeQAHandlerRouter(t *testing.T, repo *overtimeQARepo, role, branchID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "actor")
		c.Set(CtxUserRole, role)
		c.Set(CtxBranchID, branchID)
		c.Next()
	})
	api := router.Group("/api/v1")
	api.Use(authn.RequireRoles("CEO", "BRANCH_MANAGER"))
	NewHandler(overtimeQAService(repo, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))).RegisterRoutes(api)
	return router
}

func overtimeQAPerform(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func overtimeQAAssertError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, status, res.Body.String())
	}
	var payload ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("error payload decode: %v", err)
	}
	if payload.Error.Code != code || payload.Error.Message == "" {
		t.Fatalf("error = %+v, want code %q and nonempty message", payload.Error, code)
	}
}

func TestQAOvertimeHandlerDateValidationAndForbiddenRole(t *testing.T) {
	badDate := overtimeQAPerform(overtimeQAHandlerRouter(t, &overtimeQARepo{}, "CEO", ""), http.MethodPost, "/api/v1/overtime", `{"employee_id":"11111111-1111-4111-8111-111111111111","work_date":"tomorrow","hours":1,"rate_per_hour":5,"source":"manual"}`)
	overtimeQAAssertError(t, badDate, http.StatusBadRequest, "invalid_work_date")

	forbidden := overtimeQAPerform(overtimeQAHandlerRouter(t, &overtimeQARepo{}, "SUPPLY_MANAGER", ""), http.MethodGet, "/api/v1/overtime/ot", "")
	overtimeQAAssertError(t, forbidden, http.StatusForbidden, "FORBIDDEN")
}

func TestQAOvertimeHandlerBranchScopeAndListFilter(t *testing.T) {
	record := &Overtime{ID: "ot", EmployeeID: "emp", BranchID: "branch-a", Status: StatusPending, WorkDate: time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)}
	repo := &overtimeQARepo{record: record, list: []*Overtime{record}}
	router := overtimeQAHandlerRouter(t, repo, "BRANCH_MANAGER", "branch-a")
	get := overtimeQAPerform(router, http.MethodGet, "/api/v1/overtime/ot", "")
	if get.Code != http.StatusOK {
		t.Fatalf("same-branch get status=%d body=%s", get.Code, get.Body.String())
	}
	var response OvertimeResponse
	if err := json.Unmarshal(get.Body.Bytes(), &response); err != nil || response.ID != "ot" {
		t.Fatalf("get response=%+v err=%v", response, err)
	}
	repo.record = &Overtime{ID: "other", BranchID: "branch-b"}
	cross := overtimeQAPerform(router, http.MethodGet, "/api/v1/overtime/other", "")
	overtimeQAAssertError(t, cross, http.StatusForbidden, "forbidden")

	list := overtimeQAPerform(router, http.MethodGet, "/api/v1/overtime?employee_id=emp&status=pending&from=2026-10-01&to=2026-10-31", "")
	if list.Code != http.StatusOK || repo.filter.BranchID != "branch-a" || repo.filter.EmployeeID != "emp" || repo.filter.Status != "pending" {
		t.Fatalf("list status=%d filter=%+v body=%s", list.Code, repo.filter, list.Body.String())
	}
}

func TestQAOvertimeHandlerRejectReasonAndSanitizesInternalErrors(t *testing.T) {
	repo := &overtimeQARepo{record: &Overtime{ID: "ot", BranchID: "branch-a", Status: StatusPending}}
	router := overtimeQAHandlerRouter(t, repo, "CEO", "")
	blankReason := overtimeQAPerform(router, http.MethodPatch, "/api/v1/overtime/ot/reject", `{"reason":"   "}`)
	overtimeQAAssertError(t, blankReason, http.StatusBadRequest, "validation_error")

	const internalDetails = "database unavailable"
	failed := overtimeQAPerform(overtimeQAHandlerRouter(t, &overtimeQARepo{getErr: errors.New(internalDetails)}, "CEO", ""), http.MethodGet, "/api/v1/overtime/ot", "")
	overtimeQAAssertError(t, failed, http.StatusInternalServerError, "internal_error")
	if strings.Contains(failed.Body.String(), internalDetails) {
		t.Fatalf("internal error leaked: %s", failed.Body.String())
	}
}

func TestQAOvertimeHandlerSanitizesEmployeeLookupFailure(t *testing.T) {
	const internalDetails = "employee database unavailable"
	handler := NewHandler(overtimeQAService(&overtimeQARepo{}, time.Now()))
	handler.SetEmployeeBranchLookup(func(context.Context, string) (string, error) { return "", errors.New(internalDetails) })
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(CtxUserID, "actor"); c.Set(CtxUserRole, "CEO"); c.Next() })
	handler.RegisterRoutes(router.Group("/api/v1"))
	res := overtimeQAPerform(router, http.MethodPost, "/api/v1/overtime", `{"employee_id":"11111111-1111-4111-8111-111111111111","work_date":"2026-10-13","hours":1,"rate_per_hour":5,"source":"manual"}`)
	overtimeQAAssertError(t, res, http.StatusInternalServerError, "internal_error")
	if strings.Contains(res.Body.String(), internalDetails) {
		t.Fatalf("employee lookup detail leaked: %s", res.Body.String())
	}
}
