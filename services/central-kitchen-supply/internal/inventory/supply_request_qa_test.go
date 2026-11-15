package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQASupplyRequestHandlersRejectInvalidPayloadsWithoutDatabase(t *testing.T) {
	validUUID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create rejects invalid branch ID",
			method: http.MethodPost,
			path:   "/api/v1/supply/requests",
			body:   `{"branch_id":"not-a-uuid","items":[{}]}`,
		},
		{
			name:   "create rejects empty items",
			method: http.MethodPost,
			path:   "/api/v1/supply/requests",
			body:   `{"branch_id":"` + validUUID + `","items":[]}`,
		},
		{
			name:   "approve rejects invalid item ID",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/approve",
			body:   `{"items":[{"item_id":"not-a-uuid","approved_qty":1}]}`,
		},
		{
			name:   "approve rejects negative approved quantity",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/approve",
			body:   `{"items":[{"item_id":"` + validUUID + `","approved_qty":-1}]}`,
		},
		{
			name:   "reject requires reason",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/reject",
			body:   `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(nil, &supplyRequestQARepositoryStub{})
			res := supplyRequestQARequest(supplyRequestQARouter(svc, ""), tt.method, tt.path, tt.body)
			supplyRequestQAAssertError(t, res, http.StatusBadRequest, "invalid_payload")
		})
	}
}

func TestQASupplyRequestHandlerRejectsInvalidReceivedQuantity(t *testing.T) {
	repo := &supplyRequestQARepositoryStub{request: &SupplyRequest{ID: "request-1", Status: "PENDING"}}
	svc := NewService(nil, repo)
	res := supplyRequestQARequest(
		supplyRequestQARouter(svc, "BRANCH_MANAGER"),
		http.MethodPatch,
		"/api/v1/supply/requests/request-1/receive",
		`{"items":[{"item_id":"11111111-1111-4111-8111-111111111111","received_qty":-1}]}`,
	)
	supplyRequestQAAssertError(t, res, http.StatusBadRequest, "invalid_payload")
}

func TestQASupplyRequestHandlersRejectUnauthorizedRoles(t *testing.T) {
	validUUID := "11111111-1111-4111-8111-111111111111"
	tests := []struct {
		name   string
		role   string
		method string
		path   string
		body   string
	}{
		{
			name:   "branch manager cannot approve",
			role:   "BRANCH_MANAGER",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/approve",
			body:   `{"items":[{"item_id":"` + validUUID + `","approved_qty":1}]}`,
		},
		{
			name:   "branch manager cannot reject",
			role:   "BRANCH_MANAGER",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/reject",
			body:   `{"reason":"duplicate"}`,
		},
		{
			name:   "branch manager cannot ship",
			role:   "BRANCH_MANAGER",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/ship",
		},
		{
			name:   "supply manager cannot receive",
			role:   "SUPPLY_MANAGER",
			method: http.MethodPatch,
			path:   "/api/v1/supply/requests/request-1/receive",
			body:   `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := supplyRequestQARequest(supplyRequestQARouter(&Service{}, tt.role), tt.method, tt.path, tt.body)
			supplyRequestQAAssertError(t, res, http.StatusForbidden, "forbidden")
		})
	}
}

func TestQASupplyRequestWorkflowRejectsInvalidStatesBeforeDatabase(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantError string
		call      func(*Service) error
	}{
		{
			name:      "approve requires pending status",
			status:    "SHIPPED",
			wantError: "request is not pending",
			call: func(svc *Service) error {
				_, err := svc.ApproveSupplyRequest(context.Background(), ApproveSupplyRequestInput{RequestID: "request-1", ActorRole: "SUPPLY_MANAGER"})
				return err
			},
		},
		{
			name:      "reject requires pending status",
			status:    "APPROVED",
			wantError: "request is not pending",
			call: func(svc *Service) error {
				_, err := svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{RequestID: "request-1", ActorRole: "SUPPLY_MANAGER", Reason: "duplicate"})
				return err
			},
		},
		{
			name:      "ship requires approved status",
			status:    "PENDING",
			wantError: "request must be approved first",
			call: func(svc *Service) error {
				return svc.ShipSupplyRequest(context.Background(), "SUPPLY_MANAGER", "request-1")
			},
		},
		{
			name:      "receive requires shipped status",
			status:    "APPROVED",
			wantError: "request must be shipped first",
			call: func(svc *Service) error {
				return svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{RequestID: "request-1", ActorRole: "BRANCH_MANAGER"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &supplyRequestQARepositoryStub{request: &SupplyRequest{ID: "request-1", Status: tt.status}}
			err := tt.call(NewService(nil, repo))
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("workflow error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestQASupplyRequestRejectRequiresReasonBeforeRepositoryAccess(t *testing.T) {
	repo := &supplyRequestQARepositoryStub{}
	svc := NewService(nil, repo)
	_, err := svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{
		RequestID: "request-1",
		ActorRole: "SUPPLY_MANAGER",
	})
	if err == nil || err.Error() != "rejection reason is required" {
		t.Fatalf("RejectSupplyRequest() error = %v, want rejection reason required", err)
	}
	if repo.getCalls != 0 {
		t.Fatalf("repository GetSupplyRequestByID calls = %d, want 0", repo.getCalls)
	}
}

func TestQASupplyRequestResponseMapsApprovalAndReceiptQuantities(t *testing.T) {
	approvedQty := 7.0
	receivedQty := 5.0
	request := &SupplyRequest{
		ID: "request-1", BranchID: "branch-1", Status: "FULFILLED", CreatedBy: "manager-1",
		Items: []*SupplyRequestItem{{
			ID: "item-1", GoodID: "good-1", RequestedQty: 8,
			ApprovedQty: &approvedQty, ReceivedQty: &receivedQty,
		}},
	}

	response := request.ToResponse()
	if response.Status != "FULFILLED" || len(response.Items) != 1 {
		t.Fatalf("ToResponse() status/items = %q/%d, want FULFILLED/1", response.Status, len(response.Items))
	}
	item := response.Items[0]
	if item.RequestedQty != 8 || item.ApprovedQty == nil || *item.ApprovedQty != approvedQty || item.ReceivedQty == nil || *item.ReceivedQty != receivedQty {
		t.Fatalf("ToResponse() item quantities = %+v, want requested 8, approved %v, received %v", item, approvedQty, receivedQty)
	}
}

type supplyRequestQARepositoryStub struct {
	Repository
	request  *SupplyRequest
	getCalls int
}

func (r *supplyRequestQARepositoryStub) GetSupplyRequestByID(_ context.Context, _ string) (*SupplyRequest, error) {
	r.getCalls++
	return r.request, nil
}

func supplyRequestQARouter(svc *Service, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if role != "" {
		router.Use(func(c *gin.Context) {
			c.Set(CtxUserRole, role)
			c.Next()
		})
	}
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func supplyRequestQARequest(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func supplyRequestQAAssertError(t *testing.T, res *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, status, res.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid ErrorResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q; body=%s", body.Error.Code, code, res.Body.String())
	}
}
