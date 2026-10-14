package employee

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

type employeeQASQLState struct {
	query string
	args  []driver.NamedValue
	cols  []string
	rows  [][]driver.Value
	err   error
}

type employeeQAConnector struct{ state *employeeQASQLState }

func (c employeeQAConnector) Connect(context.Context) (driver.Conn, error) {
	return employeeQAConn{state: c.state}, nil
}
func (employeeQAConnector) Driver() driver.Driver { return employeeQADriver{} }

type employeeQADriver struct{}

func (employeeQADriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("employee QA driver requires Connector")
}

type employeeQAConn struct{ state *employeeQASQLState }

func (employeeQAConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare not supported")
}
func (employeeQAConn) Close() error { return nil }
func (employeeQAConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions not supported by QA driver")
}
func (c employeeQAConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.query = query
	c.state.args = append([]driver.NamedValue(nil), args...)
	if c.state.err != nil {
		return nil, c.state.err
	}
	return &employeeQARows{cols: c.state.cols, rows: c.state.rows}, nil
}

type employeeQARows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *employeeQARows) Columns() []string { return r.cols }
func (*employeeQARows) Close() error        { return nil }
func (r *employeeQARows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.idx])
	r.idx++
	return nil
}

func newEmployeeQADB(t *testing.T, state *employeeQASQLState) *sqlx.DB {
	t.Helper()
	db := sql.OpenDB(employeeQAConnector{state: state})
	t.Cleanup(func() { _ = db.Close() })
	return sqlx.NewDb(db, "employee-qa")
}

func TestQAEmployeeRepositoryNotFoundAndQueryFailure(t *testing.T) {
	t.Run("no rows maps to nil employee", func(t *testing.T) {
		state := &employeeQASQLState{cols: []string{"id"}}
		employee, err := NewRepository(newEmployeeQADB(t, state)).GetByID(context.Background(), "emp-id")
		if err != nil || employee != nil {
			t.Fatalf("GetByID() = %v, %v; want nil, nil", employee, err)
		}
		if !strings.Contains(state.query, "WHERE id = $1") || len(state.args) != 1 || state.args[0].Value != "emp-id" {
			t.Fatalf("query=%q args=%+v; expected lookup by ID", state.query, state.args)
		}
	})

	t.Run("query error is wrapped and preserved", func(t *testing.T) {
		want := errors.New("database unavailable")
		state := &employeeQASQLState{err: want}
		_, err := NewRepository(newEmployeeQADB(t, state)).GetByID(context.Background(), "emp-id")
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "get employee by id") {
			t.Fatalf("GetByID() error = %v, want wrapped database error", err)
		}
	})
}

func TestQAEmployeeRepositoryNationalIDExistsQueryAndArguments(t *testing.T) {
	state := &employeeQASQLState{cols: []string{"exists"}, rows: [][]driver.Value{{true}}}
	exists, err := NewRepository(newEmployeeQADB(t, state)).ExistsNationalID(context.Background(), "12345678901234", "employee-exclude")
	if err != nil || !exists {
		t.Fatalf("ExistsNationalID() = %v, %v; want true, nil", exists, err)
	}
	if !strings.Contains(state.query, "national_id = $1 AND id != $2") || len(state.args) != 2 || state.args[0].Value != "12345678901234" || state.args[1].Value != "employee-exclude" {
		t.Fatalf("query=%q args=%+v; expected national ID and excluded employee", state.query, state.args)
	}
}
