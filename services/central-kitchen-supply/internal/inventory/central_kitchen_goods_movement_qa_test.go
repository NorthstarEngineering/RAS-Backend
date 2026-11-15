package inventory

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestQACreateGoodHandlerReturnsSupportedFieldsAndDefaultStatus(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	repo := &centralKitchenGoodsMovementQARepository{}
	service := NewService(db, repo)
	body := `{"sku":"FLOUR-01","name":"Bread Flour","category":"dry goods","unit":"kg","unit_cost":2.75}`
	res := supplyRequestQALifecycleRequest(
		supplyRequestQALifecycleRouter(service, "", ""),
		http.MethodPost,
		"/api/v1/inventory/goods",
		body,
	)

	if res.Code != http.StatusCreated {
		t.Fatalf("create good status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	var response Good
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("create good body is not Good JSON: %v; body=%s", err, res.Body.String())
	}
	if response.SKU != "FLOUR-01" || response.Name != "Bread Flour" || response.Category != "dry goods" || response.Unit != "kg" || response.UnitCost != 2.75 || !response.IsActive {
		t.Fatalf("create good response = %+v, want submitted fields and default active status", response)
	}
	if repo.createdGood == nil || repo.createdGood.SKU != response.SKU || repo.createdGood.Name != response.Name || repo.createdGood.Category != response.Category || repo.createdGood.Unit != response.Unit || repo.createdGood.UnitCost != response.UnitCost || !repo.createdGood.IsActive {
		t.Fatalf("repository stub received good = %+v, want generated good with submitted fields and active status", repo.createdGood)
	}
	assertSupplyRequestQACommit(t, txStats)
}

func TestQARecordMovementHandlerAppliesInMovementQuantity(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	repo := &centralKitchenGoodsMovementQARepository{
		inventory: &Inventory{
			ID: "inventory-1", BranchID: supplyRequestQACentralKitchenID, GoodID: supplyRequestQAGoodID, Quantity: 5,
		},
	}
	service := NewService(db, repo)
	body := `{"branch_id":"` + supplyRequestQACentralKitchenID + `","actor_id":"manager-1","good_id":"` + supplyRequestQAGoodID + `","type":"IN","quantity":3}`
	res := supplyRequestQALifecycleRequest(
		supplyRequestQALifecycleRouter(service, "", ""),
		http.MethodPost,
		"/api/v1/inventory/movements",
		body,
	)

	if res.Code != http.StatusCreated {
		t.Fatalf("record movement status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	var response CreateMovementResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("record movement body is not CreateMovementResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if response.Inventory.Quantity != 8 || response.Inventory.BranchID != supplyRequestQACentralKitchenID || response.Inventory.GoodID != supplyRequestQAGoodID {
		t.Fatalf("movement inventory response = %+v, want quantity 8 for Central Kitchen and requested good", response.Inventory)
	}
	if response.Movement.Type != string(TypeIn) || response.Movement.Quantity != 3 || response.Movement.CreatedBy != "manager-1" {
		t.Fatalf("movement response = %+v, want IN quantity 3 created by manager-1", response.Movement)
	}
	if repo.inventory == nil || repo.inventory.Quantity != 8 || repo.inventoryUpdateCalls != 1 || len(repo.movements) != 1 || repo.movements[0].Quantity != 3 || repo.movements[0].Type != TypeIn {
		t.Fatalf("stub inventory/movements = %+v/%+v, want stock 8 and one IN movement of 3", repo.inventory, repo.movements)
	}
	assertSupplyRequestQACommit(t, txStats)
}

type centralKitchenGoodsMovementQARepository struct {
	Repository
	createdGood          *Good
	inventory            *Inventory
	inventoryUpdateCalls int
	movements            []*InventoryMovement
}

func (r *centralKitchenGoodsMovementQARepository) CreateGood(_ context.Context, _ *sqlx.Tx, good *Good) error {
	r.createdGood = good
	return nil
}

func (r *centralKitchenGoodsMovementQARepository) GetInventoryByBranchAndGood(_ context.Context, branchID, goodID string) (*Inventory, error) {
	if r.inventory != nil && r.inventory.BranchID == branchID && r.inventory.GoodID == goodID {
		return r.inventory, nil
	}
	return nil, nil
}

func (r *centralKitchenGoodsMovementQARepository) UpdateInventoryQuantity(_ context.Context, _ *sqlx.Tx, _ string, quantity float64, _ time.Time) error {
	r.inventoryUpdateCalls++
	r.inventory.Quantity = quantity
	return nil
}

func (r *centralKitchenGoodsMovementQARepository) CreateMovement(_ context.Context, _ *sqlx.Tx, movement *InventoryMovement) error {
	r.movements = append(r.movements, movement)
	return nil
}
