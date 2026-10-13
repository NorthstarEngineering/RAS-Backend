package employee

import "time"

// ============================================================
// Request DTOs
// ============================================================

// CreateEmployeeRequest — POST /api/v1/employees
// Ref: FR-EMP-001, FR-EMP-002
type CreateEmployeeRequest struct {
	Name          string  `json:"name"           binding:"required,min=2,max=200"`
	Phone         string  `json:"phone"          binding:"required"`
	NationalID    string  `json:"national_id"    binding:"required,len=14,numeric"`
	BranchID      string  `json:"branch_id"      binding:"required,uuid"`
	PaymentType   string  `json:"payment_type"   binding:"required,oneof=daily monthly"`
	PaymentAmount float64 `json:"payment_amount" binding:"required,gt=0"`
	HireDate      string  `json:"hire_date"      binding:"required"` // "2026-10-11"

	JobTitle *string `json:"job_title,omitempty"`
	Email    *string `json:"email,omitempty"     binding:"omitempty,email"`
	Address  *string `json:"address,omitempty"`
}

// UpdateEmployeeRequest — PATCH /api/v1/employees/:id
// Ref: FR-EMP-006
type UpdateEmployeeRequest struct {
	Name          *string  `json:"name,omitempty"           binding:"omitempty,min=2,max=200"`
	Phone         *string  `json:"phone,omitempty"`
	JobTitle      *string  `json:"job_title,omitempty"`
	Email         *string  `json:"email,omitempty"          binding:"omitempty,email"`
	Address       *string  `json:"address,omitempty"`
	PaymentType   *string  `json:"payment_type,omitempty"   binding:"omitempty,oneof=daily monthly"`
	PaymentAmount *float64 `json:"payment_amount,omitempty" binding:"omitempty,gt=0"`
}

// ChangePaymentRequest — PATCH /api/v1/employees/:id/payment
// Ref: FR-EMP-006
type ChangePaymentRequest struct {
	PaymentType   string  `json:"payment_type"   binding:"required,oneof=daily monthly"`
	PaymentAmount float64 `json:"payment_amount" binding:"required,gt=0"`
}

// TerminateRequest — POST /api/v1/employees/:id/terminate
type TerminateRequest struct {
	TerminationDate string `json:"termination_date" binding:"required"` // "2026-10-11"
}

// ============================================================
// Response DTOs
// ============================================================

// EmployeeResponse — تمثيل الموظف في الـ API
type EmployeeResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	BranchID string `json:"branch_id"`

	Phone      string `json:"phone"`
	NationalID string `json:"national_id"`

	JobTitle *string `json:"job_title,omitempty"`
	Email    *string `json:"email,omitempty"`
	Address  *string `json:"address,omitempty"`

	PaymentType   string  `json:"payment_type"`
	PaymentAmount float64 `json:"payment_amount"`

	HireDate        string  `json:"hire_date"`                  // "2026-10-11"
	TerminationDate *string `json:"termination_date,omitempty"` // "2026-10-11"

	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListEmployeesResponse — قائمة موظفين
type ListEmployeesResponse struct {
	Employees []EmployeeResponse `json:"employees"`
	Total     int                `json:"total"`
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

// ToResponse — Employee → EmployeeResponse
func (e *Employee) ToResponse() EmployeeResponse {
	resp := EmployeeResponse{
		ID:            e.ID,
		Name:          e.Name,
		BranchID:      e.BranchID,
		Phone:         e.Phone,
		NationalID:    e.NationalID,
		JobTitle:      e.JobTitle,
		Email:         e.Email,
		Address:       e.Address,
		PaymentType:   string(e.PaymentType),
		PaymentAmount: e.PaymentAmount,
		HireDate:      e.HireDate.Format("2006-01-02"),
		Active:        e.Active,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if e.TerminationDate != nil {
		s := e.TerminationDate.Format("2006-01-02")
		resp.TerminationDate = &s
	}
	return resp
}

// ToListResponse — []*Employee → ListEmployeesResponse
func ToListResponse(emps []*Employee) ListEmployeesResponse {
	out := ListEmployeesResponse{
		Employees: make([]EmployeeResponse, 0, len(emps)),
		Total:     len(emps),
	}
	for _, e := range emps {
		out.Employees = append(out.Employees, e.ToResponse())
	}
	return out
}
