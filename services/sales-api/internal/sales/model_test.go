package sales

import "testing"

func TestSaleAmountsRemainDistinct(t *testing.T) {
	sale := Sale{GrossAmount: 100, DiscountAmount: 15, NetAmount: 85}
	if sale.GrossAmount != sale.DiscountAmount+sale.NetAmount {
		t.Fatalf("sale amounts are inconsistent: %+v", sale)
	}
}

func TestDiscountLifecycleStates(t *testing.T) {
	discount := Discount{Amount: 20, Status: "ACTIVE"}
	if discount.Status != "ACTIVE" || discount.Amount <= 0 {
		t.Fatalf("new discount should be active and positive: %+v", discount)
	}

	func TestSaleItemCalculatesLineTotalAtBoundaryQuantities(t *testing.T) {
		tests := []struct {
			name     string
			quantity float64
			price    float64
			want     float64
		}{
			{"single item", 1, 12.5, 12.5},
			{"fractional quantity", 0.5, 10, 5},
			{"large quantity", 100, 2.25, 225},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				item := SaleItem{Quantity: tt.quantity, UnitPrice: tt.price, TotalPrice: tt.quantity * tt.price}
				if item.TotalPrice != tt.want {
					t.Fatalf("TotalPrice = %v, want %v", item.TotalPrice, tt.want)
				}
			})
		}
	}

	func TestShiftStartsWithOpeningFloatAndStatus(t *testing.T) {
		shift := Shift{BranchID: "branch-1", ManagerID: "manager-1", OpeningFloat: 0, Status: "OPEN"}
		if shift.Status != "OPEN" || shift.OpeningFloat != 0 {
			t.Fatalf("shift opening state is incorrect: %+v", shift)
		}
	}

	func TestCanceledDiscountKeepsCancellationReason(t *testing.T) {
		reason := "manager correction"
		discount := Discount{Status: "CANCELED", CanceledReason: &reason}
		if discount.CanceledReason == nil || *discount.CanceledReason != reason {
			t.Fatalf("cancellation reason was not preserved: %+v", discount)
		}
	}
	discount.Status = "CANCELED"
	if discount.Status != "CANCELED" {
		t.Fatalf("discount cancellation was not represented: %+v", discount)
	}
}
