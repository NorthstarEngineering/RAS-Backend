package attendance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type authorizationEmployeeReader struct{ employee *EmployeeInfo }

func (r authorizationEmployeeReader) GetByID(context.Context, string) (*EmployeeInfo, error) {
	return r.employee, nil
}

type authorizationAttendanceRepo struct{ openRecordReads int }

func (r *authorizationAttendanceRepo) GetOpenRecord(context.Context, string) (*AttendanceRecord, error) {
	r.openRecordReads++
	return nil, nil
}
func (*authorizationAttendanceRepo) GetRecordByDate(context.Context, string, time.Time) (*AttendanceRecord, error) {
	return nil, nil
}
func (*authorizationAttendanceRepo) GetRecordByID(context.Context, string) (*AttendanceRecord, error) {
	return nil, nil
}
func (*authorizationAttendanceRepo) CreateRecord(context.Context, *sqlx.Tx, *AttendanceRecord) error {
	return nil
}
func (*authorizationAttendanceRepo) UpdateCheckOut(context.Context, *sqlx.Tx, string, time.Time) error {
	return nil
}
func (*authorizationAttendanceRepo) CreateDeduction(context.Context, *sqlx.Tx, *Deduction) error {
	return nil
}
func (*authorizationAttendanceRepo) GetDeductionByID(context.Context, string) (*Deduction, error) {
	return nil, nil
}
func (*authorizationAttendanceRepo) GetDeductionByAttendance(context.Context, string) (*Deduction, error) {
	return nil, nil
}
func (*authorizationAttendanceRepo) UpdateDeduction(context.Context, *sqlx.Tx, *Deduction) error {
	return nil
}

func TestCheckInRejectsBranchManagerForOtherBranch(t *testing.T) {
	repo := &authorizationAttendanceRepo{}
	employees := authorizationEmployeeReader{employee: &EmployeeInfo{ID: "employee-1", BranchID: "branch-b", Active: true}}
	service := NewService(nil, repo, employees, NewMockBranchReader())

	_, err := service.CheckIn(context.Background(), CheckInInput{
		EmployeeID: "employee-1", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-a", Source: SourceManual,
	})
	if !errors.Is(err, ErrUnauthorizedBranch) {
		t.Fatalf("CheckIn() error = %v, want ErrUnauthorizedBranch", err)
	}
	if repo.openRecordReads != 0 {
		t.Fatal("CheckIn() accessed attendance repository before rejecting cross-branch request")
	}
}

func TestCheckOutRejectsBranchManagerForOtherBranchBeforeReadingAttendance(t *testing.T) {
	repo := &authorizationAttendanceRepo{}
	employees := authorizationEmployeeReader{employee: &EmployeeInfo{ID: "employee-1", BranchID: "branch-b", Active: true}}
	service := NewService(nil, repo, employees, NewMockBranchReader())

	_, err := service.CheckOut(context.Background(), CheckOutInput{
		EmployeeID: "employee-1", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-a",
	})
	if !errors.Is(err, ErrUnauthorizedBranch) {
		t.Fatalf("CheckOut() error = %v, want ErrUnauthorizedBranch", err)
	}
	if repo.openRecordReads != 0 {
		t.Fatal("CheckOut() accessed attendance repository before rejecting cross-branch request")
	}
}

func TestAuthorizeEmployeeBranch(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		actorBranch    string
		employeeBranch string
		wantErr        error
	}{
		{name: "CEO may act across branches", role: "CEO", employeeBranch: "branch-b"},
		{name: "manager may act in own branch", role: "BRANCH_MANAGER", actorBranch: "branch-a", employeeBranch: "branch-a"},
		{name: "manager cannot act in another branch", role: "BRANCH_MANAGER", actorBranch: "branch-a", employeeBranch: "branch-b", wantErr: ErrUnauthorizedBranch},
		{name: "manager must have branch claim", role: "BRANCH_MANAGER", employeeBranch: "branch-a", wantErr: ErrUnauthorizedBranch},
		{name: "other role is forbidden", role: "SUPPLY_MANAGER", actorBranch: "branch-a", employeeBranch: "branch-a", wantErr: ErrForbiddenAction},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authorizeEmployeeBranch(tt.role, tt.actorBranch, tt.employeeBranch)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("authorizeEmployeeBranch() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
