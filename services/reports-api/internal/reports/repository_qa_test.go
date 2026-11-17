package reports

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type reportsSQLCall struct {
	query string
	args  []driver.NamedValue
}
type reportsScript struct {
	calls []reportsSQLCall
	query func(string, []driver.NamedValue) (driver.Rows, error)
}
type reportsConnector struct{ script *reportsScript }

func (c reportsConnector) Connect(context.Context) (driver.Conn, error) {
	return &reportsConn{script: c.script}, nil
}
func (reportsConnector) Driver() driver.Driver { return reportsDriver{} }

type reportsDriver struct{}

func (reportsDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not used") }

type reportsConn struct{ script *reportsScript }

func (*reportsConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (*reportsConn) Close() error                        { return nil }
func (*reportsConn) Begin() (driver.Tx, error)           { return nil, errors.New("transactions not expected") }
func (c *reportsConn) QueryContext(_ context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.script.calls = append(c.script.calls, reportsSQLCall{q, append([]driver.NamedValue(nil), args...)})
	if c.script.query == nil {
		return nil, fmt.Errorf("unexpected query: %s", q)
	}
	return c.script.query(q, args)
}

type reportsRows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func (r *reportsRows) Columns() []string { return r.cols }
func (*reportsRows) Close() error        { return nil }
func (r *reportsRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
func newReportsRepo(t *testing.T, s *reportsScript) *Repository {
	t.Helper()
	db := sqlx.NewDb(sql.OpenDB(reportsConnector{script: s}), "postgres")
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return NewRepository(db)
}
func assertReportArgs(t *testing.T, got []driver.NamedValue, want ...any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args=%v, want %v", got, want)
	}
	for i := range want {
		if fmt.Sprint(got[i].Value) != fmt.Sprint(want[i]) {
			t.Fatalf("arg %d=%v, want %v", i, got[i].Value, want[i])
		}
	}
}

func TestListAuditLogsFiltersPaginationAndMapping(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	actor := "22222222-2222-4222-8222-222222222222"
	entity := "33333333-3333-4333-8333-333333333333"
	s := &reportsScript{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"id", "actor_id", "actor_role", "action", "entity_type", "entity_id", "reason", "created_at"}, rows: [][]driver.Value{{id, actor, "CEO", "APPROVE", "SHIFT", entity, "reviewed", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}}}, nil
	}}
	got, err := newReportsRepo(t, s).ListAuditLogs(context.Background(), AuditFilter{Action: "APPROVE", EntityType: "SHIFT", Limit: 7, Offset: 14})
	if err != nil || len(got) != 1 {
		t.Fatalf("logs=%+v err=%v", got, err)
	}
	resp := got[0].ToResponse()
	if resp.ActorID != actor || resp.EntityID != entity || resp.CreatedAt != "2026-01-02T03:04:05Z" || resp.Reason == nil || *resp.Reason != "reviewed" {
		t.Fatalf("response=%+v", resp)
	}
	q := s.calls[0].query
	if !strings.Contains(q, "action = $1") || !strings.Contains(q, "entity_type = $2") || !strings.Contains(q, "ORDER BY created_at DESC") || !strings.Contains(q, "LIMIT $3 OFFSET $4") {
		t.Fatalf("audit query=%s", q)
	}
	assertReportArgs(t, s.calls[0].args, "APPROVE", "SHIFT", 7, 14)
}

func TestListAuditLogsDefaultLimitAndError(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"id", "actor_id", "actor_role", "action", "entity_type", "entity_id", "reason", "created_at"}}, nil
	}}
	logs, err := newReportsRepo(t, s).ListAuditLogs(context.Background(), AuditFilter{Offset: 9})
	if err != nil || len(logs) != 0 {
		t.Fatalf("logs=%v err=%v", logs, err)
	}
	assertReportArgs(t, s.calls[0].args, 50, 9)
	s2 := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) { return nil, errors.New("audit unavailable") }}
	if _, err = newReportsRepo(t, s2).ListAuditLogs(context.Background(), AuditFilter{}); err == nil || !strings.Contains(err.Error(), "audit unavailable") {
		t.Fatalf("error=%v", err)
	}
}

func TestGetSalesReportCalculatesAverageAndAppliesFilters(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"total_revenue", "total_orders", "restaurant_sales", "delivery_sales"}, rows: [][]driver.Value{{float64(120), int64(4), float64(90), float64(30)}}}, nil
	}}
	got, err := newReportsRepo(t, s).GetSalesReport(context.Background(), ReportFilter{BranchID: "branch-1", From: "2026-01-01", To: "2026-01-31"})
	if err != nil || got.TotalRevenue != 120 || got.TotalOrders != 4 || got.AverageOrder != 30 || got.RestaurantSales != 90 || got.DeliverySales != 30 {
		t.Fatalf("sales=(%+v,%v)", got, err)
	}
	if !strings.Contains(s.calls[0].query, "branch_id = $1") || !strings.Contains(s.calls[0].query, "created_at >= $2") || !strings.Contains(s.calls[0].query, "created_at <= $3") {
		t.Fatalf("sales query=%s", s.calls[0].query)
	}
	assertReportArgs(t, s.calls[0].args, "branch-1", "2026-01-01", "2026-01-31")
}

func TestGetSalesReportHandlesZeroOrdersAndQueryError(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"total_revenue", "total_orders", "restaurant_sales", "delivery_sales"}, rows: [][]driver.Value{{float64(0), int64(0), float64(0), float64(0)}}}, nil
	}}
	got, err := newReportsRepo(t, s).GetSalesReport(context.Background(), ReportFilter{})
	if err != nil || got.AverageOrder != 0 {
		t.Fatalf("sales=(%+v,%v)", got, err)
	}
	s2 := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) { return nil, errors.New("sales unavailable") }}
	if _, err = newReportsRepo(t, s2).GetSalesReport(context.Background(), ReportFilter{}); err == nil {
		t.Fatal("expected query error")
	}
}

func TestSalesReportToDateIncludesTheWholeDay(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"total_revenue", "total_orders", "restaurant_sales", "delivery_sales"}, rows: [][]driver.Value{{float64(0), int64(0), float64(0), float64(0)}}}, nil
	}}
	_, err := newReportsRepo(t, s).GetSalesReport(context.Background(), ReportFilter{To: "2026-01-31"})
	if err != nil {
		t.Fatal(err)
	}
	query := s.calls[0].query
	if !strings.Contains(query, "created_at <") || !strings.Contains(query, "INTERVAL '1 day'") {
		t.Fatalf("sales end-date predicate may exclude records after midnight on the final day: %s", query)
	}
}

func TestGetInventoryReportMapsAggregateAndBranchFilter(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return &reportsRows{cols: []string{"total_items", "total_value", "low_stock_items"}, rows: [][]driver.Value{{int64(5), float64(99.5), int64(2)}}}, nil
	}}
	got, err := newReportsRepo(t, s).GetInventoryReport(context.Background(), ReportFilter{BranchID: "branch-2"})
	if err != nil || got.TotalItems != 5 || got.TotalValue != 99.5 || got.LowStockItems != 2 {
		t.Fatalf("inventory=(%+v,%v)", got, err)
	}
	if !strings.Contains(s.calls[0].query, "i.branch_id = $1") {
		t.Fatalf("inventory query=%s", s.calls[0].query)
	}
	assertReportArgs(t, s.calls[0].args, "branch-2")
}

func TestGetInventoryReportPropagatesQueryError(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		return nil, errors.New("inventory query failed")
	}}
	if _, err := newReportsRepo(t, s).GetInventoryReport(context.Background(), ReportFilter{}); err == nil || !strings.Contains(err.Error(), "inventory query failed") {
		t.Fatalf("inventory query error=%v", err)
	}
}

func TestGetAttendanceReportCalculatesSummaryAndFiltersQueries(t *testing.T) {
	s := &reportsScript{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		switch {
		case strings.Contains(q, "FROM employees"):
			return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(3)}}}, nil
		case strings.Contains(q, "FROM attendance_records"):
			return &reportsRows{cols: []string{"present"}, rows: [][]driver.Value{{int64(2)}}}, nil
		case strings.Contains(q, "FROM overtime"):
			return &reportsRows{cols: []string{"coalesce"}, rows: [][]driver.Value{{float64(17.5)}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", q)
		}
	}}
	got, err := newReportsRepo(t, s).GetAttendanceReport(context.Background(), ReportFilter{BranchID: "branch-3", From: "2026-02-01", To: "2026-02-02"})
	if err != nil || got.TotalEmployees != 3 || got.Present != 2 || got.Absent != 1 || got.TotalOvertime != 17.5 {
		t.Fatalf("attendance=(%+v,%v)", got, err)
	}
	if len(s.calls) != 3 {
		t.Fatalf("query count=%d", len(s.calls))
	}
	assertReportArgs(t, s.calls[0].args, "branch-3")
	assertReportArgs(t, s.calls[1].args, "branch-3", "2026-02-01", "2026-02-02")
	assertReportArgs(t, s.calls[2].args, "branch-3", "2026-02-01", "2026-02-02")
}

func TestGetAttendanceReportPropagatesEachQueryError(t *testing.T) {
	for _, failAt := range []string{"FROM employees", "FROM attendance_records", "FROM overtime"} {
		t.Run(failAt, func(t *testing.T) {
			s := &reportsScript{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
				if strings.Contains(q, failAt) {
					return nil, errors.New("db down")
				}
				if strings.Contains(q, "FROM employees") {
					return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(1)}}}, nil
				}
				if strings.Contains(q, "FROM attendance_records") {
					return &reportsRows{cols: []string{"present"}, rows: [][]driver.Value{{int64(1)}}}, nil
				}
				return &reportsRows{cols: []string{"coalesce"}, rows: [][]driver.Value{{float64(0)}}}, nil
			}}
			if _, err := newReportsRepo(t, s).GetAttendanceReport(context.Background(), ReportFilter{}); err == nil {
				t.Fatal("expected database error")
			}
		})
	}
}

func TestGetApprovalQueueCountsBothQueuesAndFiltersBranch(t *testing.T) {
	i := 0
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		i++
		n := int64(4)
		if i == 2 {
			n = 7
		}
		return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{n}}}, nil
	}}
	got, err := newReportsRepo(t, s).GetApprovalQueue(context.Background(), ReportFilter{BranchID: "branch-4"})
	if err != nil || got.PendingMenuRequests != 4 || got.PendingSupplyRequests != 7 {
		t.Fatalf("queue=(%+v,%v)", got, err)
	}
	if len(s.calls) != 2 || !strings.Contains(s.calls[0].query, "menu_item_requests") || !strings.Contains(s.calls[1].query, "supply_requests") {
		t.Fatalf("calls=%+v", s.calls)
	}
	assertReportArgs(t, s.calls[0].args, "branch-4")
	assertReportArgs(t, s.calls[1].args, "branch-4")
}

func TestGetApprovalQueuePropagatesSecondQueryError(t *testing.T) {
	i := 0
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) {
		i++
		if i == 2 {
			return nil, errors.New("supply query failed")
		}
		return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(1)}}}, nil
	}}
	if _, err := newReportsRepo(t, s).GetApprovalQueue(context.Background(), ReportFilter{}); err == nil || !strings.Contains(err.Error(), "supply query failed") {
		t.Fatalf("error=%v", err)
	}
}

func TestGetApprovalQueuePropagatesFirstQueryError(t *testing.T) {
	s := &reportsScript{query: func(string, []driver.NamedValue) (driver.Rows, error) { return nil, errors.New("menu query failed") }}
	if _, err := newReportsRepo(t, s).GetApprovalQueue(context.Background(), ReportFilter{}); err == nil || !strings.Contains(err.Error(), "menu query failed") {
		t.Fatalf("menu query error=%v", err)
	}
}

func TestReportServiceDelegatesToRepository(t *testing.T) {
	s := &reportsScript{query: func(q string, _ []driver.NamedValue) (driver.Rows, error) {
		switch {
		case strings.Contains(q, "FROM audit_logs"):
			return &reportsRows{cols: []string{"id", "actor_id", "actor_role", "action", "entity_type", "entity_id", "reason", "created_at"}}, nil
		case strings.Contains(q, "FROM sales"):
			return &reportsRows{cols: []string{"total_revenue", "total_orders", "restaurant_sales", "delivery_sales"}, rows: [][]driver.Value{{float64(0), int64(0), float64(0), float64(0)}}}, nil
		case strings.Contains(q, "FROM inventories"):
			return &reportsRows{cols: []string{"total_items", "total_value", "low_stock_items"}, rows: [][]driver.Value{{int64(0), float64(0), int64(0)}}}, nil
		case strings.Contains(q, "FROM employees"):
			return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(0)}}}, nil
		case strings.Contains(q, "FROM attendance_records"):
			return &reportsRows{cols: []string{"present"}, rows: [][]driver.Value{{int64(0)}}}, nil
		case strings.Contains(q, "FROM overtime"):
			return &reportsRows{cols: []string{"total"}, rows: [][]driver.Value{{float64(0)}}}, nil
		case strings.Contains(q, "FROM menu_item_requests"):
			return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(0)}}}, nil
		case strings.Contains(q, "FROM supply_requests"):
			return &reportsRows{cols: []string{"count"}, rows: [][]driver.Value{{int64(0)}}}, nil
		default:
			return nil, fmt.Errorf("unexpected query: %s", q)
		}
	}}
	svc := NewService(newReportsRepo(t, s))
	ctx := context.Background()
	if _, err := svc.GetAuditLogs(ctx, AuditFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetSalesReport(ctx, ReportFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetInventoryReport(ctx, ReportFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetAttendanceReport(ctx, ReportFilter{}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetApprovalQueue(ctx, ReportFilter{}); err != nil {
		t.Fatal(err)
	}
}
