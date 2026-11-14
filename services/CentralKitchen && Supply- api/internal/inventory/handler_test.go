package inventory

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func handlerRouter(h *testHarness) *gin.Engine {
	return handlerRouterWithRole(h, "")
}

func handlerRouterWithRole(h *testHarness, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if role != "" {
		router.Use(func(c *gin.Context) {
			c.Set(CtxUserRole, role)
			c.Next()
		})
	}
	NewHandler(h.svc).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func performJSON(t *testing.T, router http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	var response ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, recorder.Body.String())
	}
	return response.Error
}

func TestRecordMovementHandlerMapsValidationAndStockErrors(t *testing.T) {
	goodID := "00000000-0000-0000-0000-000000000001"
	for _, tc := range []struct {
		name, typ, reason string
		stock, quantity   float64
		wantStatus        int
		wantCode          string
	}{
		{name: "reason required", typ: "OUT", quantity: 1, wantStatus: http.StatusBadRequest, wantCode: "validation_error"},
		{name: "insufficient stock", typ: "OUT", reason: "issue", stock: 1, quantity: 2, wantStatus: http.StatusConflict, wantCode: "insufficient_stock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if tc.stock > 0 {
				putInventory(h.repo, "branch", goodID, tc.stock)
			}
			response := performJSON(t, handlerRouter(h), http.MethodPost, "/api/v1/inventory/movements", map[string]any{
				"branch_id": "branch", "actor_id": "actor", "good_id": goodID,
				"type": tc.typ, "quantity": tc.quantity, "reason": tc.reason,
			})
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, tc.wantStatus, response.Body.String())
			}
			if got := decodeError(t, response).Code; got != tc.wantCode {
				t.Fatalf("error code = %q, want %q", got, tc.wantCode)
			}
		})
	}
}

func TestApproveRequestHandlerMapsForbiddenRole(t *testing.T) {
	h := newTestHarness(t)
	response := performJSON(t, handlerRouter(h), http.MethodPatch, "/api/v1/supply/requests/request-id/approve", map[string]any{
		"actor_role": "BRANCH_MANAGER",
		"items":      []map[string]any{{"item_id": "00000000-0000-0000-0000-000000000001", "approved_qty": 1}},
	})
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if got := decodeError(t, response).Code; got != "forbidden" {
		t.Fatalf("error code = %q, want forbidden", got)
	}
}

func TestGetGoodHandlerReturnsContractNotFoundStatus(t *testing.T) {
	h := newTestHarness(t)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/inventory/goods/missing", nil)
	handlerRouter(h).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d per GET /inventory/goods/{id} contract; body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}
	if got := decodeError(t, response).Code; got != "not_found" {
		t.Fatalf("error code = %q, want not_found", got)
	}
}

func TestCreateGoodHandlerSuccessAndValidation(t *testing.T) {
	h := newTestHarness(t)
	payload := map[string]any{
		"sku": "flour", "name": "Flour", "unit": "kg", "unit_cost": 1.25,
		"category": "dry", "description": "white flour", "reorder_level": 2, "max_capacity": 10,
	}
	response := performJSON(t, handlerRouter(h), http.MethodPost, "/api/v1/inventory/goods", payload)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	var good Good
	if err := json.Unmarshal(response.Body.Bytes(), &good); err != nil {
		t.Fatal(err)
	}
	if good.SKU != "flour" || good.Name != "Flour" || good.UnitCost != 1.25 || good.Category == nil || *good.Category != "dry" {
		t.Fatalf("unexpected good response: %#v", good)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{"missing name", map[string]any{"sku": "x", "unit": "kg", "unit_cost": 1}},
		{"negative unit cost", map[string]any{"sku": "x", "name": "X", "unit": "kg", "unit_cost": -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := performJSON(t, handlerRouter(newTestHarness(t)), http.MethodPost, "/api/v1/inventory/goods", tc.body)
			if bad.Code != http.StatusBadRequest || decodeError(t, bad).Code != "invalid_payload" {
				t.Fatalf("response=(%d,%s), want 400 invalid_payload", bad.Code, bad.Body.String())
			}
		})
	}
}

func TestListGoodsHandlerSuccessAndRepositoryError(t *testing.T) {
	h := newTestHarness(t)
	h.repo.goods["g1"] = &Good{ID: "g1", SKU: "rice", Name: "Rice", Unit: "kg", UnitCost: 2}
	response := httptest.NewRecorder()
	handlerRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/inventory/goods", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", response.Code, response.Body.String())
	}
	var goods []Good
	if err := json.Unmarshal(response.Body.Bytes(), &goods); err != nil {
		t.Fatal(err)
	}
	if len(goods) != 1 || goods[0].ID != "g1" || goods[0].SKU != "rice" || goods[0].UnitCost != 2 {
		t.Fatalf("unexpected goods response: %#v", goods)
	}

	h.repo.errs["listGoods"] = errors.New("list unavailable")
	response = httptest.NewRecorder()
	handlerRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/inventory/goods", nil))
	if response.Code != http.StatusInternalServerError || decodeError(t, response).Code != "internal_error" {
		t.Fatalf("repository error response=(%d,%s)", response.Code, response.Body.String())
	}
}

func TestCreateSupplyRequestHandlerSuccessAndValidation(t *testing.T) {
	h := newTestHarness(t)
	payload := map[string]any{
		"branch_id": "00000000-0000-0000-0000-000000000001",
		"notes":     "weekly", "priority": "URGENT",
		"items": []map[string]any{
			{"good_id": "00000000-0000-0000-0000-000000000002", "requested_qty": 2},
			{"good_id": "00000000-0000-0000-0000-000000000003", "requested_qty": 4},
		},
	}
	response := performJSON(t, handlerRouter(h), http.MethodPost, "/api/v1/supply/requests", payload)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d want 201 body=%s", response.Code, response.Body.String())
	}
	var created SupplyRequestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Status != "PENDING" || created.BranchID != payload["branch_id"] || created.Priority != "URGENT" ||
		created.CreatedBy != "99999999-9999-9999-9999-999999999999" || len(created.Items) != 2 || created.Items[1].RequestedQty != 4 {
		t.Fatalf("unexpected supply request response: %#v", created)
	}

	for _, body := range []map[string]any{
		{"branch_id": "bad-id", "items": []map[string]any{{"good_id": "g", "requested_qty": 1}}},
		{"branch_id": "00000000-0000-0000-0000-000000000001", "items": []map[string]any{}},
	} {
		bad := performJSON(t, handlerRouter(newTestHarness(t)), http.MethodPost, "/api/v1/supply/requests", body)
		if bad.Code != http.StatusBadRequest || decodeError(t, bad).Code != "invalid_payload" {
			t.Fatalf("invalid request response=(%d,%s)", bad.Code, bad.Body.String())
		}
	}
}

func TestListSupplyRequestsHandlerMapsFieldsAndRepositoryError(t *testing.T) {
	h := newTestHarness(t)
	notes := "urgent restock"
	h.repo.reqs["r"] = &SupplyRequest{
		ID: "r", BranchID: "b", Status: "PENDING", CreatedBy: "manager", Priority: "URGENT",
		Notes: &notes, CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
	h.repo.items["r"] = []*SupplyRequestItem{{ID: "i", RequestID: "r", GoodID: "g", RequestedQty: 3}}
	response := httptest.NewRecorder()
	handlerRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/supply/requests", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", response.Code, response.Body.String())
	}
	var requests []SupplyRequestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &requests); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 || requests[0].ID != "r" || requests[0].BranchID != "b" ||
		requests[0].Status != "PENDING" || requests[0].CreatedBy != "manager" ||
		requests[0].Priority != "URGENT" || requests[0].Notes == nil || *requests[0].Notes != notes {
		t.Fatalf("unexpected list response: %#v", requests)
	}
	// The production ListSupplyRequests query does not load items; handler mapping preserves that behavior.
	if len(requests[0].Items) != 0 {
		t.Fatalf("list unexpectedly included request items: %#v", requests[0].Items)
	}

	h.repo.errs["listRequests"] = errors.New("list unavailable")
	response = httptest.NewRecorder()
	handlerRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/supply/requests", nil))
	if response.Code != http.StatusInternalServerError || decodeError(t, response).Code != "internal_error" {
		t.Fatalf("repository error response=(%d,%s)", response.Code, response.Body.String())
	}
}

func TestApproveSupplyRequestHandlerSuccessfulResponse(t *testing.T) {
	h := newTestHarness(t)
	requestID := "00000000-0000-0000-0000-000000000010"
	itemID := "00000000-0000-0000-0000-000000000011"
	putRequest(h.repo, requestID, "branch", "PENDING", &SupplyRequestItem{ID: itemID, GoodID: "good", RequestedQty: 5})
	response := performJSON(t, handlerRouter(h), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/approve", map[string]any{
		"actor_role": "CEO",
		"items":      []map[string]any{{"item_id": itemID, "approved_qty": 3}},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", response.Code, response.Body.String())
	}
	var approved SupplyRequestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &approved); err != nil {
		t.Fatal(err)
	}
	if approved.Status != "APPROVED" || len(approved.Items) != 1 || approved.Items[0].ApprovedQty == nil || *approved.Items[0].ApprovedQty != 3 {
		t.Fatalf("unexpected approval response: %#v", approved)
	}
}

func TestRejectSupplyRequestHandlerSuccessValidationAndAuthorization(t *testing.T) {
	requestID := "00000000-0000-0000-0000-000000000020"
	h := newTestHarness(t)
	putRequest(h.repo, requestID, "branch", "PENDING")
	response := performJSON(t, handlerRouter(h), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/reject", map[string]any{
		"actor_role": "CEO", "reason": "out of range",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", response.Code, response.Body.String())
	}
	var rejected SupplyRequestResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "REJECTED" || rejected.Notes == nil || *rejected.Notes != "out of range" {
		t.Fatalf("unexpected rejection response: %#v", rejected)
	}

	for _, tc := range []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing reason", map[string]any{"actor_role": "CEO"}, http.StatusBadRequest},
		{"forbidden role", map[string]any{"actor_role": "BRANCH_MANAGER", "reason": "reason"}, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := performJSON(t, handlerRouter(newTestHarness(t)), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/reject", tc.body)
			if bad.Code != tc.want {
				t.Fatalf("status=%d want %d body=%s", bad.Code, tc.want, bad.Body.String())
			}
		})
	}
}

func TestShipSupplyRequestHandlerSuccessAuthorizationAndStockError(t *testing.T) {
	requestID := "00000000-0000-0000-0000-000000000030"
	central := "99999999-9999-9999-9999-999999999999"
	h := newTestHarness(t)
	putInventory(h.repo, central, "g", 4)
	putRequest(h.repo, requestID, "branch", "APPROVED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(2)})
	response := httptest.NewRecorder()
	handlerRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/ship", nil))
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"shipped\"}" {
		t.Fatalf("ship response=(%d,%s)", response.Code, response.Body.String())
	}

	forbidden := httptest.NewRecorder()
	handlerRouterWithRole(newTestHarness(t), "BRANCH_MANAGER").ServeHTTP(forbidden,
		httptest.NewRequest(http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/ship", nil))
	if forbidden.Code != http.StatusForbidden || decodeError(t, forbidden).Code != "forbidden" {
		t.Fatalf("forbidden ship response=(%d,%s)", forbidden.Code, forbidden.Body.String())
	}

	short := newTestHarness(t)
	putInventory(short.repo, central, "g", 1)
	putRequest(short.repo, requestID, "branch", "APPROVED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(2)})
	insufficient := httptest.NewRecorder()
	handlerRouter(short).ServeHTTP(insufficient, httptest.NewRequest(http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/ship", nil))
	if insufficient.Code != http.StatusConflict || decodeError(t, insufficient).Code != "insufficient_stock" {
		t.Fatalf("insufficient ship response=(%d,%s)", insufficient.Code, insufficient.Body.String())
	}

	failed := newTestHarness(t)
	failed.repo.errs["getRequest"] = errors.New("lookup failed")
	internal := httptest.NewRecorder()
	handlerRouter(failed).ServeHTTP(internal, httptest.NewRequest(http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/ship", nil))
	if internal.Code != http.StatusInternalServerError || decodeError(t, internal).Code != "internal_error" {
		t.Fatalf("repository error response=(%d,%s)", internal.Code, internal.Body.String())
	}
}

func TestReceiveSupplyRequestHandlerSuccessValidationAndAuthorization(t *testing.T) {
	requestID := "00000000-0000-0000-0000-000000000040"
	itemID := "00000000-0000-0000-0000-000000000041"
	h := newTestHarness(t)
	putRequest(h.repo, requestID, "branch", "SHIPPED", &SupplyRequestItem{ID: itemID, GoodID: "g", ApprovedQty: floatPtr(4)})
	response := performJSON(t, handlerRouter(h), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/receive", map[string]any{
		"actor_role": "BRANCH_MANAGER",
		"items":      []map[string]any{{"item_id": itemID, "received_qty": 2}},
	})
	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":\"fulfilled\"}" {
		t.Fatalf("receive response=(%d,%s)", response.Code, response.Body.String())
	}
	if h.repo.inv[invKey("branch", "g")].Quantity != 2 || len(h.repo.movements) != 1 {
		t.Fatalf("receipt was not applied: inventory=%#v movements=%#v", h.repo.inv, h.repo.movements)
	}

	negativeHarness := newTestHarness(t)
	putRequest(negativeHarness.repo, requestID, "branch", "SHIPPED", &SupplyRequestItem{ID: itemID, GoodID: "g", ApprovedQty: floatPtr(4)})
	negative := performJSON(t, handlerRouter(negativeHarness), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/receive", map[string]any{
		"actor_role": "CEO",
		"items":      []map[string]any{{"item_id": itemID, "received_qty": -1}},
	})
	var negativeResponse map[string]string
	if err := json.Unmarshal(negative.Body.Bytes(), &negativeResponse); err != nil {
		t.Fatal(err)
	}
	if negative.Code != http.StatusOK || negativeResponse["status"] != "fulfilled" {
		t.Fatalf("negative quantity response=(%d,%s), want current service behavior (fulfilled without receipt)", negative.Code, negative.Body.String())
	}
	if len(negativeHarness.repo.movements) != 0 || negativeHarness.repo.inv[invKey("branch", "g")] != nil {
		t.Fatalf("negative quantity unexpectedly changed inventory: %#v", negativeHarness.repo.inv)
	}

	forbidden := performJSON(t, handlerRouter(newTestHarness(t)), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/receive", map[string]any{
		"actor_role": "SUPPLY_MANAGER",
	})
	if forbidden.Code != http.StatusForbidden || decodeError(t, forbidden).Code != "forbidden" {
		t.Fatalf("forbidden receive response=(%d,%s)", forbidden.Code, forbidden.Body.String())
	}

	failed := newTestHarness(t)
	failed.repo.errs["getRequest"] = errors.New("lookup failed")
	internal := performJSON(t, handlerRouter(failed), http.MethodPatch, "/api/v1/supply/requests/"+requestID+"/receive", map[string]any{"actor_role": "CEO"})
	if internal.Code != http.StatusInternalServerError || decodeError(t, internal).Code != "internal_error" {
		t.Fatalf("repository error response=(%d,%s)", internal.Code, internal.Body.String())
	}
}
