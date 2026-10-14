package attendance

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func attendanceQAHandlerRouter(t *testing.T, repo *attendanceQARepo, emp *attendanceQAEmployee, br *attendanceQABranch, role, branchID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "actor")
		c.Set(CtxUserRole, role)
		c.Set(CtxBranchID, branchID)
		c.Next()
	})
	svc := newAttendanceQAService(repo, emp, br, time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC))
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func attendanceQAPerform(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func attendanceQAAssertError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) ErrorResponse {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, status, res.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("error response decode: %v", err)
	}
	if body.Error.Code != code || body.Error.Message == "" {
		t.Fatalf("error = %+v, want code %q and nonempty message", body.Error, code)
	}
	return body
}

func TestQAAttendanceHandlerPayloadAndDomainErrorResponses(t *testing.T) {
	badPayload := attendanceQAPerform(attendanceQAHandlerRouter(t, &attendanceQARepo{}, &attendanceQAEmployee{}, &attendanceQABranch{}, "CEO", ""), http.MethodPost, "/api/v1/attendance/check-in", `{"employee_id":"not-a-uuid","source":"manual"}`)
	attendanceQAAssertError(t, badPayload, http.StatusBadRequest, "invalid_payload")

	missingEmployee := attendanceQAPerform(attendanceQAHandlerRouter(t, &attendanceQARepo{}, &attendanceQAEmployee{}, &attendanceQABranch{}, "CEO", ""), http.MethodPost, "/api/v1/attendance/check-in", `{"employee_id":"11111111-1111-4111-8111-111111111111","source":"manual"}`)
	attendanceQAAssertError(t, missingEmployee, http.StatusNotFound, "not_found")

	noAdjustment := attendanceQAPerform(attendanceQAHandlerRouter(t, &attendanceQARepo{}, &attendanceQAEmployee{}, &attendanceQABranch{}, "CEO", ""), http.MethodPatch, "/api/v1/attendance/deductions/deduction-1", `{"reason":"valid reason"}`)
	attendanceQAAssertError(t, noAdjustment, http.StatusBadRequest, "no_adjustment")
}

func TestQAAttendanceHandlerSanitizesInternalErrorsAndMapsAuthorization(t *testing.T) {
	const internalDetails = "employee database unavailable"
	failed := attendanceQAPerform(attendanceQAHandlerRouter(t, &attendanceQARepo{}, &attendanceQAEmployee{err: errors.New(internalDetails)}, &attendanceQABranch{}, "CEO", ""), http.MethodPost, "/api/v1/attendance/check-in", `{"employee_id":"11111111-1111-4111-8111-111111111111","source":"manual"}`)
	body := attendanceQAAssertError(t, failed, http.StatusInternalServerError, "internal_error")
	if strings.Contains(failed.Body.String(), internalDetails) || body.Error.Message != "something went wrong" {
		t.Fatalf("internal error not sanitized: %s", failed.Body.String())
	}

	denied := attendanceQAPerform(attendanceQAHandlerRouter(t, &attendanceQARepo{}, &attendanceQAEmployee{info: &EmployeeInfo{ID: "emp", BranchID: "branch-b", Active: true}}, &attendanceQABranch{}, "BRANCH_MANAGER", "branch-a"), http.MethodPost, "/api/v1/attendance/check-in", `{"employee_id":"11111111-1111-4111-8111-111111111111","source":"manual"}`)
	attendanceQAAssertError(t, denied, http.StatusForbidden, "forbidden")
}
