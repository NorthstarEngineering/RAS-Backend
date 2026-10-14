package attendance

import "time"

// ============================================================
// AttendanceRecord — سجل حضور يومي كامل لموظف
// Ref: FR-ATT-001, FR-ATT-002
// ============================================================

type AttendanceRecord struct {
	ID         string    `db:"id" json:"id"`
	EmployeeID string    `db:"employee_id" json:"employee_id"`
	BranchID   string    `db:"branch_id" json:"branch_id"`
	WorkDate   time.Time `db:"work_date" json:"work_date"`

	// Check-in إجباري، Check-out اختياري
	CheckInAt  time.Time  `db:"check_in_at" json:"check_in_at"`
	CheckOutAt *time.Time `db:"check_out_at,omitempty" json:"check_out_at,omitempty"`

	Status RecordStatus `db:"status" json:"status"`
	Source EventSource  `db:"source" json:"source"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`

	// علاقة اختيارية مع الخصم (تُملأ في الـ service قبل الرجوع للـ API)
	Deduction *Deduction `db:"-" json:"deduction,omitempty"`
}

// RecordStatus — حالة سجل الحضور
// open: الموظف لسه في الشغل
// closed: الموظف سجّل انصراف
type RecordStatus string

const (
	StatusOpen   RecordStatus = "open"
	StatusClosed RecordStatus = "closed"
)

// EventSource — مصدر تسجيل الحضور
// fingerprint: من جهاز البصمة
// manual: تم إدخاله يدويًا بواسطة BM/CEO
type EventSource string

const (
	SourceFingerprint EventSource = "fingerprint"
	SourceManual      EventSource = "manual"
)

// ============================================================
// Deduction — خصم التأخير
// Ref: FR-DED-001..009
// ============================================================

type Deduction struct {
	ID           string    `db:"id" json:"id"`
	AttendanceID string    `db:"attendance_id" json:"attendance_id"`
	EmployeeID   string    `db:"employee_id" json:"employee_id"`
	BranchID     string    `db:"branch_id" json:"branch_id"`
	WorkDate     time.Time `db:"work_date" json:"work_date"`

	// ---------- الحساب التلقائي (immutable بعد الإنشاء) ----------
	// Ref: FR-DED-001, FR-DED-002, FR-DED-003
	LateMinutes    int     `db:"late_minutes" json:"late_minutes"`
	CompletedHours int     `db:"completed_hours" json:"completed_hours"`
	RatePerHour    float64 `db:"rate_per_hour" json:"rate_per_hour"`
	OriginalAmount float64 `db:"original_amount" json:"original_amount"`

	// ---------- التعديل اليدوي (اختياري) ----------
	// Ref: FR-DED-006, FR-DED-007, FR-DED-009
	AdjustedAmount   *float64   `db:"adjusted_amount,omitempty" json:"adjusted_amount,omitempty"`
	AdjustmentReason *string    `db:"adjustment_reason,omitempty" json:"adjustment_reason,omitempty"`
	AdjustedBy       *string    `db:"adjusted_by,omitempty" json:"adjusted_by,omitempty"`
	AdjustedAt       *time.Time `db:"adjusted_at,omitempty" json:"adjusted_at,omitempty"`

	// ---------- الإلغاء (اختياري) ----------
	// Ref: FR-DED-008, FR-DED-009
	Cancelled    bool       `db:"cancelled" json:"cancelled"`
	CancelledBy  *string    `db:"cancelled_by,omitempty" json:"cancelled_by,omitempty"`
	CancelledAt  *time.Time `db:"cancelled_at,omitempty" json:"cancelled_at,omitempty"`
	CancelReason *string    `db:"cancel_reason,omitempty" json:"cancel_reason,omitempty"`

	// ---------- مصدر الخصم ----------
	// automatic: نتج عن حساب التأخير التلقائي (FR-DED-004: لا يحتاج موافقة)
	Source string `db:"source" json:"source"`

	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// EffectiveAmount — القيمة الفعلية للخصم بعد تطبيق التعديلات أو الإلغاء
// Ref: FR-DED-006, FR-DED-007, FR-DED-008
//
// القاعدة:
//  1. لو الخصم ملغي → 0
//  2. لو فيه تعديل → القيمة المعدلة
//  3. غير كده → القيمة الأصلية
func (d *Deduction) EffectiveAmount() float64 {
	if d.Cancelled {
		return 0
	}
	if d.AdjustedAmount != nil {
		return *d.AdjustedAmount
	}
	return d.OriginalAmount
}

// IsAdjusted — هل الخصم اتعدّل يدويًا؟
func (d *Deduction) IsAdjusted() bool {
	return d.AdjustedAmount != nil
}

// IsAutomatic — هل الخصم تلقائي (FR-DED-004)؟
func (d *Deduction) IsAutomatic() bool {
	return d.Source == "automatic"
}
