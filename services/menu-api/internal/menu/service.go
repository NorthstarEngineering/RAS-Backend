package menu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// Errors
// ============================================================

var (
	ErrRequestNotFound = errors.New("menu request not found")
	ErrItemNotFound    = errors.New("menu item not found")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrInvalidStatus   = errors.New("invalid status")
)

// ============================================================
// Service
// ============================================================

type Service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(db *sqlx.DB, repo Repository) *Service {
	return &Service{db: db, repo: repo}
}

// ============================================================
// 1. Create Menu Item Request (Branch Manager)
// Ref: FR-MENU-004
// ============================================================

type CreateRequestInput struct {
	BranchID       string
	RequestedBy    string
	ItemName       string
	RequestedPrice float64
	Description    string
	Justification  string
}

func (s *Service) CreateRequest(ctx context.Context, in CreateRequestInput) (*MenuItemRequest, error) {
	if in.ItemName == "" || in.RequestedPrice <= 0 {
		return nil, errors.New("invalid input")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	req := &MenuItemRequest{
		ID:             uuid.NewString(),
		BranchID:       in.BranchID,
		RequestedBy:    in.RequestedBy,
		ItemName:       in.ItemName,
		RequestedPrice: in.RequestedPrice,
		Description:    &in.Description,
		Justification:  &in.Justification,
		Status:         "PENDING",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.CreateMenuItemRequest(ctx, tx, req); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return req, nil
}

// ============================================================
// 2. Approve Menu Request (CEO only)
// Ref: FR-MENU-005
// ============================================================

type ApproveRequestInput struct {
	RequestID string
	ActorRole string // CEO only
	SKU       string // SKU اللي الـ CEO بيديه للصنف
}

func (s *Service) ApproveRequest(ctx context.Context, in ApproveRequestInput) (*MenuItem, error) {
	if in.ActorRole != "CEO" {
		return nil, ErrUnauthorized
	}

	req, err := s.repo.GetMenuItemRequestByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}
	if req.Status != "PENDING" {
		return nil, ErrInvalidStatus
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	// 1. تحديث حالة الطلب
	req.Status = "APPROVED"
	req.UpdatedAt = now
	if err := s.repo.UpdateMenuItemRequest(ctx, tx, req); err != nil {
		return nil, err
	}

	// 2. إنشاء صنف جديد في المنيو
	newItem := &MenuItem{
		ID:           uuid.NewString(),
		SKU:          in.SKU,
		Name:         req.ItemName,
		SellingPrice: req.RequestedPrice,
		Description:  req.Description,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.CreateMenuItem(ctx, tx, newItem); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return newItem, nil
}

// ============================================================
// 3. Reject Menu Request (CEO only)
// Ref: FR-MENU-005
// ============================================================

type RejectRequestInput struct {
	RequestID string
	ActorRole string
	Reason    string
}

func (s *Service) RejectRequest(ctx context.Context, in RejectRequestInput) (*MenuItemRequest, error) {
	if in.ActorRole != "CEO" {
		return nil, ErrUnauthorized
	}
	if in.Reason == "" {
		return nil, errors.New("rejection reason is required")
	}

	req, err := s.repo.GetMenuItemRequestByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, ErrRequestNotFound
	}
	if req.Status != "PENDING" {
		return nil, ErrInvalidStatus
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	req.Status = "REJECTED"
	req.RejectionReason = &in.Reason
	req.UpdatedAt = time.Now().UTC()

	if err := s.repo.UpdateMenuItemRequest(ctx, tx, req); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return req, nil
}

// ============================================================
// 4. List Requests & Menu Items
// ============================================================

func (s *Service) ListRequests(ctx context.Context) ([]*MenuItemRequest, error) {
	return s.repo.ListMenuItemRequests(ctx)
}

func (s *Service) ListMenuItems(ctx context.Context) ([]*MenuItem, error) {
	return s.repo.ListMenuItems(ctx)
}

func (s *Service) GetMenuItem(ctx context.Context, id string) (*MenuItem, error) {
	item, err := s.repo.GetMenuItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrItemNotFound
	}
	return item, nil
}

// ============================================================
// 5. Create Menu Item Directly (CEO)
// Ref: FR-MENU-001, FR-MENU-003
// ============================================================

type CreateMenuItemInput struct {
	ActorRole       string
	SKU             string
	Name            string
	NameEn          string
	CategoryID      string
	SellingPrice    float64
	KDSStation      string
	PrepTimeMinutes int
	Description     string
}

func (s *Service) CreateMenuItem(ctx context.Context, in CreateMenuItemInput) (*MenuItem, error) {
	if in.ActorRole != "CEO" {
		return nil, ErrUnauthorized
	}
	if in.SKU == "" || in.Name == "" || in.SellingPrice < 0 {
		return nil, errors.New("invalid input")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	var categoryPtr *string
	if in.CategoryID != "" {
		categoryPtr = &in.CategoryID
	}
	item := &MenuItem{
		ID:              uuid.NewString(),
		SKU:             in.SKU,
		Name:            in.Name,
		NameEn:          &in.NameEn,
		CategoryID:      categoryPtr, // ← دلوقتي بقى آمن
		SellingPrice:    in.SellingPrice,
		KDSStation:      &in.KDSStation,
		PrepTimeMinutes: &in.PrepTimeMinutes,
		Description:     &in.Description,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.CreateMenuItem(ctx, tx, item); err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

type CreateCategoryInput struct {
	Name      string
	SortOrder int
}

func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (*Category, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	c := &Category{
		ID:        uuid.NewString(),
		Name:      in.Name,
		SortOrder: in.SortOrder,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateCategory(ctx, tx, c); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Service) ListCategories(ctx context.Context) ([]*Category, error) {
	return s.repo.ListCategories(ctx)
}

type UpdateMenuItemInput struct {
	ActorRole       string
	ID              string
	Name            string
	NameEn          string
	CategoryID      string
	SellingPrice    float64
	KDSStation      string
	PrepTimeMinutes int
	Description     string
	IsActive        bool
}

func (s *Service) UpdateMenuItem(ctx context.Context, in UpdateMenuItemInput) (*MenuItem, error) {
	if in.ActorRole != "CEO" {
		return nil, ErrUnauthorized
	}
	existing, err := s.repo.GetMenuItemByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrItemNotFound
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	existing.Name = in.Name
	existing.NameEn = &in.NameEn
	existing.SellingPrice = in.SellingPrice
	existing.KDSStation = &in.KDSStation
	existing.PrepTimeMinutes = &in.PrepTimeMinutes
	existing.Description = &in.Description
	existing.IsActive = in.IsActive
	existing.UpdatedAt = time.Now().UTC()

	if in.CategoryID != "" {
		existing.CategoryID = &in.CategoryID
	} else {
		existing.CategoryID = nil
	}

	if err := s.repo.UpdateMenuItem(ctx, tx, existing); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return existing, nil
}
