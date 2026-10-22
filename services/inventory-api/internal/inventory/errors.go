package inventory
 
import (
	"errors"
	"fmt"
 
	"github.com/shopspring/decimal"
)
 
var (
	ErrMissingIdentity      = errors.New("missing or invalid identity headers")
	ErrInvalidMovementType  = errors.New("invalid movement type: must be IN, OUT or ADJUSTMENT")
	ErrInvalidQuantity      = errors.New("quantity must be greater than zero")
	ErrNegativeQuantity     = errors.New("adjusted quantity cannot be negative")
	ErrInvalidUnitCost      = errors.New("unit cost cannot be negative")
	ErrUnitCostNotAllowed   = errors.New("unit cost is only accepted for IN movements")
	ErrReasonRequired       = errors.New("reason is required for OUT and ADJUSTMENT movements")
	ErrInvalidDestination   = errors.New("destination branch is only allowed for OUT and must differ from the source branch")
	ErrInvalidBranch        = errors.New("branch_id is required")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrUnauthorizedBranch   = errors.New("not allowed to access this branch")
	ErrAdjustmentNotAllowed = errors.New("role is not allowed to record adjustments")
	ErrInventoryNotFound    = errors.New("inventory record not found")
	ErrGoodRequired         = errors.New("good_id is required")
	ErrQuantityRequired     = errors.New("quantity is required (or delta for ADJUSTMENT)")
	ErrInvalidPrecision     = errors.New("amounts accept at most 2 decimal places")
	ErrQuantityTooLarge     = errors.New("amount exceeds the maximum allowed value")
	ErrInvalidDateRange     = errors.New("'from' must be before 'to'")
 
	// ---- delta adjustments ----
	ErrQuantityAndDelta = errors.New("send either quantity or delta for an adjustment, not both")
	ErrDeltaNotAllowed  = errors.New("delta is only accepted for ADJUSTMENT movements")
	ErrZeroDelta        = errors.New("adjustment delta cannot be zero")
	ErrNegativeBalance  = errors.New("adjustment would make the balance negative")
 
	// ---- reason code / adjustment kind / free text ----
	ErrInvalidReasonCode          = errors.New("invalid reason_code")
	ErrReasonCodeNotAllowed       = errors.New("reason_code is only accepted for OUT and ADJUSTMENT movements")
	ErrInvalidAdjustmentKind      = errors.New("invalid adjustment_kind: must be SHORTAGE, SURPLUS or OPENING")
	ErrAdjustmentKindNotAllowed   = errors.New("adjustment_kind is only accepted for ADJUSTMENT movements")
	ErrAdjustmentKindMismatch     = errors.New("adjustment_kind does not match the direction of the change")
	ErrTextTooLong                = errors.New("text value is too long")
 
	// ---- filters & settings ----
	ErrInvalidStatus        = errors.New("invalid status: must be CRITICAL, LOW or AVAILABLE")
	ErrNoSettingsProvided   = errors.New("at least one setting must be provided")
	ErrNegativeSetting      = errors.New("stock limits cannot be negative")
	ErrInvalidStockSettings = errors.New("max_stock must be greater than or equal to min_stock and reorder_level")
 
	// ---- concurrency ----
	ErrStaleBalance = errors.New("balance changed since it was read")
)
 
// InsufficientStockError carries details and matches ErrInsufficientStock via errors.Is.
type InsufficientStockError struct {
	Available decimal.Decimal
	Requested decimal.Decimal
}
 
func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("insufficient stock: available %s, requested %s",
		e.Available.StringFixed(2), e.Requested.StringFixed(2))
}
 
func (e *InsufficientStockError) Unwrap() error { return ErrInsufficientStock }
 
// StaleBalanceError is returned when expected_quantity_before no longer matches the balance.
type StaleBalanceError struct {
	Expected decimal.Decimal
	Actual   decimal.Decimal
}
 
func (e *StaleBalanceError) Error() string {
	return fmt.Sprintf("balance changed since it was read: expected %s, current %s",
		e.Expected.StringFixed(2), e.Actual.StringFixed(2))
}
 
func (e *StaleBalanceError) Unwrap() error { return ErrStaleBalance }
 