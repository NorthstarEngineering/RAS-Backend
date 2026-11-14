package sales

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func handlerForTest(t *testing.T) (*gin.Engine, *fakeRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newFakeRepo()
	svc := NewService(newTestDB(t, &sqlTestState{}), repo)
	router := gin.New()
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router, repo
}
func requestJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}
func responseErrorCode(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v (%s)", err, res.Body.String())
	}
	return body.Error.Code
}
func TestOpenShiftHandlerSuccessAndValidation(t *testing.T) {
	router, _ := handlerForTest(t)
	res := requestJSON(t, router, http.MethodPost, "/api/v1/sales/shifts/open", map[string]any{"branch_id": "00000000-0000-4000-8000-000000000001", "manager_id": "00000000-0000-4000-8000-000000000002", "opening_float": 50})
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var got ShiftResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "OPEN" || got.OpeningFloat != 50 || got.ID == "" || got.OpenedAt == "" {
		t.Fatalf("unexpected response: %#v", got)
	}
	bad := requestJSON(t, router, http.MethodPost, "/api/v1/sales/shifts/open", map[string]any{"branch_id": "bad", "manager_id": "bad"})
	if bad.Code != http.StatusBadRequest || responseErrorCode(t, bad) != "invalid_payload" {
		t.Fatalf("validation response=(%d,%s)", bad.Code, bad.Body.String())
	}
}
func TestCloseShiftHandlerSuccessAndErrorMapping(t *testing.T) {
	router, repo := handlerForTest(t)
	id := "00000000-0000-4000-8000-000000000010"
	repo.shifts[id] = shift(id, "REVIEW")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodPatch, "/api/v1/sales/shifts/"+id+"/close", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("close status=%d body=%s", res.Code, res.Body.String())
	}
	var closed ShiftResponse
	if err := json.Unmarshal(res.Body.Bytes(), &closed); err != nil {
		t.Fatal(err)
	}
	if closed.Status != "CLOSED" || closed.ClosedAt == nil || repo.updatedShiftStatus != "CLOSED" {
		t.Fatalf("unexpected close response/state: %#v repo=%#v", closed, repo)
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodPatch, "/api/v1/sales/shifts/missing/close", nil))
	if missing.Code != http.StatusNotFound || responseErrorCode(t, missing) != "not_found" {
		t.Fatalf("missing response=(%d,%s)", missing.Code, missing.Body.String())
	}
	closedID := "00000000-0000-4000-8000-000000000011"
	repo.shifts[closedID] = shift(closedID, "CLOSED")
	already := httptest.NewRecorder()
	router.ServeHTTP(already, httptest.NewRequest(http.MethodPatch, "/api/v1/sales/shifts/"+closedID+"/close", nil))
	if already.Code != http.StatusInternalServerError || responseErrorCode(t, already) != "internal_error" {
		t.Fatalf("already closed mapping=(%d,%s)", already.Code, already.Body.String())
	}
}
func TestRecordSaleHandlerValidationSnapshotAndErrorMapping(t *testing.T) {
	router, repo := handlerForTest(t)
	shiftID := "00000000-0000-4000-8000-000000000020"
	itemID := "00000000-0000-4000-8000-000000000021"
	repo.shifts[shiftID] = shift(shiftID, "OPEN")
	repo.prices[itemID] = 7.5
	body := map[string]any{"shift_id": shiftID, "channel": "RESTAURANT", "items": []map[string]any{{"menu_item_id": itemID, "quantity": 2}}}
	res := requestJSON(t, router, http.MethodPost, "/api/v1/sales/", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	var got SaleResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.GrossAmount != 15 || got.NetAmount != 15 || got.Channel != "RESTAURANT" || len(got.Items) != 1 || got.Items[0].UnitPrice != 7.5 || got.Items[0].TotalPrice != 15 {
		t.Fatalf("unexpected sale response: %#v", got)
	}
	if repo.createdSale.CreatedBy != "99999999-9999-9999-9999-999999999999" {
		t.Fatalf("actor fallback=%q", repo.createdSale.CreatedBy)
	}
	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{"invalid channel", map[string]any{"shift_id": shiftID, "channel": "KIOSK", "items": []map[string]any{{"menu_item_id": itemID, "quantity": 1}}}},
		{"empty items", map[string]any{"shift_id": shiftID, "channel": "DELIVERY", "items": []any{}}},
		{"zero quantity", map[string]any{"shift_id": shiftID, "channel": "DELIVERY", "items": []map[string]any{{"menu_item_id": itemID, "quantity": 0}}}},
		{"negative quantity", map[string]any{"shift_id": shiftID, "channel": "DELIVERY", "items": []map[string]any{{"menu_item_id": itemID, "quantity": -1}}}},
		{"invalid item id", map[string]any{"shift_id": shiftID, "channel": "DELIVERY", "items": []map[string]any{{"menu_item_id": "bad", "quantity": 1}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := requestJSON(t, router, http.MethodPost, "/api/v1/sales/", tc.body)
			if bad.Code != http.StatusBadRequest || responseErrorCode(t, bad) != "invalid_payload" {
				t.Fatalf("response=(%d,%s)", bad.Code, bad.Body.String())
			}
		})
	}
	missingPrice := map[string]any{"shift_id": shiftID, "channel": "DELIVERY", "items": []map[string]any{{"menu_item_id": "00000000-0000-4000-8000-000000000022", "quantity": 1}}}
	notFound := requestJSON(t, router, http.MethodPost, "/api/v1/sales/", missingPrice)
	if notFound.Code != http.StatusNotFound || responseErrorCode(t, notFound) != "not_found" {
		t.Fatalf("missing menu mapping=(%d,%s)", notFound.Code, notFound.Body.String())
	}
	closedID := "00000000-0000-4000-8000-000000000023"
	repo.shifts[closedID] = shift(closedID, "CLOSED")
	invalidState := requestJSON(t, router, http.MethodPost, "/api/v1/sales/", map[string]any{"shift_id": closedID, "channel": "DELIVERY", "items": []map[string]any{{"menu_item_id": itemID, "quantity": 1}}})
	if invalidState.Code != http.StatusConflict || responseErrorCode(t, invalidState) != "invalid_state" {
		t.Fatalf("closed shift response=(%d,%s)", invalidState.Code, invalidState.Body.String())
	}
}
func TestDiscountHandlersValidationAndCancellation(t *testing.T) {
	router, repo := handlerForTest(t)
	shiftID := "00000000-0000-4000-8000-000000000030"
	repo.shifts[shiftID] = shift(shiftID, "OPEN")
	created := requestJSON(t, router, http.MethodPost, "/api/v1/sales/discounts", map[string]any{"shift_id": shiftID, "amount": 2.5, "reason": "customer recovery"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var discount DiscountResponse
	if err := json.Unmarshal(created.Body.Bytes(), &discount); err != nil {
		t.Fatal(err)
	}
	if discount.Status != "ACTIVE" || discount.Amount != 2.5 || discount.ID == "" {
		t.Fatalf("unexpected discount response: %#v", discount)
	}
	for _, amount := range []float64{0, -1} {
		bad := requestJSON(t, router, http.MethodPost, "/api/v1/sales/discounts", map[string]any{"shift_id": shiftID, "amount": amount, "reason": "reason"})
		if bad.Code != http.StatusBadRequest || responseErrorCode(t, bad) != "invalid_payload" {
			t.Fatalf("amount %v response=(%d,%s)", amount, bad.Code, bad.Body.String())
		}
	}
	repo.discounts[discount.ID] = &Discount{ID: discount.ID, ShiftID: shiftID, Amount: 2.5, Status: "ACTIVE"}
	canceled := requestJSON(t, router, http.MethodPatch, "/api/v1/sales/discounts/"+discount.ID+"/cancel", map[string]any{"reason": "duplicate"})
	if canceled.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", canceled.Code, canceled.Body.String())
	}
	var canceledBody DiscountResponse
	if err := json.Unmarshal(canceled.Body.Bytes(), &canceledBody); err != nil {
		t.Fatal(err)
	}
	if canceledBody.Status != "CANCELED" || canceledBody.CanceledReason == nil || *canceledBody.CanceledReason != "duplicate" {
		t.Fatalf("unexpected canceled response: %#v", canceledBody)
	}
	badReason := requestJSON(t, router, http.MethodPatch, "/api/v1/sales/discounts/x/cancel", map[string]any{})
	if badReason.Code != http.StatusBadRequest || responseErrorCode(t, badReason) != "invalid_payload" {
		t.Fatalf("cancel validation=(%d,%s)", badReason.Code, badReason.Body.String())
	}
	missing := requestJSON(t, router, http.MethodPatch, "/api/v1/sales/discounts/missing/cancel", map[string]any{"reason": "x"})
	if missing.Code != http.StatusNotFound || responseErrorCode(t, missing) != "not_found" {
		t.Fatalf("missing discount=(%d,%s)", missing.Code, missing.Body.String())
	}
}
func TestShiftSummaryHandlerResponseAndRepositoryError(t *testing.T) {
	router, repo := handlerForTest(t)
	id := "00000000-0000-4000-8000-000000000040"
	repo.shifts[id] = shift(id, "CLOSED")
	repo.channels = []ChannelSummary{{Channel: "DELIVERY", TotalSales: 12, OrderCount: 1}}
	repo.items = []ItemSummary{{MenuItemID: "m", ItemName: "Tea", TotalQuantity: 2, TotalSales: 12}}
	repo.gross = 15
	repo.discountsTotal = 3
	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/v1/sales/shifts/"+id+"/summary", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("summary status=%d body=%s", res.Code, res.Body.String())
	}
	var got ShiftSummaryResponse
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.GrossSales != 15 || got.TotalDiscounts != 3 || got.NetSales != 12 || got.TotalOrders != 1 || len(got.Channels) != 1 || len(got.Items) != 1 {
		t.Fatalf("unexpected summary: %#v", got)
	}
	repo.errs["GetShiftItemSummary"] = errors.New("summary unavailable")
	failed := httptest.NewRecorder()
	router.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/api/v1/sales/shifts/"+id+"/summary", nil))
	if failed.Code != http.StatusInternalServerError || responseErrorCode(t, failed) != "internal_error" {
		t.Fatalf("error mapping=(%d,%s)", failed.Code, failed.Body.String())
	}
}
