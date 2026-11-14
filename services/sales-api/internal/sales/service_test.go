package sales

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type sqlTestState struct {
	beginErr, commitErr        error
	begins, commits, rollbacks int
}
type sqlTestConnector struct{ state *sqlTestState }

func (c sqlTestConnector) Connect(context.Context) (driver.Conn, error) {
	return sqlTestConn{state: c.state}, nil
}
func (c sqlTestConnector) Driver() driver.Driver { return sqlTestDriver{} }

type sqlTestDriver struct{}

func (sqlTestDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type sqlTestConn struct{ state *sqlTestState }

func (c sqlTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c sqlTestConn) Close() error                        { return nil }
func (c sqlTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c sqlTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.begins++
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	return &sqlTestTx{state: c.state}, nil
}

type sqlTestTx struct{ state *sqlTestState }

func (tx *sqlTestTx) Commit() error   { tx.state.commits++; return tx.state.commitErr }
func (tx *sqlTestTx) Rollback() error { tx.state.rollbacks++; return nil }

var registerTestDriver sync.Once

func newTestDB(t *testing.T, state *sqlTestState) *sqlx.DB {
	t.Helper()
	registerTestDriver.Do(func() { sql.Register("sales-unit-test", sqlTestDriver{}) })
	// sql.OpenDB uses this connector directly; Driver is only needed to satisfy Connector.
	db := sqlx.NewDb(sql.OpenDB(sqlTestConnector{state: state}), "postgres")
	t.Cleanup(func() { _ = db.Close() })
	return db
}

type fakeRepo struct {
	Repository
	shifts                                       map[string]*Shift
	discounts                                    map[string]*Discount
	prices                                       map[string]float64
	priceErr                                     map[string]error
	errs                                         map[string]error
	createdShift                                 *Shift
	updatedShiftID, updatedShiftStatus           string
	createdSale                                  *Sale
	createdItems                                 []*SaleItem
	createdDiscount                              *Discount
	canceledDiscountID, cancelReason, canceledBy string
	channels                                     []ChannelSummary
	items                                        []ItemSummary
	gross, discountsTotal                        float64
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		shifts: map[string]*Shift{}, discounts: map[string]*Discount{},
		prices: map[string]float64{}, priceErr: map[string]error{}, errs: map[string]error{},
	}
}
func (r *fakeRepo) CreateShift(_ context.Context, _ *sqlx.Tx, s *Shift) error {
	r.createdShift = s
	return r.errs["CreateShift"]
}
func (r *fakeRepo) GetShiftByID(_ context.Context, id string) (*Shift, error) {
	if err := r.errs["GetShiftByID"]; err != nil {
		return nil, err
	}
	s := r.shifts[id]
	if s == nil {
		return nil, nil
	}
	copy := *s
	return &copy, nil
}
func (r *fakeRepo) UpdateShiftStatus(_ context.Context, _ *sqlx.Tx, id, status string) error {
	r.updatedShiftID = id
	r.updatedShiftStatus = status
	return r.errs["UpdateShiftStatus"]
}
func (r *fakeRepo) CreateSale(_ context.Context, _ *sqlx.Tx, s *Sale) error {
	r.createdSale = s
	return r.errs["CreateSale"]
}
func (r *fakeRepo) CreateSaleItem(_ context.Context, _ *sqlx.Tx, i *SaleItem) error {
	r.createdItems = append(r.createdItems, i)
	return r.errs["CreateSaleItem"]
}
func (r *fakeRepo) GetShiftSalesTotal(_ context.Context, _ string) (float64, float64, error) {
	return r.gross, r.discountsTotal, r.errs["GetShiftSalesTotal"]
}
func (r *fakeRepo) CreateDiscount(_ context.Context, _ *sqlx.Tx, d *Discount) error {
	r.createdDiscount = d
	return r.errs["CreateDiscount"]
}
func (r *fakeRepo) GetDiscountByID(_ context.Context, id string) (*Discount, error) {
	if err := r.errs["GetDiscountByID"]; err != nil {
		return nil, err
	}
	d := r.discounts[id]
	if d == nil {
		return nil, nil
	}
	copy := *d
	return &copy, nil
}
func (r *fakeRepo) CancelDiscount(_ context.Context, _ *sqlx.Tx, id, reason, actor string) error {
	r.canceledDiscountID = id
	r.cancelReason = reason
	r.canceledBy = actor
	return r.errs["CancelDiscount"]
}
func (r *fakeRepo) GetMenuItemPrice(_ context.Context, id string) (float64, error) {
	if err := r.priceErr[id]; err != nil {
		return 0, err
	}
	p, ok := r.prices[id]
	if !ok {
		return 0, fmt.Errorf("missing menu item")
	}
	return p, nil
}
func (r *fakeRepo) GetShiftChannelSummary(_ context.Context, _ string) ([]ChannelSummary, error) {
	return r.channels, r.errs["GetShiftChannelSummary"]
}
func (r *fakeRepo) GetShiftItemSummary(_ context.Context, _ string) ([]ItemSummary, error) {
	return r.items, r.errs["GetShiftItemSummary"]
}

func serviceHarness(t *testing.T) (*Service, *fakeRepo, *sqlTestState) {
	t.Helper()
	state := &sqlTestState{}
	repo := newFakeRepo()
	return NewService(newTestDB(t, state), repo), repo, state
}
func shift(id, status string) *Shift {
	return &Shift{ID: id, BranchID: "branch-1", ManagerID: "manager-1", Status: status}
}

func TestOpenShiftPersistsOpenShift(t *testing.T) {
	s, r, tx := serviceHarness(t)
	got, err := s.OpenShift(context.Background(), OpenShiftRequest{BranchID: "branch-1", ManagerID: "manager-1", OpeningFloat: 25})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Status != "OPEN" || got.BranchID != "branch-1" || got.ManagerID != "manager-1" || got.OpeningFloat != 25 || got.OpenedAt.IsZero() {
		t.Fatalf("unexpected shift: %#v", got)
	}
	if r.createdShift != got || tx.begins != 1 || tx.commits != 1 {
		t.Fatalf("not persisted/committed: repo=%#v tx=%#v", r.createdShift, tx)
	}
}
func TestOpenShiftPropagatesTransactionAndRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		beginErr, repoErr, commitErr error
	}{
		{"begin error", errors.New("begin"), nil, nil}, {"create error", nil, errors.New("create"), nil}, {"commit error", nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			r.errs["CreateShift"] = tc.repoErr
			got, err := s.OpenShift(context.Background(), OpenShiftRequest{})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
			if tx.begins != 1 || tc.commitErr != nil && tx.commits != 1 || (tc.repoErr != nil || tc.beginErr != nil) && tx.commits != 0 {
				t.Fatalf("transaction state: %#v", tx)
			}
		})
	}
}
func TestCloseShiftStateAndRepositoryFailures(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		missing      bool
		repoErr      error
		wantErr      error
		wantStatus   string
	}{
		{"missing", "", true, nil, ErrShiftNotFound, ""}, {"already closed", "CLOSED", false, nil, nil, ""}, {"open", "OPEN", false, nil, nil, "CLOSED"}, {"review", "REVIEW", false, nil, nil, "CLOSED"}, {"lookup error", "", false, errors.New("db"), nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			r.errs["GetShiftByID"] = tc.repoErr
			if !tc.missing && tc.repoErr == nil {
				r.shifts["s"] = shift("s", tc.status)
			}
			got, err := s.CloseShift(context.Background(), CloseShiftRequest{ShiftID: "s"})
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want %v", err, tc.wantErr)
			}
			if tc.name == "already closed" && (err == nil || err.Error() != "shift is already closed") {
				t.Fatalf("err=%v", err)
			}
			if tc.repoErr != nil && !errors.Is(err, tc.repoErr) {
				t.Fatalf("err=%v want lookup error", err)
			}
			if tc.wantStatus != "" && (err != nil || got.Status != tc.wantStatus || got.ClosedAt == nil || r.updatedShiftStatus != "CLOSED" || tx.commits != 1) {
				t.Fatalf("close result=%#v err=%v repo=%#v tx=%#v", got, err, r, tx)
			}
			if tc.wantStatus == "" && tx.begins != 0 {
				t.Fatalf("unexpected transaction for rejected state: %#v", tx)
			}
		})
	}
}
func TestCloseShiftPropagatesUpdateAndTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		beginErr, updateErr, commitErr error
	}{
		{"begin", errors.New("begin"), nil, nil}, {"update", nil, errors.New("update"), nil}, {"commit", nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			r.shifts["s"] = shift("s", "OPEN")
			r.errs["UpdateShiftStatus"] = tc.updateErr
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			got, err := s.CloseShift(context.Background(), CloseShiftRequest{ShiftID: "s"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}
func TestRecordSaleSnapshotsPricesAndCalculatesTotals(t *testing.T) {
	s, r, tx := serviceHarness(t)
	r.shifts["shift"] = shift("shift", "OPEN")
	r.prices["tea"] = 3.25
	r.prices["cake"] = 4
	got, items, err := s.RecordSale(context.Background(), RecordSaleRequest{ShiftID: "shift", Channel: "DELIVERY", Items: []SaleItemRequest{{MenuItemID: "tea", Quantity: 2}, {MenuItemID: "cake", Quantity: 1.5}}}, "cashier")
	if err != nil {
		t.Fatal(err)
	}
	if got.GrossAmount != 12.5 || got.NetAmount != 12.5 || got.DiscountAmount != 0 || got.BranchID != "branch-1" || got.Channel != "DELIVERY" || got.CreatedBy != "cashier" {
		t.Fatalf("unexpected sale: %#v", got)
	}
	if len(items) != 2 || items[0].UnitPrice != 3.25 || items[0].TotalPrice != 6.5 || items[1].UnitPrice != 4 || items[1].TotalPrice != 6 || items[0].SaleID != got.ID || r.createdSale != got || len(r.createdItems) != 2 || tx.commits != 1 {
		t.Fatalf("unexpected item/persistence state: items=%#v repo=%#v tx=%#v", items, r, tx)
	}
}
func TestRecordSaleRejectsMissingOrClosedShiftAndMapsMissingMenuItem(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		missing      bool
		priceErr     error
		want         error
	}{
		{"missing shift", "", true, nil, ErrShiftNotFound}, {"closed shift", "CLOSED", false, nil, ErrShiftNotOpen}, {"menu price error", "OPEN", false, errors.New("menu unavailable"), ErrMenuItemNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			if !tc.missing {
				r.shifts["s"] = shift("s", tc.status)
			}
			r.priceErr["m"] = tc.priceErr
			_, _, err := s.RecordSale(context.Background(), RecordSaleRequest{ShiftID: "s", Items: []SaleItemRequest{{MenuItemID: "m", Quantity: 1}}}, "actor")
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
			if tx.begins != 0 && tc.name != "menu price error" {
				t.Fatalf("transaction unexpectedly began: %#v", tx)
			}
			if r.createdSale != nil {
				t.Fatal("sale persisted after rejection")
			}
		})
	}
}
func TestRecordSalePropagatesRepositoryAndTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                                  string
		beginErr, saleErr, itemErr, commitErr error
	}{
		{"begin", errors.New("begin"), nil, nil, nil}, {"create sale", nil, errors.New("sale"), nil, nil}, {"create item", nil, nil, errors.New("item"), nil}, {"commit", nil, nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			r.shifts["s"] = shift("s", "OPEN")
			r.prices["m"] = 2
			r.errs["CreateSale"] = tc.saleErr
			r.errs["CreateSaleItem"] = tc.itemErr
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			got, _, err := s.RecordSale(context.Background(), RecordSaleRequest{ShiftID: "s", Items: []SaleItemRequest{{MenuItemID: "m", Quantity: 1}}}, "actor")
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}
func TestCreateAndCancelDiscount(t *testing.T) {
	s, r, tx := serviceHarness(t)
	r.shifts["s"] = shift("s", "OPEN")
	created, err := s.CreateDiscount(context.Background(), CreateDiscountRequest{ShiftID: "s", Amount: 5, Reason: "service recovery"}, "manager")
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Status != "ACTIVE" || created.Amount != 5 || created.CreatedBy != "manager" || r.createdDiscount != created || tx.commits != 1 {
		t.Fatalf("unexpected discount: %#v repo=%#v tx=%#v", created, r, tx)
	}
	r.discounts[created.ID] = created
	canceled, err := s.CancelDiscount(context.Background(), created.ID, CancelDiscountRequest{Reason: "entered incorrectly"}, "manager-2")
	if err != nil {
		t.Fatal(err)
	}
	if canceled.Status != "CANCELED" || canceled.CanceledReason == nil || *canceled.CanceledReason != "entered incorrectly" || r.canceledDiscountID != created.ID || r.canceledBy != "manager-2" || tx.commits != 2 {
		t.Fatalf("unexpected cancellation: %#v repo=%#v tx=%#v", canceled, r, tx)
	}
}
func TestDiscountMissingClosedCanceledAndRepositoryErrors(t *testing.T) {
	t.Run("create missing shift", func(t *testing.T) {
		s, _, _ := serviceHarness(t)
		_, err := s.CreateDiscount(context.Background(), CreateDiscountRequest{ShiftID: "missing"}, "a")
		if !errors.Is(err, ErrShiftNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("create on closed shift", func(t *testing.T) {
		s, r, _ := serviceHarness(t)
		r.shifts["s"] = shift("s", "CLOSED")
		_, err := s.CreateDiscount(context.Background(), CreateDiscountRequest{ShiftID: "s"}, "a")
		if !errors.Is(err, ErrShiftNotOpen) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("cancel missing", func(t *testing.T) {
		s, _, _ := serviceHarness(t)
		_, err := s.CancelDiscount(context.Background(), "missing", CancelDiscountRequest{}, "a")
		if !errors.Is(err, ErrDiscountNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("cancel already canceled", func(t *testing.T) {
		s, r, _ := serviceHarness(t)
		r.discounts["d"] = &Discount{ID: "d", Status: "CANCELED"}
		_, err := s.CancelDiscount(context.Background(), "d", CancelDiscountRequest{}, "a")
		if err == nil || err.Error() != "discount is already canceled" {
			t.Fatalf("err=%v", err)
		}
	})
	for _, tc := range []struct {
		name                   string
		create, cancel, lookup error
	}{
		{"create repository", errors.New("create"), nil, nil}, {"cancel repository", nil, errors.New("cancel"), nil}, {"lookup discount", nil, nil, errors.New("lookup")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, _ := serviceHarness(t)
			r.shifts["s"] = shift("s", "OPEN")
			r.discounts["d"] = &Discount{ID: "d", Status: "ACTIVE"}
			r.errs["CreateDiscount"] = tc.create
			r.errs["CancelDiscount"] = tc.cancel
			r.errs["GetDiscountByID"] = tc.lookup
			var err error
			if tc.name == "create repository" {
				_, err = s.CreateDiscount(context.Background(), CreateDiscountRequest{ShiftID: "s", Amount: 1}, "a")
			} else {
				_, err = s.CancelDiscount(context.Background(), "d", CancelDiscountRequest{Reason: "r"}, "a")
			}
			if err == nil {
				t.Fatal("expected repository error")
			}
		})
	}
}
func TestShiftSummaryAggregatesChannelsDiscountsAndItems(t *testing.T) {
	s, r, _ := serviceHarness(t)
	r.shifts["s"] = shift("s", "CLOSED")
	r.channels = []ChannelSummary{{Channel: "RESTAURANT", TotalSales: 80, OrderCount: 3}, {Channel: "DELIVERY", TotalSales: 20, OrderCount: 2}}
	r.items = []ItemSummary{{MenuItemID: "m", ItemName: "Soup", TotalQuantity: 4, TotalSales: 40}}
	r.gross = 110
	r.discountsTotal = 10
	got, err := s.GetShiftSummary(context.Background(), "s")
	if err != nil {
		t.Fatal(err)
	}
	if got.ShiftID != "s" || got.Status != "CLOSED" || got.GrossSales != 110 || got.TotalDiscounts != 10 || got.NetSales != 100 || got.TotalOrders != 5 || len(got.Channels) != 2 || len(got.Items) != 1 || got.Items[0].TotalQuantity != 4 {
		t.Fatalf("unexpected summary: %#v", got)
	}
}
func TestShiftSummaryEmptyShiftAndRepositoryErrors(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		s, r, _ := serviceHarness(t)
		r.shifts["s"] = shift("s", "OPEN")
		got, err := s.GetShiftSummary(context.Background(), "s")
		if err != nil {
			t.Fatal(err)
		}
		if got.NetSales != 0 || got.TotalOrders != 0 || len(got.Channels) != 0 || len(got.Items) != 0 {
			t.Fatalf("unexpected empty summary: %#v", got)
		}
	})
	t.Run("missing", func(t *testing.T) {
		s, _, _ := serviceHarness(t)
		_, err := s.GetShiftSummary(context.Background(), "missing")
		if !errors.Is(err, ErrShiftNotFound) {
			t.Fatalf("err=%v", err)
		}
	})
	for _, method := range []string{"GetShiftByID", "GetShiftChannelSummary", "GetShiftItemSummary", "GetShiftSalesTotal"} {
		t.Run(method, func(t *testing.T) {
			s, r, _ := serviceHarness(t)
			r.shifts["s"] = shift("s", "OPEN")
			r.errs[method] = errors.New("repository")
			_, err := s.GetShiftSummary(context.Background(), "s")
			if err == nil {
				t.Fatal("expected repository error")
			}
		})
	}
}

var _ driver.Connector = sqlTestConnector{}
var _ driver.ConnBeginTx = sqlTestConn{}
var _ = time.UTC

func TestDiscountOperationsPropagateTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                string
		beginErr, commitErr error
		cancel              bool
	}{
		{"create begin", errors.New("begin"), nil, false},
		{"create commit", nil, errors.New("commit"), false},
		{"cancel begin", errors.New("begin"), nil, true},
		{"cancel commit", nil, errors.New("commit"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := serviceHarness(t)
			tx.beginErr, tx.commitErr = tc.beginErr, tc.commitErr
			r.shifts["s"] = shift("s", "OPEN")
			r.discounts["d"] = &Discount{ID: "d", ShiftID: "s", Status: "ACTIVE"}
			var err error
			if tc.cancel {
				_, err = s.CancelDiscount(context.Background(), "d", CancelDiscountRequest{Reason: "cancel"}, "actor")
			} else {
				_, err = s.CreateDiscount(context.Background(), CreateDiscountRequest{ShiftID: "s", Amount: 1, Reason: "reason"}, "actor")
			}
			if err == nil {
				t.Fatal("expected transaction error")
			}
			if tc.commitErr != nil && tx.commits != 1 {
				t.Fatalf("commit attempts = %d, want 1", tx.commits)
			}
			if tc.beginErr != nil && tx.commits != 0 {
				t.Fatalf("committed despite begin failure: %#v", tx)
			}
		})
	}
}
func TestRecordSalePropagatesShiftLookupError(t *testing.T) {
	s, r, tx := serviceHarness(t)
	r.errs["GetShiftByID"] = errors.New("shift lookup")
	_, _, err := s.RecordSale(context.Background(), RecordSaleRequest{ShiftID: "s"}, "actor")
	if err == nil || err.Error() != "shift lookup" {
		t.Fatalf("error = %v, want shift lookup error", err)
	}
	if tx.begins != 0 {
		t.Fatalf("transaction began before shift lookup succeeded: %#v", tx)
	}
}
