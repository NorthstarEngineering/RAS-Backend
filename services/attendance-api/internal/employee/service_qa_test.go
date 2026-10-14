package employee

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type employeeQARepo struct {
	employee     *Employee
	created      *Employee
	getErr       error
	exists       bool
	existsErr    error
	listed       []*Employee
	listErr      error
	activeList   []*Employee
	activeErr    error
	updateErr    error
	getCalls     int
	existsCalls  int
	listCalls    int
	activeCalls  int
	updateCalls  int
	lastBranchID string
}

func (r *employeeQARepo) Create(_ context.Context, _ *sqlx.Tx, employee *Employee) error {
	r.created = employee
	return nil
}
func (r *employeeQARepo) GetByID(context.Context, string) (*Employee, error) {
	r.getCalls++
	return r.employee, r.getErr
}
func (*employeeQARepo) GetByNationalID(context.Context, string) (*Employee, error) {
	return nil, nil
}
func (r *employeeQARepo) ListByBranch(_ context.Context, branchID string) ([]*Employee, error) {
	r.listCalls++
	r.lastBranchID = branchID
	return r.listed, r.listErr
}
func (r *employeeQARepo) ListActiveByBranch(_ context.Context, branchID string) ([]*Employee, error) {
	r.activeCalls++
	r.lastBranchID = branchID
	return r.activeList, r.activeErr
}
func (r *employeeQARepo) Update(context.Context, *sqlx.Tx, *Employee) error {
	r.updateCalls++
	return r.updateErr
}
func (r *employeeQARepo) ExistsNationalID(context.Context, string, string) (bool, error) {
	r.existsCalls++
	return r.exists, r.existsErr
}

func validEmployeeQAInput(now time.Time) CreateInput {
	return CreateInput{
		Name: "  Amina Hassan  ", Phone: "01234567890", NationalID: "12345678901234",
		BranchID: "branch-a", PaymentType: PaymentMonthly, PaymentAmount: 5000,
		HireDate: now.Add(-24 * time.Hour), ActorID: "actor", ActorRole: "CEO",
	}
}

func TestQAValidateCreateInputBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	valid := validEmployeeQAInput(now)
	svc := NewService(nil, &employeeQARepo{})
	svc.SetClock(func() time.Time { return now })
	for _, tc := range []struct {
		name string
		edit func(*CreateInput)
		want error
	}{
		{name: "name required", edit: func(i *CreateInput) { i.Name = " \t" }, want: ErrNameRequired},
		{name: "branch required", edit: func(i *CreateInput) { i.BranchID = "" }, want: ErrBranchRequired},
		{name: "phone required", edit: func(i *CreateInput) { i.Phone = "" }, want: ErrPhoneRequired},
		{name: "phone too short", edit: func(i *CreateInput) { i.Phone = "123456789" }, want: ErrPhoneInvalid},
		{name: "phone too long", edit: func(i *CreateInput) { i.Phone = "1234567890123456" }, want: ErrPhoneInvalid},
		{name: "phone non-ASCII digit", edit: func(i *CreateInput) { i.Phone = "٠١٢٣٤٥٦٧٨٩" }, want: ErrPhoneInvalid},
		{name: "national ID required", edit: func(i *CreateInput) { i.NationalID = "" }, want: ErrNationalIDRequired},
		{name: "national ID wrong length", edit: func(i *CreateInput) { i.NationalID = "123" }, want: ErrNationalIDInvalid},
		{name: "national ID nonnumeric", edit: func(i *CreateInput) { i.NationalID = "1234567890123x" }, want: ErrNationalIDInvalid},
		{name: "invalid payment type", edit: func(i *CreateInput) { i.PaymentType = "weekly" }, want: ErrInvalidPaymentType},
		{name: "zero payment", edit: func(i *CreateInput) { i.PaymentAmount = 0 }, want: ErrInvalidPaymentAmount},
		{name: "negative payment", edit: func(i *CreateInput) { i.PaymentAmount = -1 }, want: ErrInvalidPaymentAmount},
		{name: "missing hire date", edit: func(i *CreateInput) { i.HireDate = time.Time{} }, want: ErrInvalidHireDate},
		{name: "future hire date", edit: func(i *CreateInput) { i.HireDate = now.Add(time.Second) }, want: ErrInvalidHireDate},
		{name: "invalid email", edit: func(i *CreateInput) { i.Email = employeeQAPtr("bad") }, want: ErrInvalidEmail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := valid
			tc.edit(&input)
			err := svc.validateCreateInput(&input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("validateCreateInput() error = %v, want %v", err, tc.want)
			}
		})
	}

	for _, phone := range []string{"0123456789", "01234567890", "012345678901234"} {
		t.Run("valid phone "+phone, func(t *testing.T) {
			input := valid
			input.Phone = phone
			input.HireDate = now
			if err := svc.validateCreateInput(&input); err != nil {
				t.Fatalf("hire date today and phone %q rejected: %v", phone, err)
			}
		})
	}
}

type employeeQATxConnector struct{}

func (employeeQATxConnector) Connect(context.Context) (driver.Conn, error) {
	return employeeQATxConn{}, nil
}
func (employeeQATxConnector) Driver() driver.Driver { return employeeQATxDriver{} }

type employeeQATxDriver struct{}

func (employeeQATxDriver) Open(string) (driver.Conn, error) {
	return employeeQATxConn{}, nil
}

type employeeQATxConn struct{}

func (employeeQATxConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare not supported")
}
func (employeeQATxConn) Close() error { return nil }
func (employeeQATxConn) Begin() (driver.Tx, error) {
	return employeeQATx{}, nil
}

type employeeQATx struct{}

func (employeeQATx) Commit() error   { return nil }
func (employeeQATx) Rollback() error { return nil }

func newEmployeeQATransactionDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db := sqlx.NewDb(sql.OpenDB(employeeQATxConnector{}), "employee-qa")
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestQACreateUsesBranchManagersOwnBranch(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	repo := &employeeQARepo{}
	svc := NewService(newEmployeeQATransactionDB(t), repo)
	svc.SetClock(func() time.Time { return now })

	input := validEmployeeQAInput(now)
	input.ActorRole = "BRANCH_MANAGER"
	input.ActorBranchID = "manager-branch"
	input.BranchID = "requested-branch"

	created, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if repo.created == nil {
		t.Fatal("repository Create() was not called")
	}
	if repo.created.BranchID != input.ActorBranchID {
		t.Fatalf("repository Create() branch ID = %q, want manager branch %q", repo.created.BranchID, input.ActorBranchID)
	}
	if created != repo.created {
		t.Fatal("Create() result does not match employee passed to repository")
	}
}
func TestQACreateAuthorizationBranchInheritanceAndDuplicateErrors(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		input CreateInput
		repo  *employeeQARepo
		want  error
	}{
		{name: "disallowed role", input: func() CreateInput { i := validEmployeeQAInput(now); i.ActorRole = "SUPPLY_MANAGER"; return i }(), want: ErrForbiddenAction},
		{name: "manager missing branch", input: func() CreateInput {
			i := validEmployeeQAInput(now)
			i.ActorRole = "BRANCH_MANAGER"
			i.ActorBranchID = ""
			return i
		}(), want: ErrBranchRequired},
		{name: "duplicate national ID", input: func() CreateInput {
			i := validEmployeeQAInput(now)
			i.ActorRole = "BRANCH_MANAGER"
			i.BranchID = "attacker-branch"
			i.ActorBranchID = "branch-b"
			return i
		}(), repo: &employeeQARepo{exists: true}, want: ErrNationalIDDuplicate},
		{name: "duplicate lookup error", input: validEmployeeQAInput(now), repo: &employeeQARepo{existsErr: errors.New("lookup failed")}, want: errors.New("lookup failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := tc.repo
			if repo == nil {
				repo = &employeeQARepo{}
			}
			svc := NewService(nil, repo)
			_, err := svc.Create(context.Background(), tc.input)
			if tc.name == "duplicate lookup error" {
				if err == nil || err.Error() != tc.want.Error() {
					t.Fatalf("Create() error = %v, want %v", err, tc.want)
				}
			} else if !errors.Is(err, tc.want) {
				t.Fatalf("Create() error = %v, want %v", err, tc.want)
			}
			if tc.name == "duplicate national ID" && repo.existsCalls != 1 {
				t.Fatalf("ExistsNationalID calls = %d, want 1", repo.existsCalls)
			}
		})
	}
}

func TestQAGetAndListApplyBranchScopeAndPropagateRepositoryResults(t *testing.T) {
	seed := &Employee{ID: "emp", BranchID: "branch-b"}
	repo := &employeeQARepo{employee: seed}
	svc := NewService(nil, repo)
	if _, err := svc.Get(context.Background(), GetInput{EmployeeID: "emp", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-a"}); !errors.Is(err, ErrUnauthorizedBranch) {
		t.Fatalf("cross-branch Get() error = %v, want ErrUnauthorizedBranch", err)
	}
	got, err := svc.Get(context.Background(), GetInput{EmployeeID: "emp", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-b"})
	if err != nil || got != seed {
		t.Fatalf("same-branch Get() = %v, %v; want seeded employee", got, err)
	}
	repo.employee = nil
	if _, err := svc.Get(context.Background(), GetInput{EmployeeID: "missing", ActorRole: "CEO"}); !errors.Is(err, ErrEmployeeNotFound) {
		t.Fatalf("missing Get() error = %v, want ErrEmployeeNotFound", err)
	}
	if _, err := svc.List(context.Background(), ListInput{BranchID: "caller-choice", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-b", ActiveOnly: true}); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if repo.lastBranchID != "branch-b" || repo.activeCalls != 1 || repo.listCalls != 0 {
		t.Fatalf("active list used branch=%q activeCalls=%d allCalls=%d", repo.lastBranchID, repo.activeCalls, repo.listCalls)
	}
}

func TestQAGetAndListPropagateRepositoryErrors(t *testing.T) {
	getErr := errors.New("get employee failed")
	getRepo := &employeeQARepo{getErr: getErr}
	getSvc := NewService(nil, getRepo)
	_, err := getSvc.Get(context.Background(), GetInput{EmployeeID: "emp", ActorRole: "CEO"})
	if !errors.Is(err, getErr) {
		t.Fatalf("Get() error = %v, want wrapped get repository error", err)
	}
	if getRepo.getCalls != 1 {
		t.Fatalf("GetByID() calls = %d, want 1", getRepo.getCalls)
	}

	listErr := errors.New("list employees failed")
	listRepo := &employeeQARepo{listErr: listErr}
	listSvc := NewService(nil, listRepo)
	_, err = listSvc.List(context.Background(), ListInput{ActorRole: "CEO"})
	if !errors.Is(err, listErr) {
		t.Fatalf("List() error = %v, want wrapped list repository error", err)
	}
	if listRepo.listCalls != 1 || listRepo.activeCalls != 0 {
		t.Fatalf("ListByBranch() calls = %d, ListActiveByBranch() calls = %d; want 1 and 0", listRepo.listCalls, listRepo.activeCalls)
	}
}
func TestQAUpdateRejectsInvalidFieldsAndCrossBranchBeforeTransaction(t *testing.T) {
	seed := &Employee{ID: "emp", BranchID: "branch-a", Name: "Original", Phone: "01234567890", PaymentType: PaymentDaily, PaymentAmount: 100}
	badName := " \t"
	badPhone := "abc"
	badEmail := "invalid"
	badType := PaymentType("weekly")
	badAmount := 0.0
	for _, tc := range []struct {
		name  string
		input UpdateInput
		want  error
	}{
		{name: "wrong role", input: UpdateInput{ActorRole: "SUPPLY_MANAGER"}, want: ErrForbiddenAction},
		{name: "cross branch", input: UpdateInput{EmployeeID: "emp", ActorRole: "BRANCH_MANAGER", ActorBranchID: "branch-b"}, want: ErrUnauthorizedBranch},
		{name: "blank name", input: UpdateInput{EmployeeID: "emp", ActorRole: "CEO", Name: &badName}, want: ErrNameRequired},
		{name: "invalid phone", input: UpdateInput{EmployeeID: "emp", ActorRole: "CEO", Phone: &badPhone}, want: ErrPhoneInvalid},
		{name: "invalid email", input: UpdateInput{EmployeeID: "emp", ActorRole: "CEO", Email: &badEmail}, want: ErrInvalidEmail},
		{name: "invalid type", input: UpdateInput{EmployeeID: "emp", ActorRole: "CEO", PaymentType: &badType}, want: ErrInvalidPaymentType},
		{name: "invalid amount", input: UpdateInput{EmployeeID: "emp", ActorRole: "CEO", PaymentAmount: &badAmount}, want: ErrInvalidPaymentAmount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &employeeQARepo{employee: seed}
			_, err := NewService(nil, repo).Update(context.Background(), tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Update() error = %v, want %v", err, tc.want)
			}
			if repo.updateCalls != 0 {
				t.Fatal("invalid update reached repository")
			}
		})
	}
}

func TestQATerminateDateAndStateBoundaries(t *testing.T) {
	hireDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name string
		seed *Employee
		date time.Time
		role string
		want error
	}{
		{name: "already inactive", seed: &Employee{ID: "emp", BranchID: "b", HireDate: hireDate, Active: false}, date: hireDate, role: "CEO", want: ErrAlreadyTerminated},
		{name: "before hire date", seed: &Employee{ID: "emp", BranchID: "b", HireDate: hireDate, Active: true}, date: hireDate.Add(-24 * time.Hour), role: "CEO", want: ErrInvalidTerminationDate},
		{name: "wrong role", seed: &Employee{ID: "emp", BranchID: "b", HireDate: hireDate, Active: true}, date: hireDate, role: "SUPPLY_MANAGER", want: ErrForbiddenAction},
		{name: "cross branch", seed: &Employee{ID: "emp", BranchID: "b", HireDate: hireDate, Active: true}, date: hireDate, role: "BRANCH_MANAGER", want: ErrUnauthorizedBranch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &employeeQARepo{employee: tc.seed}
			_, err := NewService(nil, repo).Terminate(context.Background(), TerminateInput{EmployeeID: "emp", TerminationDate: tc.date, ActorRole: tc.role, ActorBranchID: "other"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Terminate() error = %v, want %v", err, tc.want)
			}
			if repo.updateCalls != 0 {
				t.Fatal("rejected termination reached repository")
			}
		})
	}
}

func employeeQAPtr(s string) *string { return &s }
