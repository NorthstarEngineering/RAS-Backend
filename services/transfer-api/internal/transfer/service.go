package transfer

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Authorization model:
//   - CEO: may act on any transfer.
//   - BRANCH_MANAGER of the SOURCE branch: approve, reject, send.
//   - BRANCH_MANAGER of the DESTINATION branch: receive.
//   - Either branch manager (or CEO): create and view transfers involving their branch.
type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB, repo *Repository) *Service {
	return &Service{db: db, repo: repo}
}

func (s *Service) inTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) detail(ctx context.Context, q sqlx.ExtContext, id uuid.UUID) (*TransferDetail, error) {
	view, err := s.repo.GetView(ctx, q, id)
	if err != nil {
		return nil, err
	}
	events, err := s.repo.Events(ctx, q, id)
	if err != nil {
		return nil, err
	}
	return &TransferDetail{TransferView: *view, Timeline: events}, nil
}

// Create - request a new transfer (status PENDING).
func (s *Service) Create(ctx context.Context, id Identity, req CreateTransferRequest) (*TransferDetail, error) {
	if req.SourceBranchID == uuid.Nil {
		req.SourceBranchID = id.BranchID
	}
	if req.SourceBranchID == uuid.Nil {
		return nil, fmt.Errorf("%w: source_branch_id is required", ErrInvalidRequest)
	}
	if req.SourceBranchID == req.DestBranchID {
		return nil, ErrSameBranch
	}
	if !id.ManagesBranch(req.SourceBranchID) && !id.ManagesBranch(req.DestBranchID) {
		return nil, ErrForbidden
	}

	qty := round2(req.Quantity)
	if qty <= 0 {
		return nil, ErrInvalidQuantity
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: reason is required", ErrReasonRequired)
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityNormal
	}

	t := &Transfer{
		ID:             uuid.New(),
		SourceBranchID: req.SourceBranchID,
		DestBranchID:   req.DestBranchID,
		GoodID:         req.GoodID,
		RequestedQty:   qty,
		Priority:       priority,
		Reason:         reason,
		RequestedBy:    id.UserID,
	}

	var out *TransferDetail
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		for _, b := range []uuid.UUID{t.SourceBranchID, t.DestBranchID} {
			ok, err := s.repo.BranchActive(ctx, tx, b)
			if err != nil {
				return err
			}
			if !ok {
				return ErrBranchNotFound
			}
		}
		ok, err := s.repo.GoodExists(ctx, tx, t.GoodID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrGoodNotFound
		}
		if err := s.repo.Insert(ctx, tx, t); err != nil {
			return err
		}
		if err := s.repo.InsertEvent(ctx, tx, t.ID, ActionRequested, id.UserID, nullable(reason)); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, t.ID)
		return err
	})
	return out, err
}

// Approve - PENDING -> APPROVED. Inventory is NOT touched here.
func (s *Service) Approve(ctx context.Context, id Identity, transferID uuid.UUID, req ApproveRequest) (*TransferDetail, error) {
	var out *TransferDetail
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		t, err := s.repo.GetForUpdate(ctx, tx, transferID)
		if err != nil {
			return err
		}
		if !id.ManagesBranch(t.SourceBranchID) {
			return ErrForbidden
		}
		if t.Status != StatusPending {
			return ErrInvalidState
		}

		approved := t.RequestedQty
		if req.ApprovedQuantity != nil {
			approved = round2(*req.ApprovedQuantity)
		}
		if approved <= 0 {
			return ErrInvalidQuantity
		}
		if approved > t.RequestedQty {
			return ErrApprovedExceedsRequested
		}
		note := strings.TrimSpace(req.Note)
		if approved < t.RequestedQty && note == "" {
			return fmt.Errorf("%w: a note is required when approving a lower quantity", ErrReasonRequired)
		}

		if err := s.repo.MarkApproved(ctx, tx, t.ID, approved, nullable(note), id.UserID); err != nil {
			return err
		}
		if err := s.repo.InsertEvent(ctx, tx, t.ID, ActionApproved, id.UserID, nullable(note)); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, t.ID)
		return err
	})
	return out, err
}

// Reject - PENDING -> REJECTED. A reason is mandatory; the transfer can never be sent.
func (s *Service) Reject(ctx context.Context, id Identity, transferID uuid.UUID, req RejectRequest) (*TransferDetail, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, fmt.Errorf("%w: a rejection reason is required", ErrReasonRequired)
	}

	var out *TransferDetail
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		t, err := s.repo.GetForUpdate(ctx, tx, transferID)
		if err != nil {
			return err
		}
		if !id.ManagesBranch(t.SourceBranchID) {
			return ErrForbidden
		}
		if t.Status != StatusPending {
			return ErrInvalidState
		}
		if err := s.repo.MarkRejected(ctx, tx, t.ID, reason, id.UserID); err != nil {
			return err
		}
		if err := s.repo.InsertEvent(ctx, tx, t.ID, ActionRejected, id.UserID, &reason); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, t.ID)
		return err
	})
	return out, err
}

// Send - APPROVED -> SENT. Decreases source inventory by the approved quantity.
func (s *Service) Send(ctx context.Context, id Identity, transferID uuid.UUID, req SendRequest) (*TransferDetail, error) {
	var out *TransferDetail
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		t, err := s.repo.GetForUpdate(ctx, tx, transferID)
		if err != nil {
			return err
		}
		if !id.ManagesBranch(t.SourceBranchID) {
			return ErrForbidden
		}
		if t.Status != StatusApproved || t.ApprovedQty == nil {
			return ErrInvalidState
		}
		qty := *t.ApprovedQty

		inv, err := s.repo.LockInventory(ctx, tx, t.SourceBranchID, t.GoodID)
		if err != nil {
			return err
		}
		if inv == nil || inv.Quantity < qty {
			return ErrInsufficientStock
		}
		after := round2(inv.Quantity - qty)
		cost := inv.UnitCost
		dest := t.DestBranchID
		if err := s.repo.DecreaseInventory(ctx, tx, inv.ID, qty); err != nil {
			return err
		}
		if err := s.repo.InsertMovement(ctx, tx, Movement{
			InventoryID: inv.ID, BranchID: t.SourceBranchID, GoodID: t.GoodID,
			Type: "OUT", Quantity: qty, Before: inv.Quantity, After: after,
			UnitCost: &cost, DestBranchID: &dest,
			Reason: "Stock transfer " + t.Code + " sent", UserID: id.UserID,
		}); err != nil {
			return err
		}
		if err := s.repo.MarkSent(ctx, tx, t.ID, qty, cost,
			nullable(req.VehicleType), nullable(req.DriverName), nullable(req.DriverPhone),
			req.ExpectedDeparture, req.TemperatureC, id.UserID); err != nil {
			return err
		}
		if err := s.repo.InsertEvent(ctx, tx, t.ID, ActionSent, id.UserID, nil); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, t.ID)
		return err
	})
	return out, err
}

// Receive - SENT -> RECEIVED. Increases destination inventory by the received quantity.
// received < sent is allowed (shortage/damage) but requires a note.
func (s *Service) Receive(ctx context.Context, id Identity, transferID uuid.UUID, req ReceiveRequest) (*TransferDetail, error) {
	if req.ReceivedQuantity == nil {
		return nil, fmt.Errorf("%w: received_quantity is required", ErrInvalidRequest)
	}
	received := round2(*req.ReceivedQuantity)
	if received < 0 {
		return nil, ErrInvalidQuantity
	}
	note := strings.TrimSpace(req.Note)

	var out *TransferDetail
	err := s.inTx(ctx, func(tx *sqlx.Tx) error {
		t, err := s.repo.GetForUpdate(ctx, tx, transferID)
		if err != nil {
			return err
		}
		if !id.ManagesBranch(t.DestBranchID) {
			return ErrForbidden
		}
		if t.Status != StatusSent || t.SentQty == nil {
			return ErrInvalidState
		}
		sent := *t.SentQty
		if received > sent {
			return ErrReceivedExceedsSent
		}
		shortage := round2(sent - received)
		if shortage > 0 && note == "" {
			return fmt.Errorf("%w: a note is required when the received quantity differs from the sent quantity", ErrReasonRequired)
		}

		if received > 0 {
			unitCost := 0.0
			if t.UnitCost != nil {
				unitCost = *t.UnitCost
			}
			if err := s.repo.EnsureInventory(ctx, tx, t.DestBranchID, t.GoodID, unitCost); err != nil {
				return err
			}
			inv, err := s.repo.LockInventory(ctx, tx, t.DestBranchID, t.GoodID)
			if err != nil {
				return err
			}
			if inv == nil {
				return fmt.Errorf("destination inventory row missing for transfer %s", t.Code)
			}
			after := round2(inv.Quantity + received)
			newCost := unitCost
			if inv.Quantity > 0 { // weighted-average cost
				newCost = round2((inv.Quantity*inv.UnitCost + received*unitCost) / after)
			}
			if err := s.repo.SetInventory(ctx, tx, inv.ID, after, newCost); err != nil {
				return err
			}
			if err := s.repo.InsertMovement(ctx, tx, Movement{
				InventoryID: inv.ID, BranchID: t.DestBranchID, GoodID: t.GoodID,
				Type: "IN", Quantity: received, Before: inv.Quantity, After: after,
				UnitCost: &unitCost,
				Reason:   "Stock transfer " + t.Code + " received", UserID: id.UserID,
			}); err != nil {
				return err
			}
		}
		if err := s.repo.MarkReceived(ctx, tx, t.ID, received, shortage, nullable(note), id.UserID); err != nil {
			return err
		}
		if err := s.repo.InsertEvent(ctx, tx, t.ID, ActionReceived, id.UserID, nullable(note)); err != nil {
			return err
		}
		out, err = s.detail(ctx, tx, t.ID)
		return err
	})
	return out, err
}

// Get returns one transfer with its full timeline (request -> approve -> send -> receive).
func (s *Service) Get(ctx context.Context, id Identity, transferID uuid.UUID) (*TransferDetail, error) {
	d, err := s.detail(ctx, s.db, transferID)
	if err != nil {
		return nil, err
	}
	if !id.ManagesBranch(d.SourceBranchID) && !id.ManagesBranch(d.DestBranchID) {
		return nil, ErrForbidden
	}
	return d, nil
}

func (s *Service) List(ctx context.Context, id Identity, f ListFilter) ([]TransferView, error) {
	applyScope(id, &f)
	return s.repo.List(ctx, s.db, f)
}

func (s *Service) Stats(ctx context.Context, id Identity, f ListFilter) (*Stats, error) {
	applyScope(id, &f)
	st, err := s.repo.Stats(ctx, s.db, f)
	if err != nil {
		return nil, err
	}
	if st.Received > 0 {
		matched := float64(st.Received - st.ShortageCases)
		st.MatchRate = math.Round(matched/float64(st.Received)*1000) / 10
	}
	return st, nil
}

// applyScope forces branch managers to see only transfers that involve their own branch.
func applyScope(id Identity, f *ListFilter) {
	if id.Role == RoleBranchManager {
		b := id.BranchID
		f.BranchID = &b
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func nullable(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}