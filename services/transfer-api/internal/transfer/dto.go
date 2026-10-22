package transfer

import (
	"time"

	"github.com/google/uuid"
)

// CreateTransferRequest - screen TR-02.
// source_branch_id is optional and defaults to the caller's own branch (X-Branch-ID).
type CreateTransferRequest struct {
	SourceBranchID uuid.UUID `json:"source_branch_id"`
	DestBranchID   uuid.UUID `json:"dest_branch_id" binding:"required"`
	GoodID         uuid.UUID `json:"good_id" binding:"required"`
	Quantity       float64   `json:"quantity" binding:"required,gt=0"`
	Priority       Priority  `json:"priority" binding:"omitempty,oneof=NORMAL URGENT"`
	Reason         string    `json:"reason" binding:"required,max=250"`
}

// ApproveRequest - screen TR-03. approved_quantity defaults to the requested quantity.
// A note is required when the approved quantity is lower than the requested one.
type ApproveRequest struct {
	ApprovedQuantity *float64 `json:"approved_quantity" binding:"omitempty,gt=0"`
	Note             string   `json:"note"`
}

type RejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// SendRequest - screen TR-04.
type SendRequest struct {
	VehicleType       string     `json:"vehicle_type"`
	DriverName        string     `json:"driver_name"`
	DriverPhone       string     `json:"driver_phone"`
	ExpectedDeparture *time.Time `json:"expected_departure"`
	TemperatureC      *float64   `json:"temperature_c"`
}

// ReceiveRequest - screen TR-05. A note is required when received != sent.
type ReceiveRequest struct {
	ReceivedQuantity *float64 `json:"received_quantity" binding:"required,gte=0"`
	Note             string   `json:"note"`
}

type ListResponse struct {
	Items []TransferView `json:"items"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
}