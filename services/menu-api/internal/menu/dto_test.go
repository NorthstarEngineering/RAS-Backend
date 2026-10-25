package menu

import (
	"testing"
	"time"
)

func TestMenuItemToResponsePreservesPublicFields(t *testing.T) {
	now := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	nameEN := "Burger"
	item := MenuItem{ID: "item-1", SKU: "SKU-1", Name: "برجر", NameEn: &nameEN, SellingPrice: 125, IsActive: true, CreatedAt: now, UpdatedAt: now}

	response := item.ToResponse()
	if response.ID != item.ID || response.SKU != item.SKU || response.NameEn != item.NameEn ||
		response.SellingPrice != item.SellingPrice || response.CreatedAt != now.Format(time.RFC3339) {
		t.Fatalf("ToResponse() = %+v, source = %+v", response, item)
	}
}
