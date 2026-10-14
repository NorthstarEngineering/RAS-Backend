package overtime

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// Service — Business Logic
//
// Ref: FR-OT-001..008, BR-011, BR-012
// ============================================================

type Service struct {
	db    *sqlx.DB
	repo  Repository
	clock func() time.Time
}

func NewService(db *sqlx.DB, repo Repository) *Service {
	return &Service{
		db:    db,
		repo:  repo,
		clock: time.Now,
	}
}

func (s *Service) SetClock(fn func() time.Time) {
	s.clock = fn
}

// ============================================================
// 1. Create Overtime
// Ref: FR-OT-001, FR-OT-002, FR-OT-003
// ============================================================

type CreateInput struct {
	EmployeeID  string
	WorkDate    time.Time
	Hours       float64
	RatePerHour float64
	Source      Source

	// من الـ auth middleware
	ActorID       string
	ActorRole     string
	ActorBranchID string

	// الفرع (بيجي من موديول Employee)
	EmployeeBranchID string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Overtime, error) {
	// ---------- 1. Authorization ----------
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// ---------- 2. Branch Scope ----------
	branchID := in.EmployeeBranchID
	if in.ActorRole == "BRANCH_MANAGER" {
		if in.ActorBranchID == "" || branchID != in.ActorBranchID {
			return nil, ErrUnauthorizedBranch
		}
	}

	// ---------- 3. Validation ----------
	if err := s.validateCreateInput(&in); err != nil {
		return nil, err
	}

	// ---------- 4. حساب المبلغ ----------
	// Ref: FR-OT-003
	amount := round2(in.Hours * in.RatePerHour)

	// ---------- 5. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.clock().UTC()

	ot := &Overtime{
		ID:          uuid.NewString(),
		EmployeeID:  in.EmployeeID,
		BranchID:    branchID,
		WorkDate:    truncateToDate(in.WorkDate),
		Hours:       in.Hours,
		RatePerHour: in.RatePerHour,
		Amount:      amount,
		Status:      StatusPending, // Ref: FR-OT-006
		Source:      in.Source,
		CreatedBy:   in.ActorID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, tx, ot); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return ot, nil
}

// ============================================================
// 2. Get Overtime by ID
// ============================================================

type GetInput struct {
	OvertimeID    string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) Get(ctx context.Context, in GetInput) (*Overtime, error) {
	ot, err := s.repo.GetByID(ctx, in.OvertimeID)
	if err != nil {
		return nil, err
	}
	if ot == nil {
		return nil, ErrOvertimeNotFound
	}

	// Branch Scope للـ BM
	if in.ActorRole == "BRANCH_MANAGER" && ot.BranchID != in.ActorBranchID {
		return nil, ErrUnauthorizedBranch
	}

	return ot, nil
}

// ============================================================
// 3. List Overtime
// ============================================================

type ListInput struct {
	EmployeeID string
	Status     string
	FromDate   string
	ToDate     string

	ActorRole     string
	ActorBranchID string
}

func (s *Service) List(ctx context.Context, in ListInput) ([]*Overtime, error) {
	filter := ListFilter{
		EmployeeID: in.EmployeeID,
		Status:     in.Status,
		FromDate:   in.FromDate,
		ToDate:     in.ToDate,
	}

	// Branch Scope
	if in.ActorRole == "BRANCH_MANAGER" {
		filter.BranchID = in.ActorBranchID
	}
	// CEO: filter.BranchID = "" (كل الفروع)

	return s.repo.List(ctx, filter)
}

// ============================================================
// 4. Approve Overtime
// Ref: FR-OT-006, FR-OT-007, BR-011
// ============================================================

type ApproveInput struct {
	OvertimeID    string
	ActorID       string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) Approve(ctx context.Context, in ApproveInput) (*Overtime, error) {
	// ---------- 1. Authorization ----------
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// ---------- 2. جلب الـ OT ----------
	ot, err := s.repo.GetByID(ctx, in.OvertimeID)
	if err != nil {
		return nil, err
	}
	if ot == nil {
		return nil, ErrOvertimeNotFound
	}

	// ---------- 3. Branch Scope ----------
	if in.ActorRole == "BRANCH_MANAGER" && ot.BranchID != in.ActorBranchID {
		return nil, ErrUnauthorizedBranch
	}

	// ---------- 4. State Check ----------
	// Ref: FR-OT-006 — لازم يكون pending
	if ot.IsApproved() {
		return nil, ErrAlreadyApproved
	}
	if ot.IsRejected() {
		return nil, ErrCannotApproveRejected
	}

	// ---------- 5. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.clock().UTC()

	// Ref: FR-OT-007
	ot.Status = StatusApproved
	ot.ApprovedBy = &in.ActorID
	ot.ApprovedAt = &now
	ot.UpdatedAt = now

	if err := s.repo.Update(ctx, tx, ot); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return ot, nil
}

// ============================================================
// 5. Reject Overtime
// Ref: FR-OT-008
// ============================================================

type RejectInput struct {
	OvertimeID    string
	Reason        string
	ActorID       string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) Reject(ctx context.Context, in RejectInput) (*Overtime, error) {
	// ---------- 1. Authorization ----------
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// ---------- 2. Validation ----------
	if strings.TrimSpace(in.Reason) == "" {
		return nil, ErrRejectionReasonRequired
	}

	// ---------- 3. جلب الـ OT ----------
	ot, err := s.repo.GetByID(ctx, in.OvertimeID)
	if err != nil {
		return nil, err
	}
	if ot == nil {
		return nil, ErrOvertimeNotFound
	}

	// ---------- 4. Branch Scope ----------
	if in.ActorRole == "BRANCH_MANAGER" && ot.BranchID != in.ActorBranchID {
		return nil, ErrUnauthorizedBranch
	}

	// ---------- 5. State Check ----------
	if ot.IsRejected() {
		return nil, ErrAlreadyRejected
	}
	if ot.IsApproved() {
		return nil, ErrCannotRejectApproved
	}

	// ---------- 6. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.clock().UTC()

	// Ref: FR-OT-008
	ot.Status = StatusRejected
	ot.RejectedBy = &in.ActorID
	ot.RejectedAt = &now
	ot.RejectionReason = &in.Reason
	ot.UpdatedAt = now

	if err := s.repo.Update(ctx, tx, ot); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return ot, nil
}

// ============================================================
// Validation
// ============================================================

func (s *Service) validateCreateInput(in *CreateInput) error {
	if in.EmployeeID == "" {
		return ErrEmployeeRequired
	}

	if in.WorkDate.IsZero() {
		return ErrWorkDateRequired
	}
	// Ref: work_date لازم في الماضي أو النهاردة
	if in.WorkDate.After(s.clock().UTC()) {
		return ErrWorkDateFuture
	}

	if in.Hours <= 0 {
		return ErrInvalidHours
	}

	if in.RatePerHour <= 0 {
		return ErrInvalidRate
	}

	if !in.Source.IsValid() {
		return ErrInvalidSource
	}

	return nil
}

// ============================================================
// Helpers
// ============================================================

func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
