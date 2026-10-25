package sales

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/jmoiron/sqlx"
)

const (
	rass93ShiftID    = "11111111-1111-4111-8111-111111111111"
	rass93MenuItemID = "22222222-2222-4222-8222-222222222222"
	rass93DiscountID = "33333333-3333-4333-8333-333333333333"
)

func TestQARASS93RecordSaleCalculatesLineTotalAndCapturesHistoricalPrice(t *testing.T) {
	db, stats := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		shift:  &Shift{ID: rass93ShiftID, BranchID: "branch-1", Status: "OPEN"},
		prices: map[string]float64{rass93MenuItemID: 12.5},
	}
	svc := NewService(db, repo)

	sale, items, err := svc.RecordSale(context.Background(), RecordSaleRequest{
		ShiftID: rass93ShiftID,
		Channel: "RESTAURANT",
		Items:   []SaleItemRequest{{MenuItemID: rass93MenuItemID, Quantity: 2}},
	}, "manager-1")
	if err != nil {
		t.Fatalf("RecordSale() error = %v", err)
	}
	if sale.GrossAmount != 25 || sale.NetAmount != 25 || sale.Channel != "RESTAURANT" {
		t.Fatalf("sale = %+v, want restaurant sale gross/net 25", sale)
	}
	if len(items) != 1 || items[0].UnitPrice != 12.5 || items[0].TotalPrice != 25 || items[0].Quantity != 2 {
		t.Fatalf("sale items = %+v, want quantity 2 at historical unit price 12.5 and line total 25", items)
	}
	if len(repo.savedSales) != 1 || len(repo.savedItems) != 1 || repo.savedItems[0].UnitPrice != 12.5 {
		t.Fatalf("repository received sales/items = %d/%d, want one sale and one item with price snapshot", len(repo.savedSales), len(repo.savedItems))
	}
	repo.prices[rass93MenuItemID] = 20
	if items[0].UnitPrice != 12.5 || repo.savedItems[0].UnitPrice != 12.5 {
		t.Fatalf("captured sale price changed after menu price changed: returned=%v saved=%v", items[0].UnitPrice, repo.savedItems[0].UnitPrice)
	}
	assertRASS93Commit(t, stats)
}

func TestQARASS93RecordSaleSeparatesRestaurantAndDeliveryChannels(t *testing.T) {
	db, stats := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		shift:  &Shift{ID: rass93ShiftID, BranchID: "branch-1", Status: "OPEN"},
		prices: map[string]float64{rass93MenuItemID: 9},
	}
	svc := NewService(db, repo)

	for _, channel := range []string{"RESTAURANT", "DELIVERY"} {
		_, _, err := svc.RecordSale(context.Background(), RecordSaleRequest{
			ShiftID: rass93ShiftID,
			Channel: channel,
			Items:   []SaleItemRequest{{MenuItemID: rass93MenuItemID, Quantity: 1}},
		}, "manager-1")
		if err != nil {
			t.Fatalf("RecordSale(channel=%s) error = %v", channel, err)
		}
	}
	if len(repo.savedSales) != 2 || repo.savedSales[0].Channel != "RESTAURANT" || repo.savedSales[1].Channel != "DELIVERY" {
		t.Fatalf("saved channels = %+v, want separate RESTAURANT and DELIVERY sales", repo.savedSales)
	}
	if stats.commits != 2 || stats.begins != 2 {
		t.Fatalf("transaction begins/commits = %d/%d, want 2/2", stats.begins, stats.commits)
	}
}

func TestQARASS93CreateDiscountCapturesAmountAndReason(t *testing.T) {
	db, stats := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{shift: &Shift{ID: rass93ShiftID, Status: "OPEN"}}
	svc := NewService(db, repo)

	discount, err := svc.CreateDiscount(context.Background(), CreateDiscountRequest{
		ShiftID: rass93ShiftID, Amount: 7.5, Reason: "service recovery",
	}, "manager-1")
	if err != nil {
		t.Fatalf("CreateDiscount() error = %v", err)
	}
	if discount.Amount != 7.5 || discount.Reason != "service recovery" || discount.Status != "ACTIVE" {
		t.Fatalf("discount = %+v, want amount 7.5, reason and ACTIVE status", discount)
	}
	if repo.createdDiscount == nil || repo.createdDiscount.Amount != 7.5 || repo.createdDiscount.Reason != "service recovery" {
		t.Fatalf("repository received discount = %+v, want requested amount and reason", repo.createdDiscount)
	}
	assertRASS93Commit(t, stats)
}

func TestQARASS93CancelDiscountPreservesOriginalFieldsAndRecordsAuditReason(t *testing.T) {
	db, stats := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		discounts: map[string]*Discount{
			rass93DiscountID: {
				ID: rass93DiscountID, ShiftID: rass93ShiftID, Amount: 7.5, Reason: "service recovery", Status: "ACTIVE",
			},
		},
	}
	svc := NewService(db, repo)

	discount, err := svc.CancelDiscount(context.Background(), rass93DiscountID, CancelDiscountRequest{
		Reason: "entered in error",
	}, "manager-1")
	if err != nil {
		t.Fatalf("CancelDiscount() error = %v", err)
	}
	if discount.Status != "CANCELED" || discount.Amount != 7.5 || discount.Reason != "service recovery" || discount.CanceledReason == nil || *discount.CanceledReason != "entered in error" {
		t.Fatalf("canceled discount = %+v, want original amount/reason retained with cancellation audit reason", discount)
	}
	if repo.cancelReason != "entered in error" || repo.cancelActor != "manager-1" || repo.cancelCalls != 1 {
		t.Fatalf("cancel repository call = reason %q actor %q calls %d, want audit fields recorded once", repo.cancelReason, repo.cancelActor, repo.cancelCalls)
	}
	assertRASS93Commit(t, stats)
}

func TestQARASS93ShiftSummaryReturnsSalesQuantitiesAndNetCalculation(t *testing.T) {
	db, _ := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		shift: &Shift{ID: rass93ShiftID, Status: "REVIEW"},
		channels: []ChannelSummary{
			{Channel: "RESTAURANT", TotalSales: 100, OrderCount: 2},
			{Channel: "DELIVERY", TotalSales: 50, OrderCount: 1},
		},
		items: []ItemSummary{
			{MenuItemID: rass93MenuItemID, ItemName: "Soup", TotalQuantity: 4, TotalSales: 100},
		},
		grossSales: 150, totalDiscounts: 20,
	}
	svc := NewService(db, repo)

	summary, err := svc.GetShiftSummary(context.Background(), rass93ShiftID)
	if err != nil {
		t.Fatalf("GetShiftSummary() error = %v", err)
	}
	if summary.GrossSales != 150 || summary.TotalDiscounts != 20 || summary.NetSales != 130 || summary.TotalOrders != 3 {
		t.Fatalf("summary totals = %+v, want gross 150, discounts 20, net 130, orders 3", summary)
	}
	if len(summary.Items) != 1 || summary.Items[0].TotalQuantity != 4 || len(summary.Channels) != 2 {
		t.Fatalf("summary dimensions = channels %+v items %+v, want both channels and item quantity 4", summary.Channels, summary.Items)
	}
}

func TestQARASS93ClosedShiftRejectsNewSalesAndDiscounts(t *testing.T) {
	db, stats := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		shift:  &Shift{ID: rass93ShiftID, BranchID: "branch-1", Status: "OPEN"},
		prices: map[string]float64{rass93MenuItemID: 9},
	}
	svc := NewService(db, repo)

	closed, err := svc.CloseShift(context.Background(), CloseShiftRequest{ShiftID: rass93ShiftID})
	if err != nil {
		t.Fatalf("CloseShift() error = %v", err)
	}
	if closed.Status != "CLOSED" || closed.ClosedAt == nil || repo.updatedStatus != "CLOSED" {
		t.Fatalf("closed shift = %+v; repository status=%q, want CLOSED with closed_at", closed, repo.updatedStatus)
	}
	if _, _, err := svc.RecordSale(context.Background(), RecordSaleRequest{
		ShiftID: rass93ShiftID, Channel: "RESTAURANT",
		Items: []SaleItemRequest{{MenuItemID: rass93MenuItemID, Quantity: 1}},
	}, "manager-1"); !errors.Is(err, ErrShiftNotOpen) {
		t.Fatalf("RecordSale() on closed shift error = %v, want ErrShiftNotOpen", err)
	}
	if _, err := svc.CreateDiscount(context.Background(), CreateDiscountRequest{
		ShiftID: rass93ShiftID, Amount: 2, Reason: "late discount",
	}, "manager-1"); !errors.Is(err, ErrShiftNotOpen) {
		t.Fatalf("CreateDiscount() on closed shift error = %v, want ErrShiftNotOpen", err)
	}
	if stats.begins != 1 || stats.commits != 1 {
		t.Fatalf("transaction begins/commits = %d/%d, want only close transaction 1/1", stats.begins, stats.commits)
	}
}

func TestQARASS93ClosedShiftRejectsDiscountCancellation(t *testing.T) {
	db, _ := newRASS93NoopDB(t)
	repo := &rass93SalesRepository{
		shift: &Shift{ID: rass93ShiftID, Status: "CLOSED"},
		discounts: map[string]*Discount{
			rass93DiscountID: {
				ID: rass93DiscountID, ShiftID: rass93ShiftID, Amount: 7.5, Reason: "service recovery", Status: "ACTIVE",
			},
		},
	}
	svc := NewService(db, repo)

	if _, err := svc.CancelDiscount(context.Background(), rass93DiscountID, CancelDiscountRequest{
		Reason: "late change after close",
	}, "manager-1"); err == nil {
		t.Fatal("CancelDiscount() succeeded for a discount on a CLOSED shift; closed shift should be read-only")
	}
}

type rass93SalesRepository struct {
	Repository
	shift           *Shift
	prices          map[string]float64
	savedSales      []*Sale
	savedItems      []*SaleItem
	createdDiscount *Discount
	discounts       map[string]*Discount
	cancelReason    string
	cancelActor     string
	cancelCalls     int
	updatedStatus   string
	channels        []ChannelSummary
	items           []ItemSummary
	grossSales      float64
	totalDiscounts  float64
}

func (r *rass93SalesRepository) GetShiftByID(_ context.Context, _ string) (*Shift, error) {
	return r.shift, nil
}

func (r *rass93SalesRepository) GetMenuItemPrice(_ context.Context, id string) (float64, error) {
	price, ok := r.prices[id]
	if !ok {
		return 0, ErrMenuItemNotFound
	}
	return price, nil
}

func (r *rass93SalesRepository) CreateSale(_ context.Context, _ *sqlx.Tx, sale *Sale) error {
	r.savedSales = append(r.savedSales, sale)
	return nil
}

func (r *rass93SalesRepository) CreateSaleItem(_ context.Context, _ *sqlx.Tx, item *SaleItem) error {
	r.savedItems = append(r.savedItems, item)
	return nil
}

func (r *rass93SalesRepository) CreateDiscount(_ context.Context, _ *sqlx.Tx, discount *Discount) error {
	r.createdDiscount = discount
	return nil
}

func (r *rass93SalesRepository) GetDiscountByID(_ context.Context, id string) (*Discount, error) {
	return r.discounts[id], nil
}

func (r *rass93SalesRepository) CancelDiscount(_ context.Context, _ *sqlx.Tx, id, reason, actorID string) error {
	discount := r.discounts[id]
	discount.Status = "CANCELED"
	discount.CanceledReason = &reason
	discount.CanceledBy = &actorID
	r.cancelReason = reason
	r.cancelActor = actorID
	r.cancelCalls++
	return nil
}

func (r *rass93SalesRepository) UpdateShiftStatus(_ context.Context, _ *sqlx.Tx, _ string, status string) error {
	r.updatedStatus = status
	if r.shift != nil {
		r.shift.Status = status
	}
	return nil
}

func (r *rass93SalesRepository) GetShiftChannelSummary(_ context.Context, _ string) ([]ChannelSummary, error) {
	return r.channels, nil
}

func (r *rass93SalesRepository) GetShiftItemSummary(_ context.Context, _ string) ([]ItemSummary, error) {
	return r.items, nil
}

func (r *rass93SalesRepository) GetShiftSalesTotal(_ context.Context, _ string) (float64, float64, error) {
	return r.grossSales, r.totalDiscounts, nil
}

type rass93TxStats struct {
	begins  int
	commits int
}

type rass93Connector struct {
	stats *rass93TxStats
}

func (c rass93Connector) Connect(context.Context) (driver.Conn, error) {
	return &rass93Conn{stats: c.stats}, nil
}

func (c rass93Connector) Driver() driver.Driver {
	return rass93Driver{}
}

type rass93Driver struct{}

func (rass93Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("RASS-93 tests must use their isolated connector")
}

type rass93Conn struct {
	stats *rass93TxStats
}

func (c *rass93Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("RASS-93 fake driver does not execute SQL")
}

func (*rass93Conn) Close() error { return nil }

func (c *rass93Conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *rass93Conn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.stats.begins++
	return &rass93Tx{stats: c.stats}, nil
}

type rass93Tx struct {
	stats *rass93TxStats
}

func (*rass93Tx) Rollback() error { return nil }

func (tx *rass93Tx) Commit() error {
	tx.stats.commits++
	return nil
}

func newRASS93NoopDB(t *testing.T) (*sqlx.DB, *rass93TxStats) {
	t.Helper()
	stats := &rass93TxStats{}
	db := sqlx.NewDb(sql.OpenDB(rass93Connector{stats: stats}), "rass93-noop")
	t.Cleanup(func() { _ = db.Close() })
	return db, stats
}

func assertRASS93Commit(t *testing.T, stats *rass93TxStats) {
	t.Helper()
	if stats.begins != 1 || stats.commits != 1 {
		t.Fatalf("transaction begins/commits = %d/%d, want 1/1", stats.begins, stats.commits)
	}
}
