package attendance

import "time"

// ============================================================
// Request DTOs — اللي بتيجي من Flutter
// ============================================================

// CheckInRequest — POST /api/v1/attendance/check-in
// Ref: FR-ATT-001, FR-ATT-003
type CheckInRequest struct {
	EmployeeID string `json:"employee_id" binding:"required,uuid"`
	Source     string `json:"source"      binding:"required,oneof=manual fingerprint"`
}

// CheckOutRequest — POST /api/v1/attendance/check-out
// Ref: FR-ATT-002
type CheckOutRequest struct {
	EmployeeID string `json:"employee_id" binding:"required,uuid"`
}

// AdjustDeductionRequest — PATCH /api/v1/attendance/deductions/{id}
// Ref: FR-DED-006, FR-DED-007, FR-DED-008, FR-DED-009
type AdjustDeductionRequest struct {
	// NewAmount — القيمة الجديدة للخصم (لو التعديل في المبلغ)
	NewAmount *float64 `json:"new_amount,omitempty" binding:"omitempty,gte=0"`

	// Cancel — لو true، الخصم يتلغى بالكامل
	Cancel bool `json:"cancel"`

	// Reason — إجباري في كل تعديل/إلغاء
	// Ref: FR-DED-009, BR-010
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

// ============================================================
// Response DTOs
// ============================================================

type AttendanceRecordResponse struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	BranchID   string `json:"branch_id"`
	WorkDate   string `json:"work_date"`

	CheckInAt  string  `json:"check_in_at"`
	CheckOutAt *string `json:"check_out_at,omitempty"`

	Status string `json:"status"`
	Source string `json:"source"`

	Deduction *DeductionResponse `json:"deduction,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type DeductionResponse struct {
	ID           string `json:"id"`
	AttendanceID string `json:"attendance_id"`
	EmployeeID   string `json:"employee_id"`
	BranchID     string `json:"branch_id"`
	WorkDate     string `json:"work_date"`

	LateMinutes    int     `json:"late_minutes"`
	CompletedHours int     `json:"completed_hours"`
	RatePerHour    float64 `json:"rate_per_hour"`
	OriginalAmount float64 `json:"original_amount"`

	AdjustedAmount   *float64 `json:"adjusted_amount,omitempty"`
	AdjustmentReason *string  `json:"adjustment_reason,omitempty"`
	AdjustedBy       *string  `json:"adjusted_by,omitempty"`
	AdjustedAt       *string  `json:"adjusted_at,omitempty"`

	Cancelled    bool    `json:"cancelled"`
	CancelledBy  *string `json:"cancelled_by,omitempty"`
	CancelledAt  *string `json:"cancelled_at,omitempty"`
	CancelReason *string `json:"cancel_reason,omitempty"`

	EffectiveAmount float64 `json:"effective_amount"`

	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

type CheckInResponse struct {
	Record    AttendanceRecordResponse `json:"record"`
	Deduction *DeductionResponse       `json:"deduction,omitempty"`
}

// ============================================================
// Error Response
// ============================================================

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ============================================================
// Mappers — من Domain إلى Response
// ============================================================

func (r *AttendanceRecord) ToResponse() AttendanceRecordResponse {
	resp := AttendanceRecordResponse{
		ID:         r.ID,
		EmployeeID: r.EmployeeID,
		BranchID:   r.BranchID,
		WorkDate:   r.WorkDate.Format("2006-01-02"),
		CheckInAt:  r.CheckInAt.UTC().Format(time.RFC3339),
		Status:     string(r.Status),
		Source:     string(r.Source),
		CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  r.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if r.CheckOutAt != nil {
		s := r.CheckOutAt.UTC().Format(time.RFC3339)
		resp.CheckOutAt = &s
	}
	if r.Deduction != nil {
		d := r.Deduction.ToResponse()
		resp.Deduction = &d
	}
	return resp
}

func (d *Deduction) ToResponse() DeductionResponse {
	resp := DeductionResponse{
		ID:               d.ID,
		AttendanceID:     d.AttendanceID,
		EmployeeID:       d.EmployeeID,
		BranchID:         d.BranchID,
		WorkDate:         d.WorkDate.Format("2006-01-02"),
		LateMinutes:      d.LateMinutes,
		CompletedHours:   d.CompletedHours,
		RatePerHour:      d.RatePerHour,
		OriginalAmount:   d.OriginalAmount,
		AdjustedAmount:   d.AdjustedAmount,
		AdjustmentReason: d.AdjustmentReason,
		AdjustedBy:       d.AdjustedBy,
		Cancelled:        d.Cancelled,
		CancelledBy:      d.CancelledBy,
		CancelReason:     d.CancelReason,
		EffectiveAmount:  d.EffectiveAmount(),
		Source:           d.Source,
		CreatedAt:        d.CreatedAt.UTC().Format(time.RFC3339),
	}
	if d.AdjustedAt != nil {
		s := d.AdjustedAt.UTC().Format(time.RFC3339)
		resp.AdjustedAt = &s
	}
	if d.CancelledAt != nil {
		s := d.CancelledAt.UTC().Format(time.RFC3339)
		resp.CancelledAt = &s
	}
	return resp
}
