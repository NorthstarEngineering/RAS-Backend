package employee

import "errors"

// ============================================================
// Domain Errors — أخطاء خاصة بموديول الموظفين
//
// نستخدم sentinel errors عشان:
//   1. نستخدمها مع errors.Is() في الـ handler
//   2. نفصل بين أخطاء الـ domain والأخطاء التقنية
//   3. نتحكم في الـ HTTP status code لكل نوع
// ============================================================

var (
	// ---------- Not Found ----------

	// ErrEmployeeNotFound — الموظف مش موجود
	ErrEmployeeNotFound = errors.New("employee not found")

	// ErrBranchNotFound — الفرع مش موجود
	ErrBranchNotFound = errors.New("branch not found")

	// ---------- Validation ----------

	// ErrNameRequired — الاسم مطلوب
	ErrNameRequired = errors.New("name is required")

	// ErrPhoneRequired — رقم التليفون مطلوب
	ErrPhoneRequired = errors.New("phone is required")

	// ErrPhoneInvalid — رقم التليفون غير صالح
	// Ref: constraint chk_phone_format
	ErrPhoneInvalid = errors.New("phone must be 10-15 digits")

	// ErrNationalIDRequired — الرقم القومي مطلوب
	ErrNationalIDRequired = errors.New("national_id is required")

	// ErrNationalIDInvalid — الرقم القومي غير صالح
	// Ref: constraint chk_national_id_format
	ErrNationalIDInvalid = errors.New("national_id must be exactly 14 digits")

	// ErrNationalIDDuplicate — الرقم القومي مستخدم بالفعل
	// Ref: unique constraint uq_employees_national_id
	ErrNationalIDDuplicate = errors.New("national_id already exists")

	// ErrInvalidPaymentType — نوع الدفع غير صالح
	// Ref: FR-EMP-004, constraint chk_payment_type
	ErrInvalidPaymentType = errors.New("payment_type must be 'daily' or 'monthly'")

	// ErrInvalidPaymentAmount — مبلغ الأجر غير صالح
	// Ref: constraint chk_payment_amount_positive
	ErrInvalidPaymentAmount = errors.New("payment_amount must be greater than 0")

	// ErrInvalidHireDate — تاريخ التعيين غير صالح
	ErrInvalidHireDate = errors.New("hire_date is required and cannot be in the future")

	// ErrInvalidTerminationDate — تاريخ إنهاء الخدمة غير صالح
	// Ref: constraint chk_termination_after_hire
	ErrInvalidTerminationDate = errors.New("termination_date must be on or after hire_date")

	// ErrBranchRequired — الفرع مطلوب
	ErrBranchRequired = errors.New("branch_id is required")

	// ErrInvalidEmail — البريد الإلكتروني غير صالح
	ErrInvalidEmail = errors.New("email format is invalid")

	// ---------- Business Logic ----------

	// ErrAlreadyTerminated — الموظف متوقف بالفعل
	ErrAlreadyTerminated = errors.New("employee is already terminated")

	// ErrAlreadyActive — الموظف فعّال بالفعل
	ErrAlreadyActive = errors.New("employee is already active")

	// ErrCannotTerminateActive — مينفعش نوقف موظف فعّال من غير توضيح
	ErrCannotTerminateActive = errors.New("must provide termination_date to terminate an active employee")

	// ---------- Authorization ----------

	// ErrUnauthorizedBranch — المحاولة على فرع مش بتاع المستخدم
	// Ref: FR-AUTH-004, NFR-SEC-003
	ErrUnauthorizedBranch = errors.New("unauthorized: cross-branch access denied")

	// ErrForbiddenAction — الدور مش مسموح له بالعملية دي
	// Ref: FR-AUTH-003
	ErrForbiddenAction = errors.New("forbidden: role not allowed to perform this action")
)
