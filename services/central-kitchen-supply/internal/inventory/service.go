package inventory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Service struct {
	db   *sqlx.DB
	repo Repository
}

func NewService(db *sqlx.DB, repo Repository) *Service {
	return &Service{db: db, repo: repo}
}

// ============================================================
// 1. Inventory Movements (IN / OUT / ADJUSTMENT)
// ============================================================

type RecordMovementInput struct {
	BranchID string
	ActorID  string
	GoodID   string
	Type     string
	Quantity float64
	Reason   string
}

func (s *Service) RecordMovement(ctx context.Context, in RecordMovementInput) (*CreateMovementResponse, error) {
	mType := MovementType(in.Type)
	if !mType.IsValid() {
		return nil, ErrInvalidMovementType
	}
	if in.Quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	if (mType == TypeOut || mType == TypeAdjustment) && in.Reason == "" {
		return nil, ErrReasonRequired
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()

	inv, err := s.repo.GetInventoryByBranchAndGood(ctx, in.BranchID, in.GoodID)
	if err != nil {
		return nil, err
	}

	if inv == nil {
		inv = &Inventory{
			ID:        uuid.NewString(),
			BranchID:  in.BranchID,
			GoodID:    in.GoodID,
			Quantity:  0,
			UpdatedAt: now,
		}
		if err := s.repo.CreateInventory(ctx, tx, inv); err != nil {
			return nil, err
		}
	}

	oldQty := inv.Quantity
	newQty := oldQty

	switch mType {
	case TypeIn:
		newQty = oldQty + in.Quantity
	case TypeOut:
		newQty = oldQty - in.Quantity
		if newQty < 0 {
			return nil, ErrInsufficientStock
		}
	case TypeAdjustment:
		newQty = in.Quantity
	}

	if err := s.repo.UpdateInventoryQuantity(ctx, tx, inv.ID, newQty, now); err != nil {
		return nil, err
	}

	mov := &InventoryMovement{
		ID:          uuid.NewString(),
		InventoryID: inv.ID,
		Type:        mType,
		Quantity:    in.Quantity,
		Reason:      in.Reason,
		CreatedBy:   in.ActorID,
		CreatedAt:   now,
	}
	if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	inv.Quantity = newQty
	inv.UpdatedAt = now

	return &CreateMovementResponse{
		Inventory: inv.ToResponse(),
		Movement:  mov.ToResponse(),
	}, nil
}

// ============================================================
// 2. Goods (Create / List / Get)
// ============================================================

type CreateGoodInput struct {
	SKU          string
	Name         string
	Category     string
	Description  string
	Unit         string
	UnitCost     float64
	ReorderLevel float64
	MaxCapacity  float64
	InitialQty   float64
}

func (s *Service) CreateGood(ctx context.Context, in CreateGoodInput) (*Good, error) {
	if in.Name == "" || in.SKU == "" || in.Unit == "" {
		return nil, ErrInvalidQuantity
	}
	if in.UnitCost < 0 {
		return nil, ErrInvalidQuantity
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	goodID := uuid.NewString()

	g := &Good{
		ID:           goodID,
		SKU:          in.SKU,
		Name:         in.Name,
		Category:     in.Category,
		Description:  in.Description,
		Unit:         in.Unit,
		UnitCost:     in.UnitCost,
		ReorderLevel: in.ReorderLevel,
		MaxCapacity:  in.MaxCapacity,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.CreateGood(ctx, tx, g); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return g, nil
}

func (s *Service) ListGoods(ctx context.Context) ([]*Good, error) {
	return s.repo.ListGoods(ctx)
}

func (s *Service) GetGoodByID(ctx context.Context, id string) (*Good, error) {
	g, err := s.repo.GetGoodByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, ErrGoodNotFound
	}
	return g, nil
}

// ============================================================
// 3. Supply Requests (Create / List)
// ============================================================

type CreateSupplyRequestInput struct {
	BranchID  string
	CreatedBy string
	Notes     string
	Priority  string
	Items     []SupplyRequestItemInput
}

type SupplyRequestItemInput struct {
	GoodID       string
	RequestedQty float64
}

func (s *Service) CreateSupplyRequest(ctx context.Context, in CreateSupplyRequestInput) (*SupplyRequest, error) {
	if in.BranchID == "" || len(in.Items) == 0 {
		return nil, errors.New("invalid input: branch_id and items are required")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC()
	req := &SupplyRequest{
		ID:        uuid.NewString(),
		BranchID:  in.BranchID,
		Status:    "PENDING",
		CreatedBy: in.CreatedBy,
		Notes:     &in.Notes,
		Priority:  in.Priority,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.CreateSupplyRequest(ctx, tx, req); err != nil {
		return nil, err
	}

	for _, item := range in.Items {
		sItem := &SupplyRequestItem{
			ID:           uuid.NewString(),
			RequestID:    req.ID,
			GoodID:       item.GoodID,
			RequestedQty: item.RequestedQty,
			CreatedAt:    now,
		}
		if err := s.repo.CreateSupplyRequestItem(ctx, tx, sItem); err != nil {
			return nil, err
		}
		req.Items = append(req.Items, sItem)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Service) ListSupplyRequests(ctx context.Context) ([]*SupplyRequest, error) {
	return s.repo.ListSupplyRequests(ctx)
}

// ============================================================
// 4. Supply Requests (Approve / Reject)
// ============================================================

type ApproveSupplyRequestInput struct {
	RequestID string
	ActorRole string
	Items     []ApproveItemInput
}

type ApproveItemInput struct {
	ItemID      string
	ApprovedQty float64
}

func (s *Service) ApproveSupplyRequest(ctx context.Context, in ApproveSupplyRequestInput) (*SupplyRequest, error) {
	if in.ActorRole != "SUPPLY_MANAGER" && in.ActorRole != "CEO" {
		return nil, ErrForbiddenAction
	}

	req, err := s.repo.GetSupplyRequestByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errors.New("supply request not found")
	}
	if req.Status != "PENDING" {
		return nil, errors.New("request is not pending")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	for i, item := range req.Items {
		for _, appItem := range in.Items {
			if item.ID == appItem.ItemID {
				req.Items[i].ApprovedQty = &appItem.ApprovedQty
				if err := s.repo.UpdateSupplyRequestItem(ctx, tx, req.Items[i]); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := s.repo.UpdateSupplyRequestStatus(ctx, tx, req.ID, "APPROVED"); err != nil {
		return nil, err
	}
	req.Status = "APPROVED"

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

type RejectSupplyRequestInput struct {
	RequestID string
	ActorRole string
	Reason    string
}

func (s *Service) RejectSupplyRequest(ctx context.Context, in RejectSupplyRequestInput) (*SupplyRequest, error) {
	if in.ActorRole != "SUPPLY_MANAGER" && in.ActorRole != "CEO" {
		return nil, ErrForbiddenAction
	}
	if in.Reason == "" {
		return nil, errors.New("rejection reason is required")
	}

	req, err := s.repo.GetSupplyRequestByID(ctx, in.RequestID)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, errors.New("supply request not found")
	}
	if req.Status != "PENDING" {
		return nil, errors.New("request is not pending")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.UpdateSupplyRequestStatus(ctx, tx, req.ID, "REJECTED"); err != nil {
		return nil, err
	}
	req.Status = "REJECTED"
	req.Notes = &in.Reason
	if _, err := tx.ExecContext(ctx, `UPDATE supply_requests SET notes = $2 WHERE id = $1`, req.ID, in.Reason); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

// ============================================================
// 5. Supply Requests (Ship)
// ============================================================

func (s *Service) ShipSupplyRequest(ctx context.Context, actorRole, requestID string) error {
	if actorRole != "SUPPLY_MANAGER" && actorRole != "CEO" {
		return ErrForbiddenAction
	}

	req, err := s.repo.GetSupplyRequestByID(ctx, requestID)
	if err != nil {
		return err
	}
	if req == nil {
		return errors.New("request not found")
	}
	if req.Status != "APPROVED" {
		return errors.New("request must be approved first")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Central Kitchen ID
	centralKitchenID := "99999999-9999-9999-9999-999999999999"

	for _, item := range req.Items {
		if item.ApprovedQty != nil && *item.ApprovedQty > 0 {
			inv, err := s.repo.GetInventoryByBranchAndGood(ctx, centralKitchenID, item.GoodID)
			if err != nil {
				return err
			}
			if inv == nil || inv.Quantity < *item.ApprovedQty {
				return ErrInsufficientStock
			}

			newQty := inv.Quantity - *item.ApprovedQty
			if err := s.repo.UpdateInventoryQuantity(ctx, tx, inv.ID, newQty, time.Now().UTC()); err != nil {
				return err
			}

			mov := &InventoryMovement{
				ID:          uuid.NewString(),
				InventoryID: inv.ID,
				Type:        TypeOut,
				Quantity:    *item.ApprovedQty,
				Reason:      "Supply Shipment to Branch",
				CreatedBy:   req.CreatedBy,
				CreatedAt:   time.Now().UTC(),
			}
			if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
				return err
			}
		}
	}

	if err := s.repo.UpdateSupplyRequestStatus(ctx, tx, req.ID, "SHIPPED"); err != nil {
		return err
	}
	return tx.Commit()
}

// ============================================================
// 6. Supply Requests (Receive / Partial Receive)
// ============================================================

type ReceiveSupplyRequestInput struct {
	RequestID string
	ActorRole string
	Items     []ReceiveItemInput
}

type ReceiveItemInput struct {
	ItemID      string
	ReceivedQty float64
}

func (s *Service) ReceiveSupplyRequest(ctx context.Context, in ReceiveSupplyRequestInput) error {
	if in.ActorRole != "BRANCH_MANAGER" && in.ActorRole != "CEO" {
		return ErrForbiddenAction
	}

	req, err := s.repo.GetSupplyRequestByID(ctx, in.RequestID)
	if err != nil {
		return err
	}
	if req == nil {
		return errors.New("request not found")
	}
	if req.Status != "SHIPPED" {
		return errors.New("request must be shipped first")
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for i, item := range req.Items {
		receivedQty := float64(0)
		for _, recItem := range in.Items {
			if item.ID == recItem.ItemID {
				receivedQty = recItem.ReceivedQty
			}
		}
		if receivedQty == 0 && item.ApprovedQty != nil {
			receivedQty = *item.ApprovedQty
		}

		if receivedQty > 0 {
			inv, err := s.repo.GetInventoryByBranchAndGood(ctx, req.BranchID, item.GoodID)
			if err != nil {
				return err
			}
			if inv == nil {
				inv = &Inventory{
					ID: uuid.NewString(), BranchID: req.BranchID, GoodID: item.GoodID, Quantity: 0, UpdatedAt: time.Now().UTC(),
				}
				if err := s.repo.CreateInventory(ctx, tx, inv); err != nil {
					return err
				}
			}

			newQty := inv.Quantity + receivedQty
			if err := s.repo.UpdateInventoryQuantity(ctx, tx, inv.ID, newQty, time.Now().UTC()); err != nil {
				return err
			}

			mov := &InventoryMovement{
				ID: uuid.NewString(), InventoryID: inv.ID, Type: TypeIn,
				Quantity: receivedQty, Reason: "Supply Received", CreatedBy: req.CreatedBy, CreatedAt: time.Now().UTC(),
			}
			if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
				return err
			}

			req.Items[i].ReceivedQty = &receivedQty
			if err := s.repo.UpdateSupplyRequestItem(ctx, tx, req.Items[i]); err != nil {
				return err
			}
		}
	}

	if err := s.repo.UpdateSupplyRequestStatus(ctx, tx, req.ID, "FULFILLED"); err != nil {
		return err
	}
	return tx.Commit()
}
