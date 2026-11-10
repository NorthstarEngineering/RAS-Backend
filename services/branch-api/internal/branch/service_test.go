package branch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type serviceTestClock struct{ now time.Time }

func (c serviceTestClock) Now() time.Time { return c.now }

type serviceTestRepository struct {
	branches []Branch
	created  []Branch
	updated  []Branch
	getErr   error
}

func (r *serviceTestRepository) Create(_ context.Context, branch Branch) error {
	r.created = append(r.created, branch)
	return nil
}

func (r *serviceTestRepository) GetByID(_ context.Context, id uuid.UUID) (Branch, error) {
	if r.getErr != nil {
		return Branch{}, r.getErr
	}
	for _, branch := range r.branches {
		if branch.ID == id {
			return branch, nil
		}
	}
	return Branch{}, ErrNotFound
}

func (r *serviceTestRepository) List(context.Context) ([]Branch, error) {
	return r.branches, nil
}

func (r *serviceTestRepository) Update(_ context.Context, branch Branch) error {
	r.updated = append(r.updated, branch)
	return nil
}

func TestCreateValidatesAndNormalizesBranchInput(t *testing.T) {
	now := time.Date(2026, 10, 25, 3, 0, 0, 0, time.UTC)
	rate := decimal.RequireFromString("125.50")
	repo := &serviceTestRepository{}
	service := NewService(repo, serviceTestClock{now: now})

	created, err := service.Create(context.Background(), CreateBranchRequest{
		Name:              "  Downtown  ",
		Type:              string(TypeBranch),
		Address:           "  Main Street  ",
		OfficialStartTime: " 9:05 ",
		LateRate:          &rate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Downtown" || created.Address != "Main Street" || created.OfficialStartTime != "09:05" {
		t.Fatalf("Create() did not normalize fields: %+v", created)
	}
	if !created.IsActive || !created.CreatedAt.Equal(now) || !created.UpdatedAt.Equal(now) {
		t.Fatalf("Create() metadata is incorrect: %+v", created)
	}
	if len(repo.created) != 1 || repo.created[0].ID != created.ID {
		t.Fatalf("Create() did not persist the created branch: %+v", repo.created)
	}
}

func TestCreateRejectsInvalidBranchRequests(t *testing.T) {
	tooLong := ""
	for i := 0; i < maxNameLength+1; i++ {
		tooLong += "a"
	}
	validRate := decimal.RequireFromString("10.00")
	negativeRate := decimal.RequireFromString("-1")
	preciseRate := decimal.RequireFromString("1.001")
	largeRate := decimal.RequireFromString("10000.01")

	tests := []struct {
		name string
		req  CreateBranchRequest
		want error
	}{
		{"missing name", CreateBranchRequest{Type: string(TypeBranch), LateRate: &validRate}, ErrNameRequired},
		{"name too long", CreateBranchRequest{Name: tooLong, Type: string(TypeBranch), LateRate: &validRate}, ErrNameTooLong},
		{"invalid type", CreateBranchRequest{Name: "Branch", Type: "warehouse", LateRate: &validRate}, ErrInvalidType},
		{"invalid time", CreateBranchRequest{Name: "Branch", Type: string(TypeBranch), OfficialStartTime: "9am", LateRate: &validRate}, ErrInvalidTime},
		{"missing late rate", CreateBranchRequest{Name: "Branch", Type: string(TypeBranch), OfficialStartTime: "09:00"}, ErrLateRateRequired},
		{"negative late rate", CreateBranchRequest{Name: "Branch", Type: string(TypeBranch), OfficialStartTime: "09:00", LateRate: &negativeRate}, ErrInvalidLateRate},
		{"late rate too precise", CreateBranchRequest{Name: "Branch", Type: string(TypeBranch), OfficialStartTime: "09:00", LateRate: &preciseRate}, ErrLateRatePrecision},
		{"late rate too large", CreateBranchRequest{Name: "Branch", Type: string(TypeBranch), OfficialStartTime: "09:00", LateRate: &largeRate}, ErrLateRateTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(&serviceTestRepository{}, serviceTestClock{}).Create(context.Background(), test.req)
			if !errors.Is(err, test.want) {
				t.Fatalf("Create() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestUpdateOnlyPersistsActualChanges(t *testing.T) {
	now := time.Date(2026, 10, 25, 3, 0, 0, 0, time.UTC)
	originalUpdate := now.Add(-time.Hour)
	id := uuid.New()
	rate := decimal.RequireFromString("20.00")
	repo := &serviceTestRepository{branches: []Branch{{
		ID: id, Name: "Old Name", Type: TypeBranch, Address: "Old Address",
		OfficialStartTime: "09:00", LateRate: decimal.RequireFromString("10.00"),
		IsActive: true, CreatedAt: now.Add(-time.Hour), UpdatedAt: originalUpdate,
	}}}
	service := NewService(repo, serviceTestClock{now: now})

	unchanged, err := service.Update(context.Background(), id, UpdateBranchRequest{})
	if err != nil {
		t.Fatalf("Update() unchanged error = %v", err)
	}
	if len(repo.updated) != 0 || !unchanged.UpdatedAt.Equal(originalUpdate) {
		t.Fatalf("unchanged Update() should not persist or change timestamp: %+v", unchanged)
	}

	updated, err := service.Update(context.Background(), id, UpdateBranchRequest{
		Name:              stringPointer("  New Name "),
		Address:           stringPointer(" New Address "),
		OfficialStartTime: stringPointer("10:30"),
		LateRate:          &rate,
		IsActive:          boolPointer(false),
	})
	if err != nil {
		t.Fatalf("Update() changed error = %v", err)
	}
	if len(repo.updated) != 1 || updated.Name != "New Name" || updated.Address != "New Address" ||
		updated.OfficialStartTime != "10:30" || !updated.LateRate.Equal(rate) || updated.IsActive ||
		!updated.UpdatedAt.Equal(now) {
		t.Fatalf("Update() produced incorrect branch: %+v", updated)
	}
}

func TestBranchTypeRevenueRules(t *testing.T) {
	if !TypeBranch.GeneratesRevenue() {
		t.Fatal("regular branches should generate revenue")
	}

	func TestNormalizeTimeRejectsBoundaryFormats(t *testing.T) {
		for _, value := range []string{"24:00", "12:60", "12", "noon"} {
			if _, err := normalizeTime(value); !errors.Is(err, ErrInvalidTime) {
				t.Fatalf("normalizeTime(%q) error = %v, want ErrInvalidTime", value, err)
			}
		}
	}

	func TestValidateLateRateAcceptsZeroAndTwoDecimals(t *testing.T) {
		for _, value := range []string{"0", "0.01", "10000.00"} {
			if err := validateLateRate(decimal.RequireFromString(value)); err != nil {
				t.Fatalf("validateLateRate(%q) error = %v", value, err)
			}
		}
	}

	func TestValidateNameTrimsUnicodeAndCountsRunes(t *testing.T) {
		name, err := validateName("  مطعم  ")
		if err != nil || name != "مطعم" {
			t.Fatalf("validateName() = %q, %v", name, err)
		}
	}

	func TestUpdatePropagatesRepositoryErrors(t *testing.T) {
		repo := &serviceTestRepository{getErr: errors.New("database unavailable")}
		_, err := NewService(repo, serviceTestClock{}).Update(context.Background(), uuid.New(), UpdateBranchRequest{})
		if err == nil || !errors.Is(err, repo.getErr) {
			t.Fatalf("Update() error = %v, want repository error", err)
		}
	}
	if TypeCentralKitchen.GeneratesRevenue() {
		t.Fatal("central kitchens should not generate revenue")
	}
}

func stringPointer(value string) *string { return &value }

func boolPointer(value bool) *bool { return &value }
