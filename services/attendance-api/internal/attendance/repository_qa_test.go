package attendance

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type attendanceQASQLState struct {
	query string
	args  []driver.NamedValue
	cols  []string
	rows  [][]driver.Value
	err   error
}

type attendanceQAConnector struct{ state *attendanceQASQLState }

func (c attendanceQAConnector) Connect(context.Context) (driver.Conn, error) {
	return attendanceQAConn{state: c.state}, nil
}
func (attendanceQAConnector) Driver() driver.Driver { return attendanceQADriver{} }

type attendanceQADriver struct{}

func (attendanceQADriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("attendance QA driver requires Connector")
}

type attendanceQAConn struct{ state *attendanceQASQLState }

func (attendanceQAConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare not supported")
}
func (attendanceQAConn) Close() error { return nil }
func (attendanceQAConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported by QA driver")
}
func (c attendanceQAConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.query = query
	c.state.args = append([]driver.NamedValue(nil), args...)
	if c.state.err != nil {
		return nil, c.state.err
	}
	return &attendanceQARows{cols: c.state.cols, rows: c.state.rows}, nil
}

type attendanceQARows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *attendanceQARows) Columns() []string { return r.cols }
func (*attendanceQARows) Close() error        { return nil }
func (r *attendanceQARows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func newAttendanceQADB(t *testing.T, state *attendanceQASQLState) *sqlx.DB {
	t.Helper()
	db := sql.OpenDB(attendanceQAConnector{state: state})
	t.Cleanup(func() { _ = db.Close() })
	return sqlx.NewDb(db, "attendance-qa")
}

func TestQAAttendanceRepositoryNoRowsAndDatabaseError(t *testing.T) {
	t.Run("no open record returns nil", func(t *testing.T) {
		state := &attendanceQASQLState{cols: []string{"id"}}
		record, err := NewRepository(newAttendanceQADB(t, state)).GetOpenRecord(context.Background(), "employee-1")
		if err != nil || record != nil {
			t.Fatalf("GetOpenRecord() = %v, %v; want nil, nil", record, err)
		}
		if !strings.Contains(state.query, "status = 'open'") || len(state.args) != 1 || state.args[0].Value != "employee-1" {
			t.Fatalf("query=%q args=%+v; want open lookup by employee", state.query, state.args)
		}
	})

	t.Run("query failure remains discoverable", func(t *testing.T) {
		want := errors.New("database unavailable")
		state := &attendanceQASQLState{err: want}
		_, err := NewRepository(newAttendanceQADB(t, state)).GetRecordByDate(context.Background(), "employee-1", time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "get record by date") {
			t.Fatalf("GetRecordByDate() error = %v, want wrapped query error", err)
		}
		if !strings.Contains(state.query, "work_date = $2::date") || len(state.args) != 2 {
			t.Fatalf("query=%q args=%+v; want date lookup arguments", state.query, state.args)
		}
	})
}
