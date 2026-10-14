package overtime

import "time"

// ============================================================
// Overtime — سجل الساعات الإضافية
// Ref: FR-OT-001..008, BR-011, BR-012
// ============================================================

type Overtime struct {
	ID         string    `db:"id" json:"id"`
	EmployeeID string    `db:"employee_id" json:"employee_id"`
	BranchID   string    `db:"branch_id" json:"branch_id"`
	WorkDate   time.Time `db:"work_date" json:"work_date"`

	// ---------- الساعات والمبلغ ----------
	// Ref: FR-OT-002, FR-OT-003
	Hours       float64 `db:"hours" json:"hours"`
	RatePerHour float64 `db:"rate_per_hour" json:"rate_per_hour"`
	Amount      float64 `db:"amount" json:"amount"` // hours × rate

	// ---------- الحالة ----------
	Status Status `db:"status" json:"status"`
	Source Source `db:"source" json:"source"`

	// ---------- الموافقة ----------
	// Ref: FR-OT-007
	ApprovedBy *string    `db:"approved_by" json:"approved_by,omitempty"`
	ApprovedAt *time.Time `db:"approved_at" json:"approved_at,omitempty"`

	// ---------- الرفض ----------
	// Ref: FR-OT-008
	RejectedBy      *string    `db:"rejected_by" json:"rejected_by,omitempty"`
	RejectedAt      *time.Time `db:"rejected_at" json:"rejected_at,omitempty"`
	RejectionReason *string    `db:"rejection_reason" json:"rejection_reason,omitempty"`

	// ---------- الإنشاء ----------
	CreatedBy string    `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================
// Status — حالة الـ OT
// Ref: BR-011, BR-012
// ============================================================

type Status string

const (
	// StatusPending — في انتظار الموافقة
	// Ref: FR-OT-006, BR-012
	StatusPending Status = "pending"

	// StatusApproved — مقبول (payable)
	// Ref: FR-OT-007, BR-011
	StatusApproved Status = "approved"

	// StatusRejected — مرفوض
	// Ref: FR-OT-008
	StatusRejected Status = "rejected"
)

func (s Status) IsValid() bool {
	return s == StatusPending || s == StatusApproved || s == StatusRejected
}

// ============================================================
// Source — مصدر الـ OT
// ============================================================

type Source string

const (
	// SourceManual — إدخال يدوي
	// Ref: FR-OT-001
	SourceManual Source = "manual"

	// SourceFingerprint — اقتراح من Fingerprint
	// Ref: FR-OT-004, FR-OT-005
	SourceFingerprint Source = "fingerprint"
)

func (s Source) IsValid() bool {
	return s == SourceManual || s == SourceFingerprint
}

// ============================================================
// Helpers
// ============================================================

// IsPending — هل في انتظار الموافقة؟
func (o *Overtime) IsPending() bool {
	return o.Status == StatusPending
}

// IsApproved — هل مقبول (payable)؟
func (o *Overtime) IsApproved() bool {
	return o.Status == StatusApproved
}

// IsRejected — هل مرفوض؟
func (o *Overtime) IsRejected() bool {
	return o.Status == StatusRejected
}

// IsPayable — هل مدفوع؟
// Ref: BR-011 — OT يبقى payable بعد الموافقة فقط
func (o *Overtime) IsPayable() bool {
	return o.Status == StatusApproved
}

// IsFingerprintSuggestion — هل ده اقتراح من fingerprint؟
// Ref: FR-OT-004, BR-012
func (o *Overtime) IsFingerprintSuggestion() bool {
	return o.Source == SourceFingerprint
}
