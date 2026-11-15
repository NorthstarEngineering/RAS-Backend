package transfer

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusApproved Status = "APPROVED"
	StatusRejected Status = "REJECTED"
	StatusSent     Status = "SENT"
	StatusReceived Status = "RECEIVED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusSent, StatusReceived:
		return true
	}
	return false
}

type Priority string

const (
	PriorityNormal Priority = "NORMAL"
	PriorityUrgent Priority = "URGENT"
)

type Action string

const (
	ActionRequested Action = "REQUESTED"
	ActionApproved  Action = "APPROVED"
	ActionRejected  Action = "REJECTED"
	ActionSent      Action = "SENT"
	ActionReceived  Action = "RECEIVED"
)

type Direction string

const (
	DirectionIncoming Direction = "incoming"
	DirectionOutgoing Direction = "outgoing"
)

const (
	RoleCEO           = "CEO"
	RoleBranchManager = "BRANCH_MANAGER"
)

// Identity is built from the gateway headers (X-User-ID, X-User-Role, X-Branch-ID).
type Identity struct {
	UserID   uuid.UUID
	Role     string
	BranchID uuid.UUID
}

// ManagesBranch reports whether the caller may act on behalf of the given branch.
func (i Identity) ManagesBranch(branchID uuid.UUID) bool {
	if i.Role == RoleCEO {
		return true
	}
	return i.Role == RoleBranchManager && i.BranchID == branchID
}

type Transfer struct {
	ID             uuid.UUID `db:"id" json:"id"`
	Code           string    `db:"code" json:"code"`
	SourceBranchID uuid.UUID `db:"source_branch_id" json:"source_branch_id"`
	DestBranchID   uuid.UUID `db:"dest_branch_id" json:"dest_branch_id"`
	GoodID         uuid.UUID `db:"good_id" json:"good_id"`

	RequestedQty float64  `db:"requested_qty" json:"requested_quantity"`
	ApprovedQty  *float64 `db:"approved_qty" json:"approved_quantity"`
	SentQty      *float64 `db:"sent_qty" json:"sent_quantity"`
	ReceivedQty  *float64 `db:"received_qty" json:"received_quantity"`
	ShortageQty  float64  `db:"shortage_qty" json:"shortage_quantity"`
	UnitCost     *float64 `db:"unit_cost" json:"unit_cost"`

	Priority     Priority `db:"priority" json:"priority"`
	Reason       string   `db:"reason" json:"reason"`
	Status       Status   `db:"status" json:"status"`
	DecisionNote *string  `db:"decision_note" json:"decision_note"`

	VehicleType       *string    `db:"vehicle_type" json:"vehicle_type"`
	DriverName        *string    `db:"driver_name" json:"driver_name"`
	DriverPhone       *string    `db:"driver_phone" json:"driver_phone"`
	ExpectedDeparture *time.Time `db:"expected_departure" json:"expected_departure"`
	TemperatureC      *float64   `db:"temperature_c" json:"temperature_c"`
	ReceiveNote       *string    `db:"receive_note" json:"receive_note"`

	RequestedBy uuid.UUID  `db:"requested_by" json:"requested_by"`
	RequestedAt time.Time  `db:"requested_at" json:"requested_at"`
	DecidedBy   *uuid.UUID `db:"decided_by" json:"decided_by"`
	DecidedAt   *time.Time `db:"decided_at" json:"decided_at"`
	SentBy      *uuid.UUID `db:"sent_by" json:"sent_by"`
	SentAt      *time.Time `db:"sent_at" json:"sent_at"`
	ReceivedBy  *uuid.UUID `db:"received_by" json:"received_by"`
	ReceivedAt  *time.Time `db:"received_at" json:"received_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

// TransferView is a Transfer joined with branch and good display data.
type TransferView struct {
	Transfer
	SourceBranchName string `db:"source_branch_name" json:"source_branch_name"`
	SourceBranchCode string `db:"source_branch_code" json:"source_branch_code"`
	DestBranchName   string `db:"dest_branch_name" json:"dest_branch_name"`
	DestBranchCode   string `db:"dest_branch_code" json:"dest_branch_code"`
	GoodName         string `db:"good_name" json:"good_name"`
	GoodSKU          string `db:"good_sku" json:"good_sku"`
	GoodUnit         string `db:"good_unit" json:"good_unit"`
}

type TransferEvent struct {
	ID         uuid.UUID `db:"id" json:"id"`
	TransferID uuid.UUID `db:"transfer_id" json:"transfer_id"`
	Action     Action    `db:"action" json:"action"`
	ActorID    uuid.UUID `db:"actor_id" json:"actor_id"`
	Note       *string   `db:"note" json:"note"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// TransferDetail is the transfer plus its full audit timeline.
type TransferDetail struct {
	TransferView
	Timeline []TransferEvent `json:"timeline"`
}

type Stats struct {
	Total           int     `db:"total" json:"total"`
	PendingApproval int     `db:"pending_approval" json:"pending_approval"`
	InTransit       int     `db:"in_transit" json:"in_transit"`
	Received        int     `db:"received" json:"received"`
	TotalQuantity   float64 `db:"total_quantity" json:"total_quantity"`
	ShortageCases   int     `db:"shortage_cases" json:"shortage_cases"`
	MatchRate       float64 `db:"-" json:"match_rate"`
}

type ListFilter struct {
	BranchID  *uuid.UUID
	Direction Direction
	Status    Status
	Search    string
	From      *time.Time
	To        *time.Time
	Page      int
	Limit     int
}