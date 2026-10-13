package attendance

import "errors"

// ============================================================
// Domain Errors — أخطاء خاصة بموديول الحضور والخصومات
//
// نستخدم sentinel errors عشان:
//   1. نستخدمها مع errors.Is() في الـ handler
//   2. نفصل بين أخطاء الـ domain والأخطاء التقنية
//   3. نتحكم في الـ HTTP status code لكل نوع
// ============================================================

var (
	// ---------- Employee Errors ----------

	// ErrEmployeeNotFound — الموظف مش موجود (FR-ATT-001)
	ErrEmployeeNotFound = errors.New("employee not found")

	// ErrEmployeeInactive — الموظف غير فعّال (اتفصل مثلاً)
	ErrEmployeeInactive = errors.New("employee is inactive")

	// ---------- Branch Errors ----------

	// ErrBranchNotFound — الفرع مش موجود
	ErrBranchNotFound = errors.New("branch not found")

	// ErrCentralKitchenNotAllowed — مينفعش تسجّل حضور لـ Central Kitchen
	// Ref: FR-BRANCH-003
	ErrCentralKitchenNotAllowed = errors.New("central kitchen is stock-only; attendance not allowed")

	// ---------- Attendance Errors ----------

	// ErrAlreadyCheckedIn — الموظف سجّل حضور بالفعل النهارده
	// Ref: UC-ATT-001 (alt flow: duplicate event)
	ErrAlreadyCheckedIn = errors.New("employee already checked in today")

	// ErrNotCheckedIn — الموظف بيحاول يسجّل انصراف من غير حضور
	// Ref: UC-ATT-001 (alt flow)
	ErrNotCheckedIn = errors.New("no open check-in found for this employee")

	// ErrAlreadyCheckedOut — الموظف سجّل انصراف بالفعل
	ErrAlreadyCheckedOut = errors.New("employee already checked out")

	// ---------- Deduction Errors ----------

	// ErrDeductionNotFound — الخصم مش موجود
	ErrDeductionNotFound = errors.New("deduction not found")

	// ErrReasonRequired — لازم سبب للتعديل أو الإلغاء
	// Ref: FR-DED-009, BR-010
	ErrReasonRequired = errors.New("reason is required for adjustment or cancellation")

	// ErrAlreadyCancelled — الخصم ملغي بالفعل
	ErrAlreadyCancelled = errors.New("deduction is already cancelled")

	// ErrInvalidAmount — قيمة الخصم غير صحيحة (سالب مثلاً)
	ErrInvalidAmount = errors.New("invalid deduction amount")

	// ErrNoAdjustmentSpecified — ما فيش أي تعديل محدد
	ErrNoAdjustmentSpecified = errors.New("no adjustment specified: provide new_amount or cancel=true")

	// ---------- Authorization Errors ----------

	// ErrUnauthorizedBranch — المحاولة على فرع مش بتاع المستخدم
	// Ref: FR-AUTH-004, NFR-SEC-003
	ErrUnauthorizedBranch = errors.New("unauthorized: cross-branch access denied")

	// ErrForbiddenAction — الدور مش مسموح له بالعملية دي
	// Ref: FR-AUTH-003
	ErrForbiddenAction = errors.New("forbidden: role not allowed to perform this action")
)
