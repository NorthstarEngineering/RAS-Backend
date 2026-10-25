package transfer

import (
	"math"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityManagesBranchByRoleAndScope(t *testing.T) {
	branchA, branchB := uuid.New(), uuid.New()
	tests := []struct {
		name string
		id   Identity
		want uuid.UUID
		ok   bool
	}{{"CEO", Identity{Role: RoleCEO}, branchB, true}, {"own branch", Identity{Role: RoleBranchManager, BranchID: branchA}, branchA, true}, {"other branch", Identity{Role: RoleBranchManager, BranchID: branchA}, branchB, false}, {"unknown role", Identity{Role: "SUPPLY_MANAGER", BranchID: branchA}, branchA, false}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.ManagesBranch(tt.want); got != tt.ok {
				t.Fatalf("ManagesBranch() = %v, want %v", got, tt.ok)
			}
		})
	}
}

func TestRound2UsesFinancialRounding(t *testing.T) {
	for input, want := range map[float64]float64{1.234: 1.23, 1.235: 1.24, 12.999: 13} {
		if got := round2(input); math.Abs(got-want) > 0.000001 {
			t.Fatalf("round2(%v) = %v, want %v", input, got, want)
		}
	}
}

func TestTransferStatusesAreValidOnlyForKnownLifecycleStates(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusApproved, StatusRejected, StatusSent, StatusReceived} {
		if !status.Valid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if Status("CANCELLED").Valid() {
		t.Fatal("unknown transfer status should be invalid")
	}
}

func TestIdentityDoesNotManageZeroBranchForBranchManager(t *testing.T) {
	identity := Identity{Role: RoleBranchManager, BranchID: uuid.New()}
	if identity.ManagesBranch(uuid.Nil) {
		t.Fatal("branch manager must not manage the zero branch")
	}
}

func TestTransferPrioritiesHaveStableWireValues(t *testing.T) {
	if PriorityNormal != "NORMAL" || PriorityUrgent != "URGENT" {
		t.Fatalf("unexpected priorities: %q, %q", PriorityNormal, PriorityUrgent)
	}
}

func TestTransferActionsHaveAuditValues(t *testing.T) {
	for _, action := range []Action{ActionRequested, ActionApproved, ActionRejected, ActionSent, ActionReceived} {
		if action == "" {
			t.Fatal("audit action must not be empty")
		}
	}
}
