package overtime

import "errors"

// ============================================================
// Domain Errors — أخطاء خاصة بموديول Overtime
//
// Ref: FR-OT-001..008, BR-011, BR-012
// ============================================================

var (
	// ---------- Not Found ----------

	// ErrOvertimeNotFound — سجل OT مش موجود
	ErrOvertimeNotFound = errors.New("overtime record not found")

	// ErrEmployeeNotFound — الموظف مش موجود
	ErrEmployeeNotFound = errors.New("employee not found")

	// ErrBranchNotFound — الفرع مش موجود
	ErrBranchNotFound = errors.New("branch not found")

	// ---------- Validation ----------

	// ErrEmployeeRequired — الموظف مطلوب
	ErrEmployeeRequired = errors.New("employee_id is required")

	// ErrWorkDateRequired — تاريخ العمل مطلوب
	ErrWorkDateRequired = errors.New("work_date is required")

	// ErrWorkDateFuture — تاريخ العمل في المستقبل
	ErrWorkDateFuture = errors.New("work_date cannot be in the future")

	// ErrInvalidHours — عدد الساعات غير صالح
	ErrInvalidHours = errors.New("hours must be greater than 0")

	// ErrInvalidRate — سعر الساعة غير صالح
	ErrInvalidRate = errors.New("rate_per_hour must be greater than 0")

	// ErrInvalidSource — مصدر الـ OT غير صالح
	ErrInvalidSource = errors.New("source must be 'manual' or 'fingerprint'")

	// ErrRejectionReasonRequired — لازم سبب الرفض
	// Ref: FR-OT-008
	ErrRejectionReasonRequired = errors.New("rejection_reason is required when rejecting")

	// ---------- Business Logic ----------

	// ErrAlreadyApproved — الـ OT مقبول بالفعل
	// Ref: FR-OT-007
	ErrAlreadyApproved = errors.New("overtime is already approved")

	// ErrAlreadyRejected — الـ OT مرفوض بالفعل
	// Ref: FR-OT-008
	ErrAlreadyRejected = errors.New("overtime is already rejected")

	// ErrNotPending — الـ OT مش في حالة pending
	// Ref: FR-OT-006
	ErrNotPending = errors.New("overtime is not pending; cannot change status")

	// ErrCannotApproveRejected — مينفعش نوافق على مرفوض
	ErrCannotApproveRejected = errors.New("cannot approve a rejected overtime record")

	// ErrCannotRejectApproved — مينفعش نرفض مقبول
	ErrCannotRejectApproved = errors.New("cannot reject an approved overtime record")

	// ---------- Authorization ----------

	// ErrUnauthorizedBranch — المحاولة على فرع مش بتاع المستخدم
	// Ref: FR-AUTH-004, NFR-SEC-003
	ErrUnauthorizedBranch = errors.New("unauthorized: cross-branch access denied")

	// ErrForbiddenAction — الدور مش مسموح له
	// Ref: FR-AUTH-003
	ErrForbiddenAction = errors.New("forbidden: role not allowed to perform this action")
)
