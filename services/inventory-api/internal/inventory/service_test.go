package inventory

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestInventoryStatusAndValue(t *testing.T) {
	inventory := Inventory{
		Quantity: decimal.NewFromInt(3), UnitCost: decimal.NewFromInt(10),
		MinStock: decimal.NewFromInt(5), ReorderLevel: decimal.NewFromInt(8),
	}
	if got := inventory.Status(); got != StatusCritical {
		t.Fatalf("Status() = %v, want %v", got, StatusCritical)
	}
	if got := inventory.TotalValue(); !got.Equal(decimal.NewFromInt(30)) {
		t.Fatalf("TotalValue() = %s, want 30", got)
	}
	if got := inventory.Shortage(); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("Shortage() = %s, want 2", got)
	}
}

func TestApplyMovementHandlesReceiptsIssuesAndAdjustments(t *testing.T) {
	qty := decimal.NewFromInt(10)
	receipt := CreateMovementRequest{Type: TypeIn, Quantity: decimalPointer(decimal.NewFromInt(4))}
	got, recorded, err := applyMovement(qty, receipt)
	if err != nil || !got.Equal(decimal.NewFromInt(14)) || !recorded.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("receipt result = %s, %s, %v", got, recorded, err)
	}

	issue := CreateMovementRequest{Type: TypeOut, Quantity: decimalPointer(decimal.NewFromInt(3))}
	got, recorded, err = applyMovement(qty, issue)
	if err != nil || !got.Equal(decimal.NewFromInt(7)) || !recorded.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("issue result = %s, %s, %v", got, recorded, err)
	}
}

func decimalPointer(value decimal.Decimal) *decimal.Decimal { return &value }
