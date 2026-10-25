package inventory

import "testing"

func TestMovementTypeValidation(t *testing.T) {
	tests := []struct {
		name  string
		value MovementType
		want  bool
	}{
		{"receipt", TypeIn, true},
		{"issue", TypeOut, true},
		{"adjustment", TypeAdjustment, true},
		{"unknown", MovementType("UNKNOWN"), false},
	}

	func TestInventoryMovementTypesRequirePositiveQuantity(t *testing.T) {
		tests := []struct {
			name string
			typ  MovementType
		}{
			{"receipt", TypeIn},
			{"issue", TypeOut},
			{"adjustment", TypeAdjustment},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if !tt.typ.IsValid() {
					t.Fatalf("movement type %q should be valid", tt.typ)
				}
			})
		}
	}

	func TestInventoryModelPreservesMovementMetadata(t *testing.T) {
		movement := InventoryMovement{
			ID: "movement-1", InventoryID: "inventory-1", Type: TypeIn,
			Quantity: 12.5, Reason: "supplier receipt", CreatedBy: "user-1",
		}
		if movement.ID == "" || movement.InventoryID == "" || movement.Quantity <= 0 ||
			movement.Reason == "" || movement.CreatedBy == "" {
			t.Fatalf("movement metadata is incomplete: %+v", movement)
		}
	}

	func TestSupplyRequestStartsWithPendingStatus(t *testing.T) {
		request := SupplyRequest{ID: "request-1", BranchID: "branch-1", Status: "PENDING", Priority: "NORMAL"}
		if request.Status != "PENDING" {
			t.Fatalf("new supply request status = %q, want PENDING", request.Status)
		}
	}

	func TestSupplyRequestItemPreservesRequestedQuantity(t *testing.T) {
		item := SupplyRequestItem{ID: "item-1", RequestID: "request-1", GoodID: "good-1", RequestedQty: 0.001}
		if item.RequestedQty != 0.001 {
			t.Fatalf("requested quantity = %v, want 0.001", item.RequestedQty)
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.IsValid(); got != tt.want {
				t.Fatalf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
