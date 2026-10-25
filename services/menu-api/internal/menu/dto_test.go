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
	if response.ID != item.ID || response.SKU != item.SKU || response.NameEn != item.NameEn || response.SellingPrice != item.SellingPrice || response.CreatedAt != now.Format(time.RFC3339) {
		t.Fatalf("ToResponse() = %+v, source = %+v", response, item)
	}
}

func TestMenuItemToResponseKeepsOptionalFieldsNil(t *testing.T) {
	item := MenuItem{ID: "item-1", SKU: "SKU-1", Name: "Item", SellingPrice: 0}
	response := item.ToResponse()
	if response.NameEn != nil || response.CategoryID != nil || response.KDSStation != nil || response.PrepTimeMinutes != nil || response.Description != nil {
		t.Fatalf("optional fields should remain nil: %+v", response)
	}
}

func TestMenuRequestToResponsePreservesApprovalState(t *testing.T) {
	reason := "duplicate item"
	request := MenuItemRequest{ID: "request-1", ItemName: "Item", RequestedPrice: 0.01, Status: "REJECTED", RejectionReason: &reason}
	response := request.ToResponse()
	if response.Status != "REJECTED" || response.RejectionReason == nil || *response.RejectionReason != reason || response.RequestedPrice != 0.01 {
		t.Fatalf("request mapping is incorrect: %+v", response)
	}
}

func TestCategoryResponsePreservesSortOrderBoundary(t *testing.T) {
	category := Category{ID: "category-1", Name: "Drinks", SortOrder: 0, IsActive: true}
	response := category.ToResponse()
	if response.SortOrder != 0 || !response.IsActive {
		t.Fatalf("category mapping is incorrect: %+v", response)
	}
}

func TestMenuItemToResponseFormatsUTC(t *testing.T) {
	localTime := time.Date(2026, 10, 25, 3, 0, 0, 0, time.FixedZone("test", 3*60*60))
	response := (MenuItem{CreatedAt: localTime, UpdatedAt: localTime}).ToResponse()
	if response.CreatedAt != "2026-10-25T00:00:00Z" || response.UpdatedAt != "2026-10-25T00:00:00Z" {
		t.Fatalf("timestamps were not normalized to UTC: %+v", response)
	}
}
