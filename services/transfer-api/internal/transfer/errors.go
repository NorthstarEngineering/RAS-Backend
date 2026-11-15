package transfer

import "errors"

var (
	ErrInvalidIdentity = errors.New("missing or invalid identity headers")
	ErrForbidden       = errors.New("you are not allowed to perform this action on this transfer")
	ErrInvalidRequest  = errors.New("invalid request")
	ErrInvalidID       = errors.New("invalid transfer id")

	ErrTransferNotFound = errors.New("transfer not found")
	ErrBranchNotFound   = errors.New("source or destination branch not found or inactive")
	ErrGoodNotFound     = errors.New("good not found")

	ErrSameBranch               = errors.New("source and destination branches must be different")
	ErrInvalidQuantity          = errors.New("quantity must be greater than zero")
	ErrApprovedExceedsRequested = errors.New("approved quantity cannot exceed the requested quantity")
	ErrReceivedExceedsSent      = errors.New("received quantity cannot exceed the sent quantity")
	ErrReasonRequired           = errors.New("a reason/note is required")

	ErrInvalidState      = errors.New("transfer is not in a valid state for this action")
	ErrInsufficientStock = errors.New("insufficient stock in the source branch")
)