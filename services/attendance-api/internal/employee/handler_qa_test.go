package employee

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

func employeeQAHandlerRouter(t *testing.T, repo *employeeQARepo, role, branchID string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(CtxUserID, "actor")
		c.Set(CtxUserRole, role)
		c.Set(CtxBranchID, branchID)
		c.Next()
	})
	NewHandler(NewService(nil, repo)).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func employeeQAPerform(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func employeeQAAssertError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
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
		t.Fatalf("error body is not JSON: %v", err)
	}
	if body.Error.Code != code || body.Error.Message == "" {
		t.Fatalf("error payload = %+v, want code %q and nonempty message", body.Error, code)
	}
}

func TestQAEmployeeHandlerValidationAndDomainResponses(t *testing.T) {
	tests := []struct {
		name   string
		role   string
		method string
		path   string
		body   string
		status int
		code   string
	}{
		{name: "malformed create JSON", role: "CEO", method: http.MethodPost, path: "/api/v1/employees", body: `{"name":`, status: http.StatusBadRequest, code: "invalid_payload"},
		{name: "invalid date format", role: "CEO", method: http.MethodPost, path: "/api/v1/employees", body: `{"name":"Amina","phone":"01234567890","national_id":"12345678901234","branch_id":"11111111-1111-4111-8111-111111111111","payment_type":"monthly","payment_amount":1000,"hire_date":"14-10-2026"}`, status: http.StatusBadRequest, code: "invalid_hire_date"},
		{name: "forbidden service role", role: "SUPPLY_MANAGER", method: http.MethodPost, path: "/api/v1/employees", body: `{"name":"Amina","phone":"01234567890","national_id":"12345678901234","branch_id":"11111111-1111-4111-8111-111111111111","payment_type":"monthly","payment_amount":1000,"hire_date":"2026-10-13"}`, status: http.StatusForbidden, code: "forbidden"},
		{name: "malformed update JSON", role: "CEO", method: http.MethodPatch, path: "/api/v1/employees/emp", body: `{"email":7}`, status: http.StatusBadRequest, code: "invalid_payload"},
		{name: "invalid termination date", role: "CEO", method: http.MethodPost, path: "/api/v1/employees/emp/terminate", body: `{"termination_date":"yesterday"}`, status: http.StatusBadRequest, code: "invalid_termination_date"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := employeeQAPerform(employeeQAHandlerRouter(t, &employeeQARepo{}, tc.role, "branch-a"), tc.method, tc.path, tc.body)
			employeeQAAssertError(t, res, tc.status, tc.code)
		})
	}
}

func TestQAEmployeeHandlerNotFoundAndInternalErrorSanitization(t *testing.T) {
	missing := employeeQAPerform(employeeQAHandlerRouter(t, &employeeQARepo{}, "CEO", ""), http.MethodGet, "/api/v1/employees/missing", "")
	employeeQAAssertError(t, missing, http.StatusNotFound, "not_found")

	const detail = "database connection refused"
	internal := employeeQAPerform(employeeQAHandlerRouter(t, &employeeQARepo{getErr: errors.New(detail)}, "CEO", ""), http.MethodGet, "/api/v1/employees/emp", "")
	employeeQAAssertError(t, internal, http.StatusInternalServerError, "internal_error")
	if strings.Contains(internal.Body.String(), detail) {
		t.Fatalf("internal error leaked in response: %s", internal.Body.String())
	}
}

func TestQAEmployeeHandlerManagerListIsBranchScoped(t *testing.T) {
	seed := &Employee{ID: "emp", Name: "Amina", BranchID: "branch-a", Active: true, HireDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
	repo := &employeeQARepo{listed: []*Employee{seed}}
	res := employeeQAPerform(employeeQAHandlerRouter(t, repo, "BRANCH_MANAGER", "branch-a"), http.MethodGet, "/api/v1/employees?active_only=false&branch_id=attacker", "")
	if res.Code != http.StatusOK || repo.lastBranchID != "branch-a" {
		t.Fatalf("list status=%d repository branch=%q body=%s", res.Code, repo.lastBranchID, res.Body.String())
	}
	var body ListEmployeesResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("list response decode: %v", err)
	}
	if body.Total != 1 || len(body.Employees) != 1 || body.Employees[0].BranchID != "branch-a" {
		t.Fatalf("list response = %+v, want seeded branch-a employee", body)
	}
}
