package overtime

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

type overtimeQASQLState struct {
	query string
	args  []driver.NamedValue
	cols  []string
	rows  [][]driver.Value
	err   error
}

type overtimeQAConnector struct{ state *overtimeQASQLState }

func (c overtimeQAConnector) Connect(context.Context) (driver.Conn, error) {
	return overtimeQAConn{state: c.state}, nil
}
func (overtimeQAConnector) Driver() driver.Driver { return overtimeQADriver{} }

type overtimeQADriver struct{}

func (overtimeQADriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("overtime QA driver requires Connector")
}

type overtimeQAConn struct{ state *overtimeQASQLState }

func (overtimeQAConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare not supported")
}
func (overtimeQAConn) Close() error { return nil }
func (overtimeQAConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported by QA driver")
}
func (c overtimeQAConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.query = query
	c.state.args = append([]driver.NamedValue(nil), args...)
	if c.state.err != nil {
		return nil, c.state.err
	}
	return &overtimeQARows{cols: c.state.cols, rows: c.state.rows}, nil
}

type overtimeQARows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *overtimeQARows) Columns() []string { return r.cols }
func (*overtimeQARows) Close() error        { return nil }
func (r *overtimeQARows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func newOvertimeQADB(t *testing.T, state *overtimeQASQLState) *sqlx.DB {
	t.Helper()
	db := sql.OpenDB(overtimeQAConnector{state: state})
	t.Cleanup(func() { _ = db.Close() })
	return sqlx.NewDb(db, "overtime-qa")
}

func TestQAOvertimeRepositoryNoRowsAndQueryError(t *testing.T) {
	t.Run("missing row maps to nil", func(t *testing.T) {
		state := &overtimeQASQLState{cols: []string{"id"}}
		record, err := NewRepository(newOvertimeQADB(t, state)).GetByID(context.Background(), "ot-id")
		if err != nil || record != nil {
			t.Fatalf("GetByID() = %v, %v; want nil, nil", record, err)
		}
		if !strings.Contains(state.query, "WHERE id = $1") || len(state.args) != 1 || state.args[0].Value != "ot-id" {
			t.Fatalf("query=%q args=%+v; want lookup by overtime ID", state.query, state.args)
		}
	})

	t.Run("query error is wrapped", func(t *testing.T) {
		want := errors.New("database unavailable")
		state := &overtimeQASQLState{err: want}
		_, err := NewRepository(newOvertimeQADB(t, state)).GetByID(context.Background(), "ot-id")
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "get overtime by id") {
			t.Fatalf("GetByID() error = %v, want wrapped database error", err)
		}
	})
}

func TestQAOvertimeRepositoryListBuildsParameterizedFilters(t *testing.T) {
	state := &overtimeQASQLState{cols: []string{"id"}}
	_, err := NewRepository(newOvertimeQADB(t, state)).List(context.Background(), ListFilter{
		BranchID: "branch-a", EmployeeID: "employee-a", Status: "pending", FromDate: "2026-10-01", ToDate: "2026-10-31",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, clause := range []string{"branch_id = $1", "employee_id = $2", "status = $3", "work_date >= $4", "work_date <= $5", "ORDER BY created_at DESC"} {
		if !strings.Contains(state.query, clause) {
			t.Errorf("List() query %q missing %q", state.query, clause)
		}
	}
	want := []any{"branch-a", "employee-a", "pending", "2026-10-01", "2026-10-31"}
	if len(state.args) != len(want) {
		t.Fatalf("query args = %+v, want %d values", state.args, len(want))
	}
	for i, value := range want {
		if state.args[i].Value != value {
			t.Errorf("arg %d = %v, want %v", i+1, state.args[i].Value, value)
		}
	}
}
