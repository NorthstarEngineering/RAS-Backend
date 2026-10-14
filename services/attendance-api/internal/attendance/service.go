package attendance

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ============================================================
// Service — Business Logic
//
// المسؤوليات:
//   1. تسجيل الحضور (Check-in)
//   2. تسجيل الانصراف (Check-out)
//   3. حساب التأخير والخصم تلقائيًا
//   4. تعديل/إلغاء الخصم بمعرفة الإدارة
//   5. تطبيق Branch Scope على BM
// ============================================================

type Service struct {
	db        *sqlx.DB
	repo      Repository
	employees EmployeeReader
	branches  BranchReader
	clock     func() time.Time // injectable للاختبار
}

// NewService — constructor
func NewService(
	db *sqlx.DB,
	repo Repository,
	employees EmployeeReader,
	branches BranchReader,
) *Service {
	return &Service{
		db:        db,
		repo:      repo,
		employees: employees,
		branches:  branches,
		clock:     time.Now, // افتراضي
	}
}

// SetClock — للاختبار فقط
func (s *Service) SetClock(fn func() time.Time) {
	s.clock = fn
}

// ============================================================
// 1. Check-In
// Ref: FR-ATT-001, FR-ATT-003, FR-ATT-004, FR-DED-001..005
// ============================================================

type CheckInInput struct {
	EmployeeID    string
	ActorID       string // المستخدم اللي بيعمل العملية (من الـ auth middleware)
	ActorRole     string
	ActorBranchID string
	Source        EventSource
}

type CheckInResult struct {
	Record    *AttendanceRecord `json:"record"`
	Deduction *Deduction        `json:"deduction,omitempty"`
}

func (s *Service) CheckIn(ctx context.Context, in CheckInInput) (*CheckInResult, error) {
	// ---------- 1. Validation ----------
	if in.EmployeeID == "" {
		return nil, ErrEmployeeNotFound
	}
	if in.Source != SourceFingerprint && in.Source != SourceManual {
		return nil, fmt.Errorf("invalid source: %s", in.Source)
	}

	// ---------- 2. جيب الموظف ----------
	emp, err := s.employees.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}
	if !emp.Active {
		return nil, ErrEmployeeInactive
	}
	if err := authorizeEmployeeBranch(in.ActorRole, in.ActorBranchID, emp.BranchID); err != nil {
		return nil, err
	}

	// ---------- 3. جيب الفرع ----------
	br, err := s.branches.GetByID(ctx, emp.BranchID)
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}
	if br == nil {
		return nil, ErrBranchNotFound
	}
	if br.IsCentralKitchen {
		// Ref: FR-BRANCH-003 — Central Kitchen = stock only، مفيش حضور
		return nil, ErrCentralKitchenNotAllowed
	}

	// ---------- 4. وقت السيرفر (مش وقت الموبايل) ----------
	now := s.clock().UTC()
	workDate := truncateToDate(now)

	// ---------- 5. منع التكرار ----------
	// 5a. فيه سجل مفتوح؟
	open, err := s.repo.GetOpenRecord(ctx, in.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("check open record: %w", err)
	}
	if open != nil {
		return nil, ErrAlreadyCheckedIn
	}

	// 5b. فيه سجل لنفس اليوم (حتى لو مقفول)؟
	today, err := s.repo.GetRecordByDate(ctx, in.EmployeeID, workDate)
	if err != nil {
		return nil, fmt.Errorf("check today record: %w", err)
	}
	if today != nil {
		return nil, ErrAlreadyCheckedIn
	}

	// ---------- 6. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	record := &AttendanceRecord{
		ID:         uuid.NewString(),
		EmployeeID: in.EmployeeID,
		BranchID:   emp.BranchID,
		WorkDate:   workDate,
		CheckInAt:  now,
		Status:     StatusOpen,
		Source:     in.Source,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.CreateRecord(ctx, tx, record); err != nil {
		return nil, err
	}

	// ---------- 7. حساب التأخير والخصم ----------
	deduction, err := s.calculateDeduction(ctx, tx, record, br)
	if err != nil {
		return nil, err
	}
	record.Deduction = deduction

	// ---------- 8. Commit ----------
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &CheckInResult{Record: record, Deduction: deduction}, nil
}

// ============================================================
// 2. Check-Out
// Ref: FR-ATT-002
// ============================================================

type CheckOutInput struct {
	EmployeeID    string
	ActorID       string
	ActorRole     string
	ActorBranchID string
}

func (s *Service) CheckOut(ctx context.Context, in CheckOutInput) (*AttendanceRecord, error) {
	if in.EmployeeID == "" {
		return nil, ErrEmployeeNotFound
	}

	// Verify employee scope before reading or changing attendance records.
	emp, err := s.employees.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	if emp == nil {
		// Preserve the existing check-out response; a valid open record also
		// implies an employee row by foreign key.
		return nil, ErrNotCheckedIn
	}
	if err := authorizeEmployeeBranch(in.ActorRole, in.ActorBranchID, emp.BranchID); err != nil {
		return nil, err
	}

	// جيب السجل المفتوح
	open, err := s.repo.GetOpenRecord(ctx, in.EmployeeID)
	if err != nil {
		return nil, fmt.Errorf("get open record: %w", err)
	}
	if open == nil {
		return nil, ErrNotCheckedIn
	}

	now := s.clock().UTC()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.UpdateCheckOut(ctx, tx, open.ID, now); err != nil {
		return nil, err
	}

	// ملاحظة: مش بنحسب خصم تاني هنا
	// الخصم بيتم مرة واحدة وقت الـ check-in

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	open.CheckOutAt = &now
	open.Status = StatusClosed
	open.UpdatedAt = now

	return open, nil
}

func authorizeEmployeeBranch(role, actorBranchID, employeeBranchID string) error {
	switch role {
	case "CEO":
		return nil
	case "BRANCH_MANAGER":
		if actorBranchID != "" && actorBranchID == employeeBranchID {
			return nil
		}
		return ErrUnauthorizedBranch
	default:
		return ErrForbiddenAction
	}
}

// ============================================================
// 3. حساب التأخير والخصم (القاعدة الأساسية)
// Ref: FR-DED-001, FR-DED-002, FR-DED-003, FR-DED-004
// ============================================================

func (s *Service) calculateDeduction(
	ctx context.Context,
	tx *sqlx.Tx,
	record *AttendanceRecord,
	br *BranchInfo,
) (*Deduction, error) {

	// حوّل "HH:MM" لتاريخ كامل بنفس يوم الحضور
	officialStart, err := parseHHMM(record.CheckInAt, br.OfficialStartTime)
	if err != nil {
		return nil, fmt.Errorf("parse official start time: %w", err)
	}

	// ---------- لو حضر في نفس الوقت أو قبله → مفيش خصم ----------
	if !record.CheckInAt.After(officialStart) {
		return nil, nil
	}

	// ---------- احسب الدقائق المتأخرة ----------
	lateMinutes := int(record.CheckInAt.Sub(officialStart).Minutes())

	// ---------- FR-DED-003: الساعات المكتملة فقط ----------
	completedHours := lateMinutes / 60

	// ---------- لو أقل من ساعة → مفيش خصم ----------
	if completedHours <= 0 {
		return nil, nil
	}

	// ---------- احسب المبلغ ----------
	amount := round2(float64(completedHours) * br.LatenessRatePerHour)

	d := &Deduction{
		ID:             uuid.NewString(),
		AttendanceID:   record.ID,
		EmployeeID:     record.EmployeeID,
		BranchID:       record.BranchID,
		WorkDate:       record.WorkDate,
		LateMinutes:    lateMinutes,
		CompletedHours: completedHours,
		RatePerHour:    br.LatenessRatePerHour,
		OriginalAmount: amount,
		Source:         "automatic", // FR-DED-004
		CreatedAt:      s.clock().UTC(),
	}

	if err := s.repo.CreateDeduction(ctx, tx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// ============================================================
// 4. تعديل أو إلغاء الخصم
// Ref: FR-DED-006, FR-DED-007, FR-DED-008, FR-DED-009
// Ref: FR-AUTH-004, NFR-SEC-003 (Branch Scope)
// ============================================================

type AdjustDeductionInput struct {
	DeductionID   string
	ActorID       string
	ActorRole     string   // "CEO" | "BRANCH_MANAGER"
	ActorBranchID string   // للـ BM — فرعه هو
	NewAmount     *float64 // لو nil → مفيش تعديل في القيمة
	Cancel        bool
	Reason        string
}

func (s *Service) AdjustDeduction(ctx context.Context, in AdjustDeductionInput) (*Deduction, error) {
	// ---------- 1. التحقق من الصلاحية ----------
	if in.ActorRole != "CEO" && in.ActorRole != "BRANCH_MANAGER" {
		return nil, ErrForbiddenAction
	}

	// ---------- 2. التحقق من المدخلات ----------
	if in.Reason == "" {
		// Ref: FR-DED-009, BR-010
		return nil, ErrReasonRequired
	}
	if in.NewAmount != nil && *in.NewAmount < 0 {
		return nil, ErrInvalidAmount
	}
	if !in.Cancel && in.NewAmount == nil {
		return nil, ErrNoAdjustmentSpecified
	}

	// ---------- 3. جيب الخصم ----------
	d, err := s.repo.GetDeductionByID(ctx, in.DeductionID)
	if err != nil {
		return nil, fmt.Errorf("get deduction: %w", err)
	}
	if d == nil {
		return nil, ErrDeductionNotFound
	}
	if d.Cancelled {
		return nil, ErrAlreadyCancelled
	}

	// ---------- 4. Branch Scope Check (BM فقط) ----------
	// Ref: FR-AUTH-004, NFR-SEC-003
	if in.ActorRole == "BRANCH_MANAGER" {
		if in.ActorBranchID == "" || d.BranchID != in.ActorBranchID {
			return nil, ErrUnauthorizedBranch
		}
	}

	// ---------- 5. Transaction ----------
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.clock().UTC()

	if in.Cancel {
		// ---------- إلغاء ----------
		// Ref: FR-DED-008, FR-DED-009
		d.Cancelled = true
		d.CancelledBy = &in.ActorID
		d.CancelledAt = &now
		d.CancelReason = &in.Reason
	} else {
		// ---------- تعديل ----------
		// Ref: FR-DED-006, FR-DED-007, FR-DED-009
		d.AdjustedAmount = in.NewAmount
		d.AdjustmentReason = &in.Reason
		d.AdjustedBy = &in.ActorID
		d.AdjustedAt = &now
	}

	if err := s.repo.UpdateDeduction(ctx, tx, d); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return d, nil
}

// ============================================================
// Helpers
// ============================================================

// parseHHMM — يحوّل "HH:MM" لتاريخ كامل في نفس يوم base
// مثال: base=2026-10-10 08:00 UTC، hhmm="09:00" → 2026-10-10 09:00 UTC
func parseHHMM(base time.Time, hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format %q: %w", hhmm, err)
	}
	y, m, d := base.Date()
	return time.Date(y, m, d, t.Hour(), t.Minute(), 0, 0, base.Location()), nil
}

// truncateToDate — يرجّع منتصف الليل لنفس اليوم
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// round2 — يقرب لـ 2 رقم عشري
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
