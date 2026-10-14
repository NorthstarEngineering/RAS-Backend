package attendance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type attendanceQARepo struct {
	openRecord      *AttendanceRecord
	todayRecord     *AttendanceRecord
	recordByID      *AttendanceRecord
	deduction       *Deduction
	openErr         error
	todayErr        error
	recordErr       error
	deductionErr    error
	createErr       error
	checkoutErr     error
	createDedErr    error
	updateDedErr    error
	openReads       int
	todayReads      int
	createRecordN   int
	updateCheckoutN int
	createDedN      int
	updateDedN      int
}

func (r *attendanceQARepo) GetOpenRecord(context.Context, string) (*AttendanceRecord, error) {
	r.openReads++
	return r.openRecord, r.openErr
}
func (r *attendanceQARepo) GetRecordByDate(context.Context, string, time.Time) (*AttendanceRecord, error) {
	r.todayReads++
	return r.todayRecord, r.todayErr
}
func (r *attendanceQARepo) GetRecordByID(context.Context, string) (*AttendanceRecord, error) {
	return r.recordByID, r.recordErr
}
func (r *attendanceQARepo) CreateRecord(context.Context, *sqlx.Tx, *AttendanceRecord) error {
	r.createRecordN++
	return r.createErr
}
func (r *attendanceQARepo) UpdateCheckOut(context.Context, *sqlx.Tx, string, time.Time) error {
	r.updateCheckoutN++
	return r.checkoutErr
}
func (r *attendanceQARepo) CreateDeduction(_ context.Context, _ *sqlx.Tx, d *Deduction) error {
	r.createDedN++
	r.deduction = d
	return r.createDedErr
}
func (r *attendanceQARepo) GetDeductionByID(context.Context, string) (*Deduction, error) {
	return r.deduction, r.deductionErr
}
func (r *attendanceQARepo) GetDeductionByAttendance(context.Context, string) (*Deduction, error) {
	return nil, nil
}
func (r *attendanceQARepo) UpdateDeduction(context.Context, *sqlx.Tx, *Deduction) error {
	r.updateDedN++
	return r.updateDedErr
}

type attendanceQAEmployee struct {
	info *EmployeeInfo
	err  error
	gets int
}

func (r *attendanceQAEmployee) GetByID(context.Context, string) (*EmployeeInfo, error) {
	r.gets++
	return r.info, r.err
}

type attendanceQABranch struct {
	info *BranchInfo
	err  error
	gets int
}

func (r *attendanceQABranch) GetByID(context.Context, string) (*BranchInfo, error) {
	r.gets++
	return r.info, r.err
}

func newAttendanceQAService(repo *attendanceQARepo, emp *attendanceQAEmployee, br *attendanceQABranch, now time.Time) *Service {
	s := NewService(nil, repo, emp, br)
	s.SetClock(func() time.Time { return now })
	return s
}

func TestQACheckInRejectsInvalidInputAndDependenciesBeforeTransaction(t *testing.T) {
	errReader := errors.New("employee lookup unavailable")
	tests := []struct {
		name        string
		input       CheckInInput
		employee    *EmployeeInfo
		employeeErr error
		want        error
		wantText    string
	}{
		{name: "missing employee id", input: CheckInInput{Source: SourceManual}, want: ErrEmployeeNotFound},
		{name: "unsupported source", input: CheckInInput{EmployeeID: "emp", Source: "import"}, wantText: "invalid source"},
		{name: "employee lookup error", input: CheckInInput{EmployeeID: "emp", Source: SourceManual}, employeeErr: errReader, want: errReader},
		{name: "employee missing", input: CheckInInput{EmployeeID: "emp", Source: SourceManual}, want: ErrEmployeeNotFound},
		{name: "employee inactive", input: CheckInInput{EmployeeID: "emp", Source: SourceManual}, employee: &EmployeeInfo{ID: "emp", BranchID: "b1"}, want: ErrEmployeeInactive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &attendanceQARepo{}
			employee := &attendanceQAEmployee{info: tt.employee, err: tt.employeeErr}
			_, err := newAttendanceQAService(repo, employee, &attendanceQABranch{}, time.Unix(1, 0)).CheckIn(context.Background(), tt.input)
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("CheckIn() error = %v, want %v", err, tt.want)
			}
			if tt.wantText != "" && (err == nil || !strings.Contains(err.Error(), tt.wantText)) {
				t.Fatalf("CheckIn() error = %v, want text containing %q", err, tt.wantText)
			}
			if repo.openReads != 0 || repo.createRecordN != 0 {
				t.Fatalf("attendance repository touched before validation completed: %+v", repo)
			}
		})
	}
}

func TestQACheckInRejectsMissingCentralKitchenAndDuplicateBranches(t *testing.T) {
	active := &EmployeeInfo{ID: "emp", BranchID: "branch-a", Active: true}
	tests := []struct {
		name      string
		branch    *BranchInfo
		branchErr error
		repo      *attendanceQARepo
		want      error
	}{
		{name: "branch reader error", branchErr: errors.New("branch lookup unavailable"), want: errors.New("get branch: branch lookup unavailable")},
		{name: "branch missing", want: ErrBranchNotFound},
		{name: "central kitchen", branch: &BranchInfo{ID: "branch-a", OfficialStartTime: "09:00", IsCentralKitchen: true}, want: ErrCentralKitchenNotAllowed},
		{name: "open record", branch: &BranchInfo{ID: "branch-a", OfficialStartTime: "09:00"}, repo: &attendanceQARepo{openRecord: &AttendanceRecord{ID: "existing"}}, want: ErrAlreadyCheckedIn},
		{name: "record already exists for work date", branch: &BranchInfo{ID: "branch-a", OfficialStartTime: "09:00"}, repo: &attendanceQARepo{todayRecord: &AttendanceRecord{ID: "existing"}}, want: ErrAlreadyCheckedIn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.repo
			if repo == nil {
				repo = &attendanceQARepo{}
			}
			_, err := newAttendanceQAService(repo, &attendanceQAEmployee{info: active}, &attendanceQABranch{info: tt.branch, err: tt.branchErr}, time.Unix(1, 0)).CheckIn(context.Background(), CheckInInput{EmployeeID: "emp", ActorRole: "CEO", Source: SourceManual})
			if tt.name == "branch reader error" {
				if err == nil || !strings.Contains(err.Error(), "get branch: branch lookup unavailable") {
					t.Fatalf("CheckIn() error = %v, want wrapped branch lookup error", err)
				}
			} else if !errors.Is(err, tt.want) {
				t.Fatalf("CheckIn() error = %v, want %v", err, tt.want)
			}
			if repo.createRecordN != 0 {
				t.Fatal("CheckIn() attempted record creation before rejection")
			}
		})
	}
}

func TestQACalculateDeductionCompletedHourBoundaries(t *testing.T) {
	base := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name          string
		checkIn       time.Time
		wantLate      int
		wantHours     int
		wantAmount    float64
		wantDeduction bool
	}{
		{name: "before start", checkIn: base.Add(8*time.Hour + 59*time.Minute)},
		{name: "at start", checkIn: base.Add(9 * time.Hour)},
		{name: "59 minutes late", checkIn: base.Add(9*time.Hour + 59*time.Minute), wantLate: 59},
		{name: "one completed hour", checkIn: base.Add(10 * time.Hour), wantLate: 60, wantHours: 1, wantAmount: 10.01, wantDeduction: true},
		{name: "119 minutes late", checkIn: base.Add(10*time.Hour + 59*time.Minute), wantLate: 119, wantHours: 1, wantAmount: 10.01, wantDeduction: true},
		{name: "two completed hours", checkIn: base.Add(11 * time.Hour), wantLate: 120, wantHours: 2, wantAmount: 20.02, wantDeduction: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &attendanceQARepo{}
			svc := newAttendanceQAService(repo, &attendanceQAEmployee{}, &attendanceQABranch{}, tt.checkIn.Add(time.Hour))
			rec := &AttendanceRecord{ID: "record", EmployeeID: "emp", BranchID: "branch", WorkDate: truncateToDate(tt.checkIn), CheckInAt: tt.checkIn}
			deduction, err := svc.calculateDeduction(context.Background(), nil, rec, &BranchInfo{OfficialStartTime: "09:00", LatenessRatePerHour: 10.01})
			if err != nil {
				t.Fatalf("calculateDeduction() error = %v", err)
			}
			if !tt.wantDeduction {
				if deduction != nil || repo.createDedN != 0 {
					t.Fatalf("deduction = %+v, CreateDeduction calls=%d, want none", deduction, repo.createDedN)
				}
				return
			}
			if deduction == nil || deduction.LateMinutes != tt.wantLate || deduction.CompletedHours != tt.wantHours || deduction.OriginalAmount != tt.wantAmount || deduction.Source != "automatic" {
				t.Fatalf("deduction = %+v; want late=%d hours=%d amount=%v automatic", deduction, tt.wantLate, tt.wantHours, tt.wantAmount)
			}
			if repo.createDedN != 1 {
				t.Fatalf("CreateDeduction calls = %d, want 1", repo.createDedN)
			}
		})
	}
}

func TestQACalculateDeductionRejectsInvalidOfficialTime(t *testing.T) {
	svc := newAttendanceQAService(&attendanceQARepo{}, &attendanceQAEmployee{}, &attendanceQABranch{}, time.Unix(1, 0))
	rec := &AttendanceRecord{CheckInAt: time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)}
	if _, err := svc.calculateDeduction(context.Background(), nil, rec, &BranchInfo{OfficialStartTime: "25:00"}); err == nil || !strings.Contains(err.Error(), "parse official start time") {
		t.Fatalf("calculateDeduction() error = %v, want invalid official-time error", err)
	}
}

func TestQACheckOutRejectsMissingEmployeeOrOpenRecord(t *testing.T) {
	for _, tc := range []struct {
		name       string
		employeeID string
		emp        *EmployeeInfo
		repo       *attendanceQARepo
		want       error
	}{
		{name: "empty employee ID", employeeID: "", repo: &attendanceQARepo{}, want: ErrEmployeeNotFound},
		{name: "employee absent", employeeID: "emp", repo: &attendanceQARepo{}, want: ErrNotCheckedIn},
		{name: "no open record", employeeID: "emp", emp: &EmployeeInfo{ID: "emp", BranchID: "b1", Active: true}, repo: &attendanceQARepo{}, want: ErrNotCheckedIn},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newAttendanceQAService(tc.repo, &attendanceQAEmployee{info: tc.emp}, &attendanceQABranch{}, time.Unix(1, 0)).CheckOut(context.Background(), CheckOutInput{EmployeeID: tc.employeeID, ActorRole: "CEO"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("CheckOut() error = %v, want %v", err, tc.want)
			}
			if tc.repo.updateCheckoutN != 0 {
				t.Fatal("CheckOut() attempted update without open record")
			}
		})
	}
}

func TestQADeductionModelEffectiveAmount(t *testing.T) {
	adjusted := 0.0
	for _, tc := range []struct {
		name string
		d    Deduction
		want float64
	}{
		{name: "original amount", d: Deduction{OriginalAmount: 12.5}, want: 12.5},
		{name: "adjusted zero", d: Deduction{OriginalAmount: 12.5, AdjustedAmount: &adjusted}, want: 0},
		{name: "cancel overrides adjustment", d: Deduction{OriginalAmount: 12.5, AdjustedAmount: &adjusted, Cancelled: true}, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.d.EffectiveAmount(); got != tc.want {
				t.Fatalf("EffectiveAmount() = %v, want %v", got, tc.want)
			}
		})
	}
}
