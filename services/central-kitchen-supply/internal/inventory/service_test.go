package inventory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecordMovementValidation(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name    string
		input   RecordMovementInput
		wantErr error
	}{
		{
			name:    "invalid movement type",
			input:   RecordMovementInput{Type: "INVALID", Quantity: 1},
			wantErr: ErrInvalidMovementType,
		},
		{
			name:    "zero quantity",
			input:   RecordMovementInput{Type: "IN", Quantity: 0},
			wantErr: ErrInvalidQuantity,
		},
		{
			name:    "negative quantity",
			input:   RecordMovementInput{Type: "IN", Quantity: -1},
			wantErr: ErrInvalidQuantity,
		},
		{
			name:    "OUT without reason",
			input:   RecordMovementInput{Type: "OUT", Quantity: 1},
			wantErr: ErrReasonRequired,
		},
		{
			name:    "ADJUSTMENT without reason",
			input:   RecordMovementInput{Type: "ADJUSTMENT", Quantity: 1},
			wantErr: ErrReasonRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.RecordMovement(context.Background(), tt.input)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCreateGoodValidation(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name  string
		input CreateGoodInput
	}{
		{
			name: "missing SKU",
			input: CreateGoodInput{
				Name: "Rice", Unit: "kg",
			},
		},
		{
			name: "missing name",
			input: CreateGoodInput{
				SKU: "RICE-01", Unit: "kg",
			},
		},
		{
			name: "missing unit",
			input: CreateGoodInput{
				SKU: "RICE-01", Name: "Rice",
			},
		},
		{
			name: "negative unit cost",
			input: CreateGoodInput{
				SKU: "RICE-01", Name: "Rice",
				Unit: "kg", UnitCost: -1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.CreateGood(context.Background(), tt.input)

			if !errors.Is(err, ErrInvalidQuantity) {
				t.Fatalf("expected ErrInvalidQuantity, got %v", err)
			}
		})
	}
}

func TestCreateSupplyRequestRequiresBranchAndItems(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name  string
		input CreateSupplyRequestInput
	}{
		{
			name: "missing branch",
			input: CreateSupplyRequestInput{
				Items: []SupplyRequestItemInput{
					{GoodID: "good-1", RequestedQty: 5},
				},
			},
		},
		{
			name: "missing items",
			input: CreateSupplyRequestInput{
				BranchID: "branch-1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.CreateSupplyRequest(context.Background(), tt.input)

			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestSupplyRequestRoleAuthorization(t *testing.T) {
	s := &Service{}
	ctx := context.Background()

	t.Run("branch manager cannot approve", func(t *testing.T) {
		_, err := s.ApproveSupplyRequest(ctx, ApproveSupplyRequestInput{
			ActorRole: "BRANCH_MANAGER",
		})
		if !errors.Is(err, ErrForbiddenAction) {
			t.Fatalf("expected ErrForbiddenAction, got %v", err)
		}

	})

	t.Run("branch manager cannot reject", func(t *testing.T) {
		_, err := s.RejectSupplyRequest(ctx, RejectSupplyRequestInput{
			ActorRole: "BRANCH_MANAGER",
			Reason:    "test",
		})
		if !errors.Is(err, ErrForbiddenAction) {
			t.Fatalf("expected ErrForbiddenAction, got %v", err)
		}
	})

	t.Run("branch manager cannot ship", func(t *testing.T) {
		err := s.ShipSupplyRequest(ctx, "BRANCH_MANAGER", "request-1")
		if !errors.Is(err, ErrForbiddenAction) {
			t.Fatalf("expected ErrForbiddenAction, got %v", err)
		}
	})

	t.Run("supply manager cannot receive", func(t *testing.T) {
		err := s.ReceiveSupplyRequest(ctx, ReceiveSupplyRequestInput{
			ActorRole: "SUPPLY_MANAGER",
		})
		if !errors.Is(err, ErrForbiddenAction) {
			t.Fatalf("expected ErrForbiddenAction, got %v", err)
		}
	})
}

func TestCreateGoodHandlerRejectsMalformedJSONAndMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"name":`},
		{name: "missing SKU", body: `{"name":"Rice","category":"Grain","unit":"kg","unit_cost":2}`},
		{name: "missing name", body: `{"sku":"RICE-01","category":"Grain","unit":"kg","unit_cost":2}`},
		{name: "missing unit", body: `{"sku":"RICE-01","name":"Rice","category":"Grain","unit_cost":2}`},
		{name: "missing unit cost", body: `{"sku":"RICE-01","name":"Rice","category":"Grain","unit":"kg"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := inventoryPerformRequest(inventoryTestRouter(&Service{}), http.MethodPost, "/api/v1/inventory/goods", tt.body)
			assertInventoryError(t, res, http.StatusBadRequest, "invalid_payload", "")
		})
	}
}

func TestCreateGoodHandlerRejectsInvalidUnitCostBeforeDatabaseAccess(t *testing.T) {
	tests := []struct {
		name string
		cost string
	}{
		{name: "negative unit cost", cost: `-1`},
		{name: "malformed unit cost", cost: `"not-a-number"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"sku":"RICE-01","name":"Rice","category":"Grain","unit":"kg","unit_cost":` + tt.cost + `}`
			res := inventoryPerformRequest(inventoryTestRouter(&Service{}), http.MethodPost, "/api/v1/inventory/goods", body)
			assertInventoryError(t, res, http.StatusBadRequest, "invalid_payload", "")
		})
	}
}

func TestRecordMovementHandlerRejectsMalformedJSONAndInvalidDTOFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"good_id":`},
		{name: "missing good ID", body: `{"type":"IN","quantity":1}`},
		{name: "invalid good ID", body: `{"good_id":"not-a-uuid","type":"IN","quantity":1}`},
		{name: "invalid movement type", body: `{"good_id":"11111111-1111-4111-8111-111111111111","type":"INVALID","quantity":1}`},
		{name: "zero quantity", body: `{"good_id":"11111111-1111-4111-8111-111111111111","type":"IN","quantity":0}`},
		{name: "negative quantity", body: `{"good_id":"11111111-1111-4111-8111-111111111111","type":"IN","quantity":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := inventoryPerformRequest(inventoryTestRouter(&Service{}), http.MethodPost, "/api/v1/inventory/movements", tt.body)
			assertInventoryError(t, res, http.StatusBadRequest, "invalid_payload", "")
		})
	}
}

func TestRecordMovementHandlerMapsMissingReasonBeforeDatabaseAccess(t *testing.T) {
	for _, movementType := range []string{"OUT", "ADJUSTMENT"} {
		t.Run(movementType, func(t *testing.T) {
			body := `{"good_id":"11111111-1111-4111-8111-111111111111","type":"` + movementType + `","quantity":1}`
			res := inventoryPerformRequest(inventoryTestRouter(&Service{}), http.MethodPost, "/api/v1/inventory/movements", body)
			assertInventoryError(t, res, http.StatusBadRequest, "validation_error", ErrReasonRequired.Error())
		})
	}
}

type inventoryGoodLookupStub struct {
	Repository
	good *Good
	err  error
}

func (r *inventoryGoodLookupStub) GetGoodByID(_ context.Context, _ string) (*Good, error) {
	return r.good, r.err
}

func TestGetGoodHandlerSerializesGoodFieldsFromRepository(t *testing.T) {
	want := &Good{
		ID: "good-1", SKU: "RICE-01", Name: "Rice", Category: "Grain",
		Unit: "kg", UnitCost: 2.5, IsActive: true,
	}
	svc := NewService(nil, &inventoryGoodLookupStub{good: want})
	res := inventoryPerformRequest(inventoryTestRouter(svc), http.MethodGet, "/api/v1/inventory/goods/good-1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}

	var got Good
	if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
		t.Fatalf("GET body is not a Good JSON response: %v; body=%s", err, res.Body.String())
	}
	if got.ID != want.ID || got.SKU != want.SKU || got.Name != want.Name || got.Category != want.Category || got.Unit != want.Unit || got.UnitCost != want.UnitCost || got.IsActive != want.IsActive {
		t.Fatalf("GET good = %+v, want fields from repository value %+v", got, want)
	}
}

func TestGetGoodHandlerReturnsNotFoundForMissingGood(t *testing.T) {
	svc := NewService(nil, &inventoryGoodLookupStub{})
	res := inventoryPerformRequest(inventoryTestRouter(svc), http.MethodGet, "/api/v1/inventory/goods/missing-good", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("GET missing good status = %d, want %d (documented response); body=%s", res.Code, http.StatusNotFound, res.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET missing good body is not valid ErrorResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if body.Error.Code == "" {
		t.Fatalf("GET missing good error code is empty; body=%s", res.Body.String())
	}
}

func TestGetGoodHandlerMapsRepositoryErrorToInternalError(t *testing.T) {
	svc := NewService(nil, &inventoryGoodLookupStub{err: errors.New("repository unavailable")})
	res := inventoryPerformRequest(inventoryTestRouter(svc), http.MethodGet, "/api/v1/inventory/goods/good-1", "")
	assertInventoryError(t, res, http.StatusInternalServerError, "internal_error", "something went wrong")
}

func TestWriteDomainErrorMapsInsufficientStock(t *testing.T) {
	res := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(res)

	writeDomainError(c, ErrInsufficientStock)

	assertInventoryError(t, res, http.StatusConflict, "insufficient_stock", "")
}

func TestGetGoodHandlerAllowsRequestWithoutAuthentication(t *testing.T) {
	want := &Good{ID: "good-1", SKU: "RICE-01", Name: "Rice", Unit: "kg", UnitCost: 2.5}
	svc := NewService(nil, &inventoryGoodLookupStub{good: want})
	res := inventoryPerformRequest(inventoryTestRouter(svc), http.MethodGet, "/api/v1/inventory/goods/good-1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("unauthenticated GET status = %d, want %d based on current route registration; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
}

func TestGetGoodByIDReturnsNotFoundWhenRepositoryReturnsNil(t *testing.T) {
	svc := NewService(nil, &inventoryGoodLookupStub{})

	good, err := svc.GetGoodByID(context.Background(), "missing-good")
	if good != nil {
		t.Fatalf("GetGoodByID() good = %+v, want nil", good)
	}
	if !errors.Is(err, ErrGoodNotFound) {
		t.Fatalf("GetGoodByID() error = %v, want %v", err, ErrGoodNotFound)
	}
}

func TestGetGoodByIDPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("repository unavailable")
	svc := NewService(nil, &inventoryGoodLookupStub{err: wantErr})

	good, err := svc.GetGoodByID(context.Background(), "good-1")
	if good != nil {
		t.Fatalf("GetGoodByID() good = %+v, want nil", good)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetGoodByID() error = %v, want %v", err, wantErr)
	}
}

func inventoryTestRouter(svc *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func inventoryPerformRequest(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func assertInventoryError(t *testing.T, res *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if res.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", res.Code, status, res.Body.String())
	}
	var body ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid ErrorResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q; body=%s", body.Error.Code, code, res.Body.String())
	}
	if message != "" && body.Error.Message != message {
		t.Fatalf("error message = %q, want %q", body.Error.Message, message)
	}
}
