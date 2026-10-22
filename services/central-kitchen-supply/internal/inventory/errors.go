package inventory

import "errors"

// ============================================================
// Domain Errors - Inventory Module
// ============================================================

var (
	// ---------- Not Found ----------
	ErrInventoryNotFound = errors.New("inventory record not found")
	ErrMovementNotFound  = errors.New("inventory movement not found")
	ErrGoodNotFound      = errors.New("good item not found")

	// ---------- Validation ----------
	ErrInvalidQuantity     = errors.New("quantity must be greater than zero")
	ErrInvalidMovementType = errors.New("invalid movement type")
	ErrReasonRequired      = errors.New("reason is required for this movement type")

	// ---------- Business Logic ----------
	ErrInsufficientStock = errors.New("insufficient stock for this operation")

	// ---------- Authorization ----------
	ErrUnauthorizedBranch = errors.New("unauthorized: cross-branch access denied")
	ErrForbiddenAction    = errors.New("forbidden: role not allowed to perform this action")
)
