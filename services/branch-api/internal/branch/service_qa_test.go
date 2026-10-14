package branch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type qaRepository struct {
	created     Branch
	getBranch   Branch
	listed      []Branch
	updated     Branch
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	createCalls int
	getCalls    int
	listCalls   int
	updateCalls int
}

func (r *qaRepository) Create(_ context.Context, b Branch) error {
	r.createCalls++
	r.created = b
	return r.createErr
}

func (r *qaRepository) GetByID(_ context.Context, _ uuid.UUID) (Branch, error) {
	r.getCalls++
	return r.getBranch, r.getErr
}

func (r *qaRepository) List(_ context.Context) ([]Branch, error) {
	r.listCalls++
	return r.listed, r.listErr
}

func (r *qaRepository) Update(_ context.Context, b Branch) error {
	r.updateCalls++
	r.updated = b
	return r.updateErr
}

type qaClock struct{ now time.Time }

func (c qaClock) Now() time.Time { return c.now }

func qaDecimal(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func validCreateRequest() CreateBranchRequest {
	rate := qaDecimal("12.34")
	return CreateBranchRequest{
		Name:              "North",
		Type:              string(TypeBranch),
		Address:           "Main Street",
		OfficialStartTime: "09:00",
		LateRate:          &rate,
	}
}

func TestQACreateValidation(t *testing.T) {
	negative := qaDecimal("-0.01")
	tooLarge := qaDecimal("10000.01")
	tooPrecise := qaDecimal("1.001")
	largeExponent := decimal.New(1, 21)
	smallExponent := decimal.New(1, -21)

	tests := []struct {
		name string
		edit func(*CreateBranchRequest)
		want error
	}{
		{name: "blank name", edit: func(r *CreateBranchRequest) { r.Name = " \t\n " }, want: ErrNameRequired},
		{name: "name over 100 Unicode runes", edit: func(r *CreateBranchRequest) { r.Name = strings.Repeat("界", 101) }, want: ErrNameTooLong},
		{name: "invalid type", edit: func(r *CreateBranchRequest) { r.Type = "warehouse" }, want: ErrInvalidType},
		{name: "invalid time", edit: func(r *CreateBranchRequest) { r.OfficialStartTime = "25:61" }, want: ErrInvalidTime},
		{name: "missing late rate", edit: func(r *CreateBranchRequest) { r.LateRate = nil }, want: ErrLateRateRequired},
		{name: "negative late rate", edit: func(r *CreateBranchRequest) { r.LateRate = &negative }, want: ErrInvalidLateRate},
		{name: "late rate above configured maximum", edit: func(r *CreateBranchRequest) { r.LateRate = &tooLarge }, want: ErrLateRateTooLarge},
		{name: "late rate with excess precision", edit: func(r *CreateBranchRequest) { r.LateRate = &tooPrecise }, want: ErrLateRatePrecision},
		{name: "extreme positive exponent", edit: func(r *CreateBranchRequest) { r.LateRate = &largeExponent }, want: ErrLateRateTooLarge},
		{name: "extreme negative exponent", edit: func(r *CreateBranchRequest) { r.LateRate = &smallExponent }, want: ErrLateRatePrecision},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &qaRepository{}
			req := validCreateRequest()
			tt.edit(&req)
			_, err := NewService(repo, qaClock{now: time.Unix(1, 0)}).Create(context.Background(), req)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Create() error = %v, want %v", err, tt.want)
			}
			if repo.createCalls != 0 {
				t.Fatalf("repository Create called %d times for invalid request", repo.createCalls)
			}
		})
	}
}

func TestQACreateAcceptsUnicodeNameBoundaryAndLateRateLimits(t *testing.T) {
	for _, tc := range []struct {
		name string
		rate string
	}{
		{name: strings.Repeat("界", 100), rate: "0"},
		{name: "Boundary", rate: "10000"},
	} {
		t.Run(tc.rate, func(t *testing.T) {
			repo := &qaRepository{}
			req := validCreateRequest()
			req.Name = tc.name
			rate := qaDecimal(tc.rate)
			req.LateRate = &rate
			if _, err := NewService(repo, qaClock{now: time.Unix(10, 0)}).Create(context.Background(), req); err != nil {
				t.Fatalf("Create() rejected boundary values: %v", err)
			}
			if repo.createCalls != 1 {
				t.Fatalf("repository Create calls = %d, want 1", repo.createCalls)
			}
		})
	}
}

func TestQACreateNormalizesAndAppliesDefaults(t *testing.T) {
	now := time.Date(2025, 3, 4, 5, 6, 7, 0, time.FixedZone("test", 2*60*60))
	repo := &qaRepository{}
	req := validCreateRequest()
	req.Name = "  Riverside  "
	req.Address = "  5 River Road  "
	req.OfficialStartTime = "9:05"

	created, err := NewService(repo, qaClock{now: now}).Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Name != "Riverside" || created.Address != "5 River Road" {
		t.Fatalf("Create() did not trim fields: name=%q address=%q", created.Name, created.Address)
	}
	if created.OfficialStartTime != "09:05" {
		t.Fatalf("normalized official start time = %q, want 09:05", created.OfficialStartTime)
	}
	if !created.IsActive {
		t.Fatal("new branch IsActive = false, want true")
	}
	if !created.CreatedAt.Equal(now) || !created.UpdatedAt.Equal(now) {
		t.Fatalf("create timestamps = (%v, %v), want (%v, %v)", created.CreatedAt, created.UpdatedAt, now, now)
	}
	if repo.created.ID != created.ID || !repo.created.CreatedAt.Equal(created.CreatedAt) {
		t.Fatal("repository did not receive the created branch")
	}
}

func TestQACreateSupportsBothDefinedBranchTypes(t *testing.T) {
	for _, branchType := range []BranchType{TypeBranch, TypeCentralKitchen} {
		t.Run(string(branchType), func(t *testing.T) {
			req := validCreateRequest()
			req.Type = string(branchType)
			if _, err := NewService(&qaRepository{}, qaClock{now: time.Unix(1, 0)}).Create(context.Background(), req); err != nil {
				t.Fatalf("Create() rejected defined type %q: %v", branchType, err)
			}
		})
	}
}

func TestQACreatePropagatesRepositoryError(t *testing.T) {
	want := errors.New("create unavailable")
	repo := &qaRepository{createErr: want}
	_, err := NewService(repo, qaClock{now: time.Unix(1, 0)}).Create(context.Background(), validCreateRequest())
	if !errors.Is(err, want) {
		t.Fatalf("Create() error = %v, want repository error %v", err, want)
	}
}

func qaExistingBranch() Branch {
	return Branch{
		ID:                uuid.New(),
		Name:              "North",
		Type:              TypeBranch,
		Address:           "Old address",
		OfficialStartTime: "08:30",
		LateRate:          qaDecimal("10.00"),
		IsActive:          true,
		CreatedAt:         time.Unix(100, 0),
		UpdatedAt:         time.Unix(200, 0),
	}
}

func TestQAUpdatePreservesOmittedFieldsAndPersistsExplicitFalse(t *testing.T) {
	now := time.Unix(300, 0)
	seed := qaExistingBranch()
	address := "  New address  "
	active := false
	repo := &qaRepository{getBranch: seed}

	updated, err := NewService(repo, qaClock{now: now}).Update(context.Background(), seed.ID, UpdateBranchRequest{
		Address:  &address,
		IsActive: &active,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Address != "New address" || updated.IsActive {
		t.Fatalf("updated fields = address %q, active %v", updated.Address, updated.IsActive)
	}
	if updated.Name != seed.Name || updated.Type != seed.Type || updated.OfficialStartTime != seed.OfficialStartTime || !updated.LateRate.Equal(seed.LateRate) {
		t.Fatalf("omitted fields changed: got %+v, seed %+v", updated, seed)
	}
	if !updated.UpdatedAt.Equal(now) || repo.updateCalls != 1 {
		t.Fatalf("updated_at=%v update calls=%d; want %v and 1", updated.UpdatedAt, repo.updateCalls, now)
	}
	if repo.updated.Address != "New address" || repo.updated.IsActive {
		t.Fatalf("repository did not receive trimmed address and explicit false: %+v", repo.updated)
	}
}

func TestQAUpdateNoOpDoesNotWriteOrChangeTimestamp(t *testing.T) {
	seed := qaExistingBranch()
	name := " North "
	address := " Old address "
	start := "8:30"
	rate := qaDecimal("10")
	active := true
	repo := &qaRepository{getBranch: seed}

	got, err := NewService(repo, qaClock{now: time.Unix(999, 0)}).Update(context.Background(), seed.ID, UpdateBranchRequest{
		Name: &name, Address: &address, OfficialStartTime: &start, LateRate: &rate, IsActive: &active,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("repository Update calls = %d, want 0", repo.updateCalls)
	}
	if !got.UpdatedAt.Equal(seed.UpdatedAt) {
		t.Fatalf("no-op UpdatedAt = %v, want unchanged %v", got.UpdatedAt, seed.UpdatedAt)
	}
}

func TestQAUpdateValidationAndRepositoryErrors(t *testing.T) {
	seed := qaExistingBranch()
	invalidName := strings.Repeat("x", 101)
	invalidTime := "24:00"
	invalidRate := qaDecimal("-1")
	updateErr := errors.New("update unavailable")
	tests := []struct {
		name       string
		getErr     error
		updateErr  error
		request    UpdateBranchRequest
		want       error
		wantWrites int
	}{
		{name: "get failure", getErr: errors.New("read unavailable"), want: nil},
		{name: "invalid name", request: UpdateBranchRequest{Name: &invalidName}, want: ErrNameTooLong},
		{name: "invalid time", request: UpdateBranchRequest{OfficialStartTime: &invalidTime}, want: ErrInvalidTime},
		{name: "invalid rate", request: UpdateBranchRequest{LateRate: &invalidRate}, want: ErrInvalidLateRate},
		{name: "update failure", updateErr: updateErr, request: UpdateBranchRequest{Name: stringPointer("South")}, want: updateErr, wantWrites: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getErr := tt.getErr
			if tt.name == "get failure" {
				getErr = updateErr
			}
			repo := &qaRepository{getBranch: seed, getErr: getErr, updateErr: tt.updateErr}
			_, err := NewService(repo, qaClock{now: time.Unix(500, 0)}).Update(context.Background(), seed.ID, tt.request)
			want := tt.want
			if tt.name == "get failure" {
				want = updateErr
			}
			if !errors.Is(err, want) {
				t.Fatalf("Update() error = %v, want %v", err, want)
			}
			if repo.updateCalls != tt.wantWrites {
				t.Fatalf("repository Update calls = %d, want %d", repo.updateCalls, tt.wantWrites)
			}
		})
	}
}

func TestQAGetAndListPropagateRepositoryErrors(t *testing.T) {
	want := errors.New("repository unavailable")
	id := uuid.New()
	repo := &qaRepository{getErr: want, listErr: want}
	svc := NewService(repo, qaClock{now: time.Unix(1, 0)})
	if _, err := svc.GetByID(context.Background(), id); !errors.Is(err, want) {
		t.Fatalf("GetByID() error = %v, want %v", err, want)
	}
	if _, err := svc.List(context.Background()); !errors.Is(err, want) {
		t.Fatalf("List() error = %v, want %v", err, want)
	}
}

func stringPointer(s string) *string { return &s }
