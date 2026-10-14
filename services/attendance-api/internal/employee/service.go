package employee

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// Regex patterns — للـ validation
// ============================================================

var (
	rePhone      = regexp.MustCompile(`^[0-9]{10,15}$`)
	reNationalID = regexp.MustCompile(`^[0-9]{14}$`)
	reEmail      = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// ============================================================
// Service
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

// SetClock — للاختبار
func (s *Service) SetClock(fn func() time.Time) {
	s.clock = fn
}

// ============================================================
// 1. Create Employee
// Ref: FR-EMP-001, FR-EMP-002, FR-EMP-003
// BR-002: CEO يختار الفرع
// BR-003: BM-created → branch inherited
// ============================================================

type CreateInput struct {
	// من الـ request
	Name          string
	Phone         string
	NationalID    string
	BranchID      string
	PaymentType   PaymentType
	PaymentAmount float64
	HireDate      time.Time
	JobTitle      *string
	Email         *string
	Address       *string

	// من الـ auth middleware
	ActorID       string
	ActorRole     string // "CEO" | "BRANCH_MANAGER"
	ActorBranchID string // فرع الـ BM
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Employee, error) {
	// ---------- 1. Authorization ----------
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// BR-003: BM بياخد فرعه تلقائيًا
	if in.ActorRole == "BRANCH_MANAGER" {
		if in.ActorBranchID == "" {
			return nil, ErrBranchRequired
		}
		in.BranchID = in.ActorBranchID
	}

	// ---------- 2. Validation ----------
	if err := s.validateCreateInput(&in); err != nil {
		return nil, err
	}

	// ---------- 3. فحص تكرار الرقم القومي ----------
	exists, err := s.repo.ExistsNationalID(ctx, in.NationalID, "")
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrNationalIDDuplicate
	}

	// ---------- 4. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.clock().UTC()

	emp := &Employee{
		ID:              uuid.NewString(),
		Name:            strings.TrimSpace(in.Name),
		BranchID:        in.BranchID,
		Phone:           in.Phone,
		NationalID:      in.NationalID,
		JobTitle:        in.JobTitle,
		Email:           in.Email,
		Address:         in.Address,
		PaymentType:     in.PaymentType,
		PaymentAmount:   in.PaymentAmount,
		HireDate:        in.HireDate,
		TerminationDate: nil,
		Active:          true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.repo.Create(ctx, tx, emp); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return emp, nil
}

// ============================================================
// 2. Get Employee by ID
// ============================================================

type GetInput struct {
	EmployeeID    string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) Get(ctx context.Context, in GetInput) (*Employee, error) {
	emp, err := s.repo.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}

	// BM: لازم الموظف يكون في فرعه
	if in.ActorRole == "BRANCH_MANAGER" {
		if emp.BranchID != in.ActorBranchID {
			return nil, ErrUnauthorizedBranch
		}
	}

	return emp, nil
}

// ============================================================
// 3. List Employees
// ============================================================

type ListInput struct {
	BranchID      string // "" للـ CEO
	ActiveOnly    bool
	ActorRole     string
	ActorBranchID string
}

func (s *Service) List(ctx context.Context, in ListInput) ([]*Employee, error) {
	// BM: يفرض فرعه
	if in.ActorRole == "BRANCH_MANAGER" {
		in.BranchID = in.ActorBranchID
	}

	if in.ActiveOnly {
		return s.repo.ListActiveByBranch(ctx, in.BranchID)
	}
	return s.repo.ListByBranch(ctx, in.BranchID)
}

// ============================================================
// 4. Update Employee (full update)
// Ref: FR-EMP-006 (change payment)
// ============================================================

type UpdateInput struct {
	EmployeeID string

	// الحقول القابلة للتعديل
	Name          *string
	Phone         *string
	JobTitle      *string
	Email         *string
	Address       *string
	PaymentType   *PaymentType
	PaymentAmount *float64

	// Actor
	ActorID       string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*Employee, error) {
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// جيب الموظف
	emp, err := s.repo.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}

	// Branch scope
	if in.ActorRole == "BRANCH_MANAGER" && emp.BranchID != in.ActorBranchID {
		return nil, ErrUnauthorizedBranch
	}

	// ---------- تطبيق التعديلات ----------
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, ErrNameRequired
		}
		emp.Name = name
	}
	if in.Phone != nil {
		if !rePhone.MatchString(*in.Phone) {
			return nil, ErrPhoneInvalid
		}
		emp.Phone = *in.Phone
	}
	if in.JobTitle != nil {
		emp.JobTitle = in.JobTitle
	}
	if in.Email != nil {
		if *in.Email != "" && !reEmail.MatchString(*in.Email) {
			return nil, ErrInvalidEmail
		}
		emp.Email = in.Email
	}
	if in.Address != nil {
		emp.Address = in.Address
	}
	if in.PaymentType != nil {
		if !in.PaymentType.IsValid() {
			return nil, ErrInvalidPaymentType
		}
		emp.PaymentType = *in.PaymentType
	}
	if in.PaymentAmount != nil {
		if *in.PaymentAmount <= 0 {
			return nil, ErrInvalidPaymentAmount
		}
		emp.PaymentAmount = *in.PaymentAmount
	}

	emp.UpdatedAt = s.clock().UTC()

	// ---------- Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.Update(ctx, tx, emp); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return emp, nil
}

// ============================================================
// 5. Change Payment (convenience — FR-EMP-006)
// ============================================================

type ChangePaymentInput struct {
	EmployeeID    string
	PaymentType   PaymentType
	PaymentAmount float64
	ActorID       string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) ChangePayment(ctx context.Context, in ChangePaymentInput) (*Employee, error) {
	if !in.PaymentType.IsValid() {
		return nil, ErrInvalidPaymentType
	}
	if in.PaymentAmount <= 0 {
		return nil, ErrInvalidPaymentAmount
	}

	return s.Update(ctx, UpdateInput{
		EmployeeID:    in.EmployeeID,
		PaymentType:   &in.PaymentType,
		PaymentAmount: &in.PaymentAmount,
		ActorID:       in.ActorID,
		ActorRole:     in.ActorRole,
		ActorBranchID: in.ActorBranchID,
	})
}

// ============================================================
// 6. Terminate Employee (deactivate)
// ============================================================

type TerminateInput struct {
	EmployeeID      string
	TerminationDate time.Time
	ActorID         string
	ActorRole       string
	ActorBranchID   string
}

func (s *Service) Terminate(ctx context.Context, in TerminateInput) (*Employee, error) {
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	emp, err := s.repo.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, err
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}

	if in.ActorRole == "BRANCH_MANAGER" && emp.BranchID != in.ActorBranchID {
		return nil, ErrUnauthorizedBranch
	}

	if !emp.Active {
		return nil, ErrAlreadyTerminated
	}

	// termination_date لازم يكون بعد hire_date
	terminationDate := in.TerminationDate.UTC()
	if terminationDate.Before(emp.HireDate) {
		return nil, ErrInvalidTerminationDate
	}

	emp.Active = false
	emp.TerminationDate = &terminationDate
	emp.UpdatedAt = s.clock().UTC()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.Update(ctx, tx, emp); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return emp, nil
}

// ============================================================
// Validation Helpers
// ============================================================

func (s *Service) validateCreateInput(in *CreateInput) error {
	// Name
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return ErrNameRequired
	}

	// Branch
	if in.BranchID == "" {
		return ErrBranchRequired
	}

	// Phone
	if in.Phone == "" {
		return ErrPhoneRequired
	}
	if !rePhone.MatchString(in.Phone) {
		return ErrPhoneInvalid
	}

	// National ID
	if in.NationalID == "" {
		return ErrNationalIDRequired
	}
	if !reNationalID.MatchString(in.NationalID) {
		return ErrNationalIDInvalid
	}

	// Payment Type
	if !in.PaymentType.IsValid() {
		return ErrInvalidPaymentType
	}

	// Payment Amount
	if in.PaymentAmount <= 0 {
		return ErrInvalidPaymentAmount
	}

	// Hire Date
	if in.HireDate.IsZero() {
		return ErrInvalidHireDate
	}
	// Hire date لازم يكون في الماضي أو النهاردة
	if in.HireDate.After(s.clock().UTC()) {
		return ErrInvalidHireDate
	}

	// Email (اختياري — لو موجود، لازم يكون صالح)
	if in.Email != nil && *in.Email != "" {
		if !reEmail.MatchString(*in.Email) {
			return ErrInvalidEmail
		}
	}

	return nil
}
