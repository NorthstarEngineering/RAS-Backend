package employee

import "time"

// ============================================================
// Employee — كيان الموظف
// Ref: FR-EMP-001..006, BR-002, BR-003, BR-004, BR-005
// ============================================================

type Employee struct {
	ID       string `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	BranchID string `db:"branch_id" json:"branch_id"`

	// ---------- إجباري ----------
	Phone      string `db:"phone" json:"phone"`
	NationalID string `db:"national_id" json:"national_id"`

	// ---------- اختياري ----------
	JobTitle *string `db:"job_title" json:"job_title,omitempty"`
	Email    *string `db:"email" json:"email,omitempty"`
	Address  *string `db:"address" json:"address,omitempty"`

	// ---------- الراتب ----------
	// Ref: FR-EMP-004, FR-EMP-005, FR-EMP-006
	PaymentType   PaymentType `db:"payment_type" json:"payment_type"`
	PaymentAmount float64     `db:"payment_amount" json:"payment_amount"`

	// ---------- التواريخ ----------
	HireDate        time.Time  `db:"hire_date" json:"hire_date"`
	TerminationDate *time.Time `db:"termination_date" json:"termination_date,omitempty"`

	// ---------- الحالة ----------
	Active    bool      `db:"active" json:"active"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================
// PaymentType — نوع الدفع
// Ref: FR-EMP-004, BR-004
// ============================================================

type PaymentType string

const (
	// PaymentDaily — مبلغ/يوم
	PaymentDaily PaymentType = "daily"

	// PaymentMonthly — مبلغ/شهر
	PaymentMonthly PaymentType = "monthly"
)

// IsValid — يتحقق إن النوع صحيح
func (p PaymentType) IsValid() bool {
	return p == PaymentDaily || p == PaymentMonthly
}

// ============================================================
// Helpers — دوال مساعدة
// ============================================================

// IsActive — هل الموظف فعّال؟
func (e *Employee) IsActive() bool {
	return e.Active
}

// IsDaily — هل الدفع يومي؟
func (e *Employee) IsDaily() bool {
	return e.PaymentType == PaymentDaily
}

// IsMonthly — هل الدفع شهري؟
func (e *Employee) IsMonthly() bool {
	return e.PaymentType == PaymentMonthly
}

// IsTerminated — هل الموظف اتوقف؟
func (e *Employee) IsTerminated() bool {
	return !e.Active && e.TerminationDate != nil
}

// ============================================================
// Compensation — الأجر
// ============================================================

// CompensationInfo — معلومات الأجر (للـ Labor Cost في FR-FIN-004)
type CompensationInfo struct {
	Type   PaymentType `json:"type"`
	Amount float64     `json:"amount"`
}

// Compensation — يرجع معلومات الأجر
// Ref: FR-FIN-004 (Employee Cost for profitability reporting)
func (e *Employee) Compensation() CompensationInfo {
	return CompensationInfo{
		Type:   e.PaymentType,
		Amount: e.PaymentAmount,
	}
}
