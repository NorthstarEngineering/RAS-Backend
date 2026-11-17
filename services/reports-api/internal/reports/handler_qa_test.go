package reports

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type reportCaptureService struct {
	audit                                   AuditFilter
	sales, inventory, attendance, approvals ReportFilter
	logs                                    []*AuditLog
	err                                     error
}

func (s *reportCaptureService) GetAuditLogs(_ context.Context, f AuditFilter) ([]*AuditLog, error) {
	s.audit = f
	return s.logs, s.err
}
func (s *reportCaptureService) GetSalesReport(_ context.Context, f ReportFilter) (*SalesReportResponse, error) {
	s.sales = f
	return &SalesReportResponse{TotalRevenue: 10}, s.err
}
func (s *reportCaptureService) GetInventoryReport(_ context.Context, f ReportFilter) (*InventoryReportResponse, error) {
	s.inventory = f
	return &InventoryReportResponse{TotalItems: 1}, s.err
}
func (s *reportCaptureService) GetAttendanceReport(_ context.Context, f ReportFilter) (*AttendanceReportResponse, error) {
	s.attendance = f
	return &AttendanceReportResponse{TotalEmployees: 1}, s.err
}
func (s *reportCaptureService) GetApprovalQueue(_ context.Context, f ReportFilter) (*ApprovalQueueResponse, error) {
	s.approvals = f
	return &ApprovalQueueResponse{PendingMenuRequests: 1}, s.err
}

func requestReportAs(s reportService, role, branch, path string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("user_role", role)
		}
		if branch != "" {
			c.Set("branch_id", branch)
		}
		c.Next()
	})
	NewHandler(s).RegisterRoutes(r.Group("/api/v1"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestAuditHandlerAppliesFiltersPaginationAndMapsAuditIdentity(t *testing.T) {
	actor := uuid.New()
	entity := uuid.New()
	id := uuid.New()
	reason := "approved"
	s := &reportCaptureService{logs: []*AuditLog{{ID: id, ActorID: actor, ActorRole: "CEO", Action: "APPROVE", EntityType: "MENU_ITEM", EntityID: entity, Reason: &reason}}}
	w := requestReportAs(s, "CEO", "", "/api/v1/reports/audit?action=APPROVE&entity_type=MENU_ITEM&limit=999&offset=-4")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if s.audit.Action != "APPROVE" || s.audit.EntityType != "MENU_ITEM" || s.audit.Limit != 500 || s.audit.Offset != 0 {
		t.Fatalf("audit filter=%+v", s.audit)
	}
	for _, part := range []string{id.String(), actor.String(), entity.String(), "APPROVE", "created_at"} {
		if !strings.Contains(w.Body.String(), part) {
			t.Fatalf("response %s missing %q", w.Body.String(), part)
		}
	}
}

func TestAuditHandlerInvalidPaginationFallsBackToDefaults(t *testing.T) {
	s := &reportCaptureService{}
	w := requestReportAs(s, "CEO", "", "/api/v1/reports/audit?limit=bad&offset=bad")
	if w.Code != http.StatusOK || s.audit.Limit != 50 || s.audit.Offset != 0 {
		t.Fatalf("status=%d filter=%+v body=%s", w.Code, s.audit, w.Body.String())
	}
}

func TestSalesHandlerPassesCEOAndBranchManagerScopes(t *testing.T) {
	ceo := &reportCaptureService{}
	w := requestReportAs(ceo, "CEO", "", "/api/v1/reports/sales?branch_id=branch-2&from=2026-01-01&to=2026-01-31")
	if w.Code != http.StatusOK || ceo.sales != (ReportFilter{BranchID: "branch-2", From: "2026-01-01", To: "2026-01-31"}) {
		t.Fatalf("CEO status=%d filter=%+v", w.Code, ceo.sales)
	}
	bm := &reportCaptureService{}
	w = requestReportAs(bm, "BRANCH_MANAGER", "assigned-branch", "/api/v1/reports/sales?branch_id=other-branch&from=2026-02-01")
	if w.Code != http.StatusOK || bm.sales != (ReportFilter{BranchID: "assigned-branch", From: "2026-02-01"}) {
		t.Fatalf("BM status=%d filter=%+v", w.Code, bm.sales)
	}
}

func TestAttendanceHandlerPassesBranchAndDateRange(t *testing.T) {
	s := &reportCaptureService{}
	w := requestReportAs(s, "BRANCH_MANAGER", "assigned", "/api/v1/reports/attendance?from=2026-03-01&to=2026-03-07")
	if w.Code != http.StatusOK || s.attendance != (ReportFilter{BranchID: "assigned", From: "2026-03-01", To: "2026-03-07"}) {
		t.Fatalf("status=%d filter=%+v", w.Code, s.attendance)
	}
}

func TestSupplyManagerApprovalScopeAndBranchManagerAssignments(t *testing.T) {
	s := &reportCaptureService{}
	w := requestReportAs(s, "SUPPLY_MANAGER", "", "/api/v1/reports/approvals?branch_id=central-kitchen")
	if w.Code != http.StatusOK || s.approvals.BranchID != "central-kitchen" {
		t.Fatalf("supply manager status=%d filter=%+v", w.Code, s.approvals)
	}
	for _, path := range []string{"/api/v1/reports/sales", "/api/v1/reports/inventory", "/api/v1/reports/attendance"} {
		w = requestReportAs(&reportCaptureService{}, "BRANCH_MANAGER", "", path)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s status=%d; want 403", path, w.Code)
		}
	}
}

func TestReportsRejectMissingAndUnauthorizedRoles(t *testing.T) {
	cases := []struct {
		role, path string
		want       int
	}{
		{"", "/api/v1/reports/audit", http.StatusUnauthorized},
		{"BRANCH_MANAGER", "/api/v1/reports/audit", http.StatusForbidden},
		{"SUPPLY_MANAGER", "/api/v1/reports/sales", http.StatusForbidden},
		{"CASHIER", "/api/v1/reports/inventory", http.StatusForbidden},
		{"SUPPLY_MANAGER", "/api/v1/reports/attendance", http.StatusForbidden},
		{"BRANCH_MANAGER", "/api/v1/reports/approvals", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.role+tc.path, func(t *testing.T) {
			w := requestReportAs(&reportCaptureService{}, tc.role, "assigned", tc.path)
			if w.Code != tc.want {
				t.Fatalf("status=%d; want %d", w.Code, tc.want)
			}
		})
	}
}

func TestEveryReportHandlerMapsServiceErrorsTo500(t *testing.T) {
	cases := []struct{ path string }{
		{"/api/v1/reports/audit"}, {"/api/v1/reports/sales"}, {"/api/v1/reports/inventory"}, {"/api/v1/reports/attendance"}, {"/api/v1/reports/approvals"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := requestReportAs(&reportCaptureService{err: errors.New("database unavailable")}, "CEO", "", tc.path)
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestInventoryHandlerPassesSupplyManagerRequestedBranch(t *testing.T) {
	s := &reportCaptureService{}
	w := requestReportAs(s, "SUPPLY_MANAGER", "", "/api/v1/reports/inventory?branch_id=branch-7")
	if w.Code != http.StatusOK || s.inventory.BranchID != "branch-7" {
		t.Fatalf("status=%d filter=%+v", w.Code, s.inventory)
	}
}
