package overtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type overtimeQARepo struct {
	record    *Overtime
	getErr    error
	list      []*Overtime
	listErr   error
	createErr error
	updateErr error
	filter    ListFilter
	gets      int
	lists     int
	creates   int
	updates   int
}

func (r *overtimeQARepo) Create(context.Context, *sqlx.Tx, *Overtime) error {
	r.creates++
	return r.createErr
}
func (r *overtimeQARepo) GetByID(context.Context, string) (*Overtime, error) {
	r.gets++
	return r.record, r.getErr
}
func (r *overtimeQARepo) List(_ context.Context, f ListFilter) ([]*Overtime, error) {
	r.lists++
	r.filter = f
	return r.list, r.listErr
}
func (r *overtimeQARepo) Update(context.Context, *sqlx.Tx, *Overtime) error {
	r.updates++
	return r.updateErr
}
func (*overtimeQARepo) ListPayableByBranchDate(context.Context, string, string, string) ([]*Overtime, error) {
	return nil, nil
}

func overtimeQAService(repo *overtimeQARepo, now time.Time) *Service {
	s := NewService(nil, repo)
	s.SetClock(func() time.Time { return now })
	return s
}

func validOvertimeQAInput(now time.Time) CreateInput {
	return CreateInput{
		EmployeeID: "emp", WorkDate: now.Add(-24 * time.Hour), Hours: 1.5, RatePerHour: 10.25,
		Source: SourceManual, ActorID: "actor", ActorRole: "CEO", EmployeeBranchID: "branch-a",
	}
}

func TestQAValidateOvertimeCreateBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	valid := validOvertimeQAInput(now)
	for _, tc := range []struct {
		name string
		edit func(*CreateInput)
		want error
	}{
		{name: "employee required", edit: func(i *CreateInput) { i.EmployeeID = "" }, want: ErrEmployeeRequired},
		{name: "work date required", edit: func(i *CreateInput) { i.WorkDate = time.Time{} }, want: ErrWorkDateRequired},
		{name: "future work date", edit: func(i *CreateInput) { i.WorkDate = now.Add(time.Second) }, want: ErrWorkDateFuture},
		{name: "zero hours", edit: func(i *CreateInput) { i.Hours = 0 }, want: ErrInvalidHours},
		{name: "negative hours", edit: func(i *CreateInput) { i.Hours = -1 }, want: ErrInvalidHours},
		{name: "zero rate", edit: func(i *CreateInput) { i.RatePerHour = 0 }, want: ErrInvalidRate},
		{name: "negative rate", edit: func(i *CreateInput) { i.RatePerHour = -1 }, want: ErrInvalidRate},
		{name: "unsupported source", edit: func(i *CreateInput) { i.Source = "import" }, want: ErrInvalidSource},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.edit(&in)
			if err := overtimeQAService(&overtimeQARepo{}, now).validateCreateInput(&in); !errors.Is(err, tc.want) {
				t.Fatalf("validateCreateInput() error = %v, want %v", err, tc.want)
			}
		})
	}
	for _, source := range []Source{SourceManual, SourceFingerprint} {
		t.Run("valid source "+string(source), func(t *testing.T) {
			in := valid
			in.Source = source
			in.WorkDate = now
			if err := overtimeQAService(&overtimeQARepo{}, now).validateCreateInput(&in); err != nil {
				t.Fatalf("validateCreateInput() rejected today and source %q: %v", source, err)
			}
		})
	}
}

func TestQACreateAuthorizationAndBranchScopePrecedePersistence(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		edit func(*CreateInput)
		want error
	}{
		{name: "disallowed role", edit: func(i *CreateInput) { i.ActorRole = "SUPPLY_MANAGER" }, want: ErrForbiddenAction},
		{name: "manager missing branch claim", edit: func(i *CreateInput) { i.ActorRole = "BRANCH_MANAGER"; i.ActorBranchID = "" }, want: ErrUnauthorizedBranch},
		{name: "manager employee branch mismatch", edit: func(i *CreateInput) { i.ActorRole = "BRANCH_MANAGER"; i.ActorBranchID = "branch-b" }, want: ErrUnauthorizedBranch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := validOvertimeQAInput(now)
			tc.edit(&in)
			repo := &overtimeQARepo{}
			_, err := overtimeQAService(repo, now).Create(context.Background(), in)
			if !errors.Is(err, tc.want) || repo.creates != 0 {
				t.Fatalf("Create() error=%v create calls=%d; want %v and no write", err, repo.creates, tc.want)
			}
		})
	}
}

func TestQAOvertimeGetScopeAndListFilters(t *testing.T) {
	record := &Overtime{ID: "ot", BranchID: "branch-a", Status: StatusPending}
	repo := &overtimeQARepo{record: record, list: []*Overtime{record}}
	svc := overtimeQAService(repo, time.Unix(100, 0))
	if _, err := svc.Get(context.Background(), GetInput{OvertimeID: "ot", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-b"}); !errors.Is(err, ErrUnauthorizedBranch) {
		t.Fatalf("cross-branch Get() error = %v", err)
	}
	if got, err := svc.Get(context.Background(), GetInput{OvertimeID: "ot", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-a"}); err != nil || got != record {
		t.Fatalf("same-branch Get() = %v, %v", got, err)
	}
	repo.record = nil
	if _, err := svc.Get(context.Background(), GetInput{OvertimeID: "missing", ActorRole: "CEO"}); !errors.Is(err, ErrOvertimeNotFound) {
		t.Fatalf("missing Get() error = %v", err)
	}
	filters := ListInput{EmployeeID: "emp", Status: "pending", FromDate: "2026-10-01", ToDate: "2026-10-31", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-a"}
	if _, err := svc.List(context.Background(), filters); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.filter.BranchID != "branch-a" || repo.filter.EmployeeID != filters.EmployeeID || repo.filter.Status != filters.Status || repo.filter.FromDate != filters.FromDate || repo.filter.ToDate != filters.ToDate {
		t.Fatalf("List() filter = %+v", repo.filter)
	}
	filters.ActorRole = "CEO"
	filters.ActorBranchID = "ignored"
	if _, err := svc.List(context.Background(), filters); err != nil || repo.filter.BranchID != "" {
		t.Fatalf("CEO List() filter = %+v err=%v, want all branches", repo.filter, err)
	}
}

func TestQAOvertimeApprovalAndRejectionStateGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		role   string
		reason string
		ot     *Overtime
		want   error
		call   string
	}{
		{name: "approve rejected", role: "CEO", ot: &Overtime{Status: StatusRejected}, want: ErrCannotApproveRejected, call: "approve"},
		{name: "approve approved", role: "CEO", ot: &Overtime{Status: StatusApproved}, want: ErrAlreadyApproved, call: "approve"},
		{name: "reject approved", role: "CEO", reason: "duplicate entry", ot: &Overtime{Status: StatusApproved}, want: ErrCannotRejectApproved, call: "reject"},
		{name: "reject rejected", role: "CEO", reason: "duplicate entry", ot: &Overtime{Status: StatusRejected}, want: ErrAlreadyRejected, call: "reject"},
		{name: "reject blank reason", role: "CEO", ot: &Overtime{Status: StatusPending}, want: ErrRejectionReasonRequired, call: "reject"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &overtimeQARepo{record: tc.ot}
			svc := overtimeQAService(repo, time.Unix(100, 0))
			var err error
			if tc.call == "approve" {
				_, err = svc.Approve(context.Background(), ApproveInput{OvertimeID: "ot", ActorRole: tc.role})
			} else {
				_, err = svc.Reject(context.Background(), RejectInput{OvertimeID: "ot", ActorRole: tc.role, Reason: tc.reason})
			}
			if !errors.Is(err, tc.want) || repo.updates != 0 {
				t.Fatalf("transition error=%v updates=%d; want %v and no write", err, repo.updates, tc.want)
			}
		})
	}
}

func TestQAOvertimeGetAndListPropagateRepositoryErrors(t *testing.T) {
	getErr := errors.New("get overtime failed")
	getRepo := &overtimeQARepo{getErr: getErr}
	getSvc := overtimeQAService(getRepo, time.Unix(100, 0))
	_, err := getSvc.Get(context.Background(), GetInput{OvertimeID: "ot", ActorRole: "CEO"})
	if !errors.Is(err, getErr) {
		t.Fatalf("Get() error = %v, want repository error", err)
	}
	if getRepo.gets != 1 {
		t.Fatalf("GetByID() calls = %d, want 1", getRepo.gets)
	}

	listErr := errors.New("list overtime failed")
	listRepo := &overtimeQARepo{listErr: listErr}
	listSvc := overtimeQAService(listRepo, time.Unix(100, 0))
	_, err = listSvc.List(context.Background(), ListInput{ActorRole: "CEO"})
	if !errors.Is(err, listErr) {
		t.Fatalf("List() error = %v, want repository error", err)
	}
	if listRepo.lists != 1 {
		t.Fatalf("List() calls = %d, want 1", listRepo.lists)
	}
}
func TestQAOvertimePureDateRoundingAndPayableHelpers(t *testing.T) {
	loc := time.FixedZone("qa", 3*60*60)
	date := time.Date(2026, 10, 13, 14, 30, 0, 0, loc)
	gotDate := truncateToDate(date)
	if gotDate.Hour() != 0 || gotDate.Minute() != 0 || gotDate.Location() != loc || gotDate.Day() != date.Day() {
		t.Fatalf("truncateToDate(%v) = %v", date, gotDate)
	}
	if got := round2(2.345); got != 2.35 {
		t.Fatalf("round2(2.345) = %v, want 2.35", got)
	}
	for _, tc := range []struct {
		status Status
		want   bool
	}{
		{status: StatusPending, want: false},
		{status: StatusApproved, want: true},
		{status: StatusRejected, want: false},
	} {
		o := &Overtime{Status: tc.status}
		if o.IsPayable() != tc.want || o.IsPending() != (tc.status == StatusPending) {
			t.Fatalf("status %q payable=%v pending=%v", tc.status, o.IsPayable(), o.IsPending())
		}
	}
}
