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
	discount.Status = "CANCELED"
	if discount.Status != "CANCELED" {
		t.Fatalf("discount cancellation was not represented: %+v", discount)
	}
}
