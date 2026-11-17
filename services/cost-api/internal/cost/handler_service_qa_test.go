package cost

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type costTxState struct {
	beginErr, commitErr              error
	begins, commits, rollbacks       int
	queryErr, execErr                error
	nextConn, txConnID, upsertConnID int
	failComponentInsert              bool
	query                            func(string, []driver.NamedValue) (driver.Rows, error)
	calls                            []costSQLCall
}

type costSQLCall struct {
	query string
	args  []driver.NamedValue
}

type costConnector struct{ state *costTxState }

func (c costConnector) Connect(context.Context) (driver.Conn, error) {
	c.state.nextConn++
	return &costConn{state: c.state, id: c.state.nextConn}, nil
}
func (costConnector) Driver() driver.Driver { return costDriver{} }

type costDriver struct{}

func (costDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not used") }

type costConn struct {
	state *costTxState
	id    int
}

func (c *costConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not used") }
func (*costConn) Close() error                          { return nil }
func (c *costConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *costConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.begins++
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	c.state.txConnID = c.id
	return &costTx{state: c.state}, nil
}
func (c *costConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.calls = append(c.state.calls, costSQLCall{query: query, args: append([]driver.NamedValue(nil), args...)})
	if c.state.queryErr != nil {
		return nil, c.state.queryErr
	}
	if c.state.query != nil {
		return c.state.query(query, args)
	}
	return &costRows{}, nil
}
func (c *costConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.calls = append(c.state.calls, costSQLCall{query: query, args: append([]driver.NamedValue(nil), args...)})
	if strings.Contains(query, "INSERT INTO menu_item_costs") {
		c.state.upsertConnID = c.id
	}
	if c.state.failComponentInsert && strings.Contains(query, "INSERT INTO recipe_components") {
		return nil, errors.New("component insert failed")
	}
	if c.state.execErr != nil {
		return nil, c.state.execErr
	}
	return driver.RowsAffected(1), nil
}

type costTx struct{ state *costTxState }

func (t *costTx) Commit() error   { t.state.commits++; return t.state.commitErr }
func (t *costTx) Rollback() error { t.state.rollbacks++; return nil }

func newCostTxDB(t *testing.T, state *costTxState) *sqlx.DB {
	t.Helper()
	db := sqlx.NewDb(sql.OpenDB(costConnector{state: state}), "postgres")
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newCostSQLDB(t *testing.T, state *costTxState) *sqlx.DB { return newCostTxDB(t, state) }

type costRepoStub struct {
	cost                                                                 *MenuItemCost
	components                                                           []*RecipeComponent
	price                                                                float64
	upsertErr, deleteErr, createErr, getCostErr, priceErr, componentsErr error
	cogs                                                                 *COGSReportResponse
	gross                                                                *GrossProfitReportResponse
	contribution                                                         *EstimatedContributionResponse
	reportErr                                                            error
	upserts, deletes, creates, costReads, priceReads, componentReads     int
	filter                                                               ReportFilter
}

func (r *costRepoStub) UpsertCost(_ context.Context, c *MenuItemCost) error {
	r.upserts++
	r.cost = c
	return r.upsertErr
}
func (r *costRepoStub) GetCostByMenuItemID(context.Context, string) (*MenuItemCost, error) {
	r.costReads++
	return r.cost, r.getCostErr
}
func (r *costRepoStub) GetMenuItemPrice(context.Context, string) (float64, error) {
	r.priceReads++
	return r.price, r.priceErr
}
func (r *costRepoStub) CreateComponent(_ context.Context, _ *sqlx.Tx, c *RecipeComponent) error {
	r.creates++
	r.components = append(r.components, c)
	return r.createErr
}
func (r *costRepoStub) DeleteComponentsByMenuItem(context.Context, *sqlx.Tx, string) error {
	r.deletes++
	return r.deleteErr
}
func (r *costRepoStub) GetComponentsByMenuItem(context.Context, string) ([]*RecipeComponent, error) {
	r.componentReads++
	return r.components, r.componentsErr
}
func (r *costRepoStub) GetCOGSReport(_ context.Context, f ReportFilter) (*COGSReportResponse, error) {
	r.filter = f
	return r.cogs, r.reportErr
}
func (r *costRepoStub) GetGrossProfitReport(_ context.Context, f ReportFilter) (*GrossProfitReportResponse, error) {
	r.filter = f
	return r.gross, r.reportErr
}
func (r *costRepoStub) GetEstimatedContribution(_ context.Context, f ReportFilter) (*EstimatedContributionResponse, error) {
	r.filter = f
	return r.contribution, r.reportErr
}

func costRouter(s *Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(s).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestSetCostHandlerRejectsInvalidPayloads(t *testing.T) {
	validID := "11111111-1111-4111-8111-111111111111"
	validGood := "22222222-2222-4222-8222-222222222222"
	cases := []struct{ name, body string }{
		{"malformed JSON", `{"menu_item_id":`},
		{"missing menu item ID", `{"cost_amount":2}`},
		{"malformed menu item ID", `{"menu_item_id":"bad","cost_amount":2}`},
		{"negative cost", `{"menu_item_id":"` + validID + `","cost_amount":-1}`},
		{"malformed component ID", `{"menu_item_id":"` + validID + `","cost_amount":2,"components":[{"good_id":"bad","quantity":1,"unit":"kg"}]}`},
		{"zero component quantity", `{"menu_item_id":"` + validID + `","cost_amount":2,"components":[{"good_id":"` + validGood + `","quantity":0,"unit":"kg"}]}`},
		{"missing component unit", `{"menu_item_id":"` + validID + `","cost_amount":2,"components":[{"good_id":"` + validGood + `","quantity":1}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			costRouter(nil).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/cost/items", strings.NewReader(tc.body)))
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s; want 400", w.Code, w.Body.String())
			}
		})
	}
}

func TestSetCostHandlerAcceptsZeroCostBoundary(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{}
	r := costRouter(NewService(newCostTxDB(t, state), repo))
	body := `{"menu_item_id":"11111111-1111-4111-8111-111111111111","cost_amount":0,"components":[]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/cost/items", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s; zero is allowed by the persisted cost constraint", w.Code, w.Body.String())
	}
	if repo.upserts != 1 || repo.cost == nil || repo.cost.CostAmount != 0 {
		t.Fatalf("zero cost was not saved: repo=%+v", repo)
	}
}

func TestSetCostHandlerPersistsValidCostAndComponents(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{}
	r := costRouter(NewService(newCostTxDB(t, state), repo))
	body := `{"menu_item_id":"11111111-1111-4111-8111-111111111111","cost_amount":12.5,"components":[{"good_id":"22222222-2222-4222-8222-222222222222","quantity":2,"unit":"kg"}]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/cost/items", strings.NewReader(body)))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if repo.upserts != 1 || repo.deletes != 1 || repo.creates != 1 || len(repo.components) != 1 {
		t.Fatalf("persistence calls missing: %+v", repo)
	}
}

func TestSetCostHandlerMapsPersistenceErrorTo500(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{upsertErr: errors.New("cost database unavailable")}
	r := costRouter(NewService(newCostTxDB(t, state), repo))
	body := `{"menu_item_id":"11111111-1111-4111-8111-111111111111","cost_amount":12.5,"components":[]}`
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/cost/items", strings.NewReader(body)))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestSetCostServiceCommitsCostAndComponents(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{}
	input := SetCostRequest{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 12.5, Components: []RecipeComponentRequest{{GoodID: "22222222-2222-4222-8222-222222222222", Quantity: 2, Unit: "kg"}}}
	err := NewService(newCostTxDB(t, state), repo).SetCost(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if state.begins != 1 || state.commits != 1 || state.rollbacks != 0 {
		t.Fatalf("transaction counts begin/commit/rollback=%d/%d/%d", state.begins, state.commits, state.rollbacks)
	}
	if repo.upserts != 1 || repo.deletes != 1 || repo.creates != 1 || repo.cost.CostAmount != 12.5 {
		t.Fatalf("persistence calls or cost incorrect: %+v", repo)
	}
	if len(repo.components) != 1 || repo.components[0].GoodID != input.Components[0].GoodID || repo.components[0].Quantity != 2 {
		t.Fatalf("component not persisted: %+v", repo.components)
	}
}

func TestSetCostServiceRollsBackAfterComponentFailure(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{createErr: errors.New("component insert failed")}
	err := NewService(newCostTxDB(t, state), repo).SetCost(context.Background(), SetCostRequest{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 2, Components: []RecipeComponentRequest{{GoodID: "22222222-2222-4222-8222-222222222222", Quantity: 1, Unit: "ea"}}})
	if err == nil || !strings.Contains(err.Error(), "component insert failed") {
		t.Fatalf("error=%v, want component failure", err)
	}
	if state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("transaction counts commit/rollback=%d/%d", state.commits, state.rollbacks)
	}
}

func TestSetCostServiceRollsBackAfterCostPersistenceFailure(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{upsertErr: errors.New("cost write failed")}
	err := NewService(newCostTxDB(t, state), repo).SetCost(context.Background(), SetCostRequest{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 2})
	if err == nil || !strings.Contains(err.Error(), "cost write failed") {
		t.Fatalf("error=%v, want cost persistence failure", err)
	}
	if state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("transaction counts commit/rollback=%d/%d", state.commits, state.rollbacks)
	}
}

func TestSetCostServiceRollsBackAfterComponentDeleteFailure(t *testing.T) {
	state := &costTxState{}
	repo := &costRepoStub{deleteErr: errors.New("component delete failed")}
	err := NewService(newCostTxDB(t, state), repo).SetCost(context.Background(), SetCostRequest{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 2})
	if err == nil || !strings.Contains(err.Error(), "component delete failed") {
		t.Fatalf("error=%v, want delete failure", err)
	}
	if state.commits != 0 || state.rollbacks != 1 {
		t.Fatalf("transaction counts commit/rollback=%d/%d", state.commits, state.rollbacks)
	}
}

func TestSetCostServiceReturnsBeginAndCommitErrors(t *testing.T) {
	beginState := &costTxState{beginErr: errors.New("begin failed")}
	err := NewService(newCostTxDB(t, beginState), &costRepoStub{}).SetCost(context.Background(), SetCostRequest{})
	if err == nil || !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("begin error=%v", err)
	}
	commitState := &costTxState{commitErr: errors.New("commit failed")}
	err = NewService(newCostTxDB(t, commitState), &costRepoStub{}).SetCost(context.Background(), SetCostRequest{})
	if err == nil || !strings.Contains(err.Error(), "commit failed") {
		t.Fatalf("commit error=%v", err)
	}
}

func TestGetCostingServiceNotFoundAndRepositoryErrors(t *testing.T) {
	repo := &costRepoStub{}
	svc := NewService(nil, repo)
	got, err := svc.GetMenuItemCosting(context.Background(), "missing")
	if err != nil || got != nil {
		t.Fatalf("missing cost=(%+v,%v), want nil,nil", got, err)
	}
	repo.getCostErr = errors.New("read failed")
	if _, err = svc.GetMenuItemCosting(context.Background(), "id"); err == nil {
		t.Fatal("expected cost lookup error")
	}
	if repo.priceReads != 0 {
		t.Fatal("price should not be read after cost lookup fails")
	}
}

func TestGetCostingServicePropagatesPriceAndComponentErrors(t *testing.T) {
	repo := &costRepoStub{cost: &MenuItemCost{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 3}}
	svc := NewService(nil, repo)
	repo.priceErr = errors.New("price query failed")
	if _, err := svc.GetMenuItemCosting(context.Background(), repo.cost.MenuItemID); err == nil || !strings.Contains(err.Error(), "price query failed") {
		t.Fatalf("price error=%v", err)
	}
	repo.priceErr = nil
	repo.componentsErr = errors.New("component query failed")
	if _, err := svc.GetMenuItemCosting(context.Background(), repo.cost.MenuItemID); err == nil || !strings.Contains(err.Error(), "component query failed") {
		t.Fatalf("component error=%v", err)
	}
}

func TestCostReportHandlersForwardFiltersAndErrors(t *testing.T) {
	repo := &costRepoStub{cogs: &COGSReportResponse{TotalCOGS: 9}, gross: &GrossProfitReportResponse{GrossProfit: 4}, contribution: &EstimatedContributionResponse{GrossProfit: 4, LaborCost: 2, EstimatedContribution: 2}}
	r := costRouter(NewService(nil, repo))
	for _, path := range []string{"/api/v1/cost/reports/cogs?branch_id=b&from=2026-01-01&to=2026-01-31", "/api/v1/cost/reports/gross-profit?branch_id=b&from=2026-01-01&to=2026-01-31", "/api/v1/cost/reports/estimated-contribution?branch_id=b&from=2026-01-01&to=2026-01-31"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	if repo.filter != (ReportFilter{BranchID: "b", From: "2026-01-01", To: "2026-01-31"}) {
		t.Fatalf("filter=%+v", repo.filter)
	}
	repo.reportErr = errors.New("report query failed")
	for _, path := range []string{"/api/v1/cost/reports/cogs", "/api/v1/cost/reports/gross-profit", "/api/v1/cost/reports/estimated-contribution"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("%s status=%d; want 500", path, w.Code)
		}
	}
}

func TestCostingHandlerMapsNotFoundAndServiceErrors(t *testing.T) {
	repo := &costRepoStub{}
	r := costRouter(NewService(nil, repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/cost/items/11111111-1111-4111-8111-111111111111", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d; want 404", w.Code)
	}
	repo.getCostErr = errors.New("database unavailable")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/cost/items/11111111-1111-4111-8111-111111111111", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d; want 500", w.Code)
	}
}

func TestCostingHandlerReturnsStoredCalculation(t *testing.T) {
	repo := &costRepoStub{cost: &MenuItemCost{MenuItemID: "11111111-1111-4111-8111-111111111111", CostAmount: 40}, price: 100, components: []*RecipeComponent{{GoodID: "22222222-2222-4222-8222-222222222222", Quantity: 2, Unit: "kg"}}}
	r := costRouter(NewService(nil, repo))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/cost/items/11111111-1111-4111-8111-111111111111", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	for _, expected := range []string{`"selling_price":100`, `"cost_amount":40`, `"gross_profit":60`, `"food_cost_pct":40`, `"good_id":"22222222-2222-4222-8222-222222222222"`} {
		if !strings.Contains(w.Body.String(), expected) {
			t.Fatalf("response %s missing %s", w.Body.String(), expected)
		}
	}
}

var _ driver.Rows = (*costRows)(nil)

type costRows struct {
	cols  []string
	data  [][]driver.Value
	index int
}

func (r *costRows) Columns() []string { return r.cols }
func (*costRows) Close() error        { return nil }
func (r *costRows) Next(dst []driver.Value) error {
	if r.index >= len(r.data) {
		return io.EOF
	}
	copy(dst, r.data[r.index])
	r.index++
	return nil
}
