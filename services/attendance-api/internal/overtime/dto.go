package overtime

import "time"

// ============================================================
// Request DTOs
// ============================================================

// CreateOvertimeRequest — POST /api/v1/overtime
// Ref: FR-OT-001, FR-OT-002
type CreateOvertimeRequest struct {
	EmployeeID  string  `json:"employee_id"   binding:"required,uuid"`
	WorkDate    string  `json:"work_date"     binding:"required"` // "2026-10-11"
	Hours       float64 `json:"hours"         binding:"required,gt=0"`
	RatePerHour float64 `json:"rate_per_hour" binding:"required,gt=0"`
	Source      string  `json:"source"        binding:"required,oneof=manual fingerprint"`
}

// ApproveOvertimeRequest — PATCH /api/v1/overtime/:id/approve
// Ref: FR-OT-007
type ApproveOvertimeRequest struct {
	// No body needed — approver is from context
}

// RejectOvertimeRequest — PATCH /api/v1/overtime/:id/reject
// Ref: FR-OT-008
type RejectOvertimeRequest struct {
	Reason string `json:"reason" binding:"required,min=3,max=500"`
}

// ============================================================
// Response DTOs
// ============================================================

// OvertimeResponse — تمثيل الـ OT في الـ API
type OvertimeResponse struct {
	ID         string `json:"id"`
	EmployeeID string `json:"employee_id"`
	BranchID   string `json:"branch_id"`
	WorkDate   string `json:"work_date"` // "2026-10-11"

	// ---------- الحساب ----------
	Hours       float64 `json:"hours"`
	RatePerHour float64 `json:"rate_per_hour"`
	Amount      float64 `json:"amount"` // hours × rate

	// ---------- الحالة ----------
	Status string `json:"status"`
	Source string `json:"source"`

	// ---------- Payable ----------
	// Ref: BR-011 — OT يبقى payable بعد الموافقة
	IsPayable bool `json:"is_payable"`

	// ---------- الموافقة ----------
	ApprovedBy *string `json:"approved_by,omitempty"`
	ApprovedAt *string `json:"approved_at,omitempty"`

	// ---------- الرفض ----------
	RejectedBy      *string `json:"rejected_by,omitempty"`
	RejectedAt      *string `json:"rejected_at,omitempty"`
	RejectionReason *string `json:"rejection_reason,omitempty"`

	// ---------- الإنشاء ----------
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListOvertimeResponse — قائمة سجلات OT
type ListOvertimeResponse struct {
	Records []OvertimeResponse `json:"records"`
	Total   int                `json:"total"`
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
// Mappers
// ============================================================

// ToResponse — Overtime → OvertimeResponse
func (o *Overtime) ToResponse() OvertimeResponse {
	resp := OvertimeResponse{
		ID:              o.ID,
		EmployeeID:      o.EmployeeID,
		BranchID:        o.BranchID,
		WorkDate:        o.WorkDate.Format("2006-01-02"),
		Hours:           o.Hours,
		RatePerHour:     o.RatePerHour,
		Amount:          o.Amount,
		Status:          string(o.Status),
		Source:          string(o.Source),
		IsPayable:       o.IsPayable(),
		ApprovedBy:      o.ApprovedBy,
		RejectedBy:      o.RejectedBy,
		RejectionReason: o.RejectionReason,
		CreatedBy:       o.CreatedBy,
		CreatedAt:       o.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       o.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if o.ApprovedAt != nil {
		s := o.ApprovedAt.UTC().Format(time.RFC3339)
		resp.ApprovedAt = &s
	}
	if o.RejectedAt != nil {
		s := o.RejectedAt.UTC().Format(time.RFC3339)
		resp.RejectedAt = &s
	}
	return resp
}

// ToListResponse — []*Overtime → ListOvertimeResponse
func ToListResponse(list []*Overtime) ListOvertimeResponse {
	out := ListOvertimeResponse{
		Records: make([]OvertimeResponse, 0, len(list)),
		Total:   len(list),
	}
	for _, o := range list {
		out.Records = append(out.Records, o.ToResponse())
	}
	return out
}
