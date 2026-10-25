package inventory

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

const (
	supplyRequestQACentralKitchenID = "99999999-9999-9999-9999-999999999999"
	supplyRequestQABranchID         = "11111111-1111-4111-8111-111111111111"
	supplyRequestQAGoodID           = "22222222-2222-4222-8222-222222222222"
	supplyRequestQAItemID           = "33333333-3333-4333-8333-333333333333"
	qaDriverName                    = "supply-request-noop-transaction"
)

func TestQASupplyRequestLifecycleCreateHandlerUsesSubmittedData(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	repo := &supplyRequestQALifecycleRepository{}
	service := NewService(db, repo)
	body := `{"branch_id":"` + supplyRequestQABranchID + `","notes":"weekly restock","priority":"URGENT","items":[{"good_id":"` + supplyRequestQAGoodID + `","requested_qty":6}]}`
	res := supplyRequestQALifecycleRequest(supplyRequestQALifecycleRouter(service, "manager-1", ""), http.MethodPost, "/api/v1/supply/requests", body)

	if res.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", res.Code, http.StatusCreated, res.Body.String())
	}
	var response SupplyRequestResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("create body is not SupplyRequestResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if response.BranchID != supplyRequestQABranchID || response.Status != "PENDING" || response.Priority != "URGENT" || len(response.Items) != 1 {
		t.Fatalf("create response = %+v, want submitted branch, priority, one item, and PENDING status", response)
	}
	if response.Items[0].GoodID != supplyRequestQAGoodID || response.Items[0].RequestedQty != 6 {
		t.Fatalf("create response item = %+v, want requested good and quantity 6", response.Items[0])
	}
	if repo.createdRequest == nil || repo.createdRequest.CreatedBy != "manager-1" || len(repo.createdItems) != 1 || repo.createdItems[0].RequestedQty != 6 {
		t.Fatalf("repository stub received request/items = %+v/%+v, want submitted request and quantity", repo.createdRequest, repo.createdItems)
	}
	assertSupplyRequestQACommit(t, txStats)
}

func TestQASupplyRequestLifecycleApprovalQuantityAndResponse(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	item := &SupplyRequestItem{ID: supplyRequestQAItemID, GoodID: supplyRequestQAGoodID, RequestedQty: 6}
	repo := &supplyRequestQALifecycleRepository{request: &SupplyRequest{
		ID: "request-1", BranchID: supplyRequestQABranchID, Status: "PENDING", Items: []*SupplyRequestItem{item},
	}}
	service := NewService(db, repo)
	body := `{"items":[{"item_id":"` + supplyRequestQAItemID + `","approved_qty":4}]}`
	res := supplyRequestQALifecycleRequest(
		supplyRequestQALifecycleRouter(service, "", "SUPPLY_MANAGER"),
		http.MethodPatch,
		"/api/v1/supply/requests/request-1/approve",
		body,
	)

	if res.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	var response SupplyRequestResponse
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("approve body is not SupplyRequestResponse JSON: %v; body=%s", err, res.Body.String())
	}
	if response.Status != "APPROVED" || len(response.Items) != 1 || response.Items[0].ApprovedQty == nil || *response.Items[0].ApprovedQty != 4 {
		t.Fatalf("approve response = %+v, want APPROVED with approved quantity 4", response)
	}
	if len(repo.updatedItems) != 1 || repo.updatedItems[0].ApprovedQty == nil || *repo.updatedItems[0].ApprovedQty != 4 {
		t.Fatalf("repository stub updated items = %+v, want approved quantity 4", repo.updatedItems)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != "APPROVED" {
		t.Fatalf("repository status updates = %v, want [APPROVED]", repo.statusUpdates)
	}
	assertSupplyRequestQACommit(t, txStats)
}

func TestQASupplyRequestLifecycleShippingDeductsApprovedQuantity(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	approvedQty := 4.0
	repo := &supplyRequestQALifecycleRepository{
		request: &SupplyRequest{
			ID: "request-1", BranchID: supplyRequestQABranchID, Status: "APPROVED", CreatedBy: "manager-1",
			Items: []*SupplyRequestItem{{ID: supplyRequestQAItemID, GoodID: supplyRequestQAGoodID, ApprovedQty: &approvedQty}},
		},
		inventories: map[[2]string]*Inventory{
			{supplyRequestQACentralKitchenID, supplyRequestQAGoodID}: {
				ID: "central-inventory-1", BranchID: supplyRequestQACentralKitchenID, GoodID: supplyRequestQAGoodID, Quantity: 10,
			},
		},
	}
	service := NewService(db, repo)
	res := supplyRequestQALifecycleRequest(
		supplyRequestQALifecycleRouter(service, "", "SUPPLY_MANAGER"),
		http.MethodPatch,
		"/api/v1/supply/requests/request-1/ship",
		"",
	)

	if res.Code != http.StatusOK {
		t.Fatalf("ship status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("ship body is not JSON: %v; body=%s", err, res.Body.String())
	}
	if response["status"] != "shipped" {
		t.Fatalf("ship response status = %q, want shipped", response["status"])
	}
	if len(repo.inventoryUpdates) != 1 || repo.inventoryUpdates[0].quantity != 6 {
		t.Fatalf("inventory updates = %+v, want Central Kitchen quantity 6", repo.inventoryUpdates)
	}
	if len(repo.movements) != 1 || repo.movements[0].Type != TypeOut || repo.movements[0].Quantity != 4 {
		t.Fatalf("shipment movements = %+v, want one OUT movement of quantity 4", repo.movements)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != "SHIPPED" {
		t.Fatalf("repository status updates = %v, want [SHIPPED]", repo.statusUpdates)
	}
	assertSupplyRequestQACommit(t, txStats)
}

func TestQASupplyRequestLifecycleReceiptAddsReceivedQuantityToBranchStock(t *testing.T) {
	db, txStats := newSupplyRequestQATransactionDB(t)
	approvedQty := 4.0
	item := &SupplyRequestItem{ID: supplyRequestQAItemID, GoodID: supplyRequestQAGoodID, ApprovedQty: &approvedQty}
	repo := &supplyRequestQALifecycleRepository{
		request: &SupplyRequest{
			ID: "request-1", BranchID: supplyRequestQABranchID, Status: "SHIPPED", CreatedBy: "manager-1",
			Items: []*SupplyRequestItem{item},
		},
		inventories: map[[2]string]*Inventory{
			{supplyRequestQABranchID, supplyRequestQAGoodID}: {
				ID: "branch-inventory-1", BranchID: supplyRequestQABranchID, GoodID: supplyRequestQAGoodID, Quantity: 2,
			},
		},
	}
	service := NewService(db, repo)
	body := `{"items":[{"item_id":"` + supplyRequestQAItemID + `","received_qty":3}]}`
	res := supplyRequestQALifecycleRequest(
		supplyRequestQALifecycleRouter(service, "", "BRANCH_MANAGER"),
		http.MethodPatch,
		"/api/v1/supply/requests/request-1/receive",
		body,
	)

	if res.Code != http.StatusOK {
		t.Fatalf("receive status = %d, want %d; body=%s", res.Code, http.StatusOK, res.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("receive body is not JSON: %v; body=%s", err, res.Body.String())
	}
	if response["status"] != "fulfilled" {
		t.Fatalf("receive response status = %q, want fulfilled", response["status"])
	}
	if len(repo.inventoryUpdates) != 1 || repo.inventoryUpdates[0].quantity != 5 {
		t.Fatalf("inventory updates = %+v, want branch quantity 5", repo.inventoryUpdates)
	}
	if len(repo.movements) != 1 || repo.movements[0].Type != TypeIn || repo.movements[0].Quantity != 3 {
		t.Fatalf("receipt movements = %+v, want one IN movement of quantity 3", repo.movements)
	}
	if item.ReceivedQty == nil || *item.ReceivedQty != 3 || len(repo.updatedItems) != 1 || repo.updatedItems[0].ReceivedQty == nil || *repo.updatedItems[0].ReceivedQty != 3 {
		t.Fatalf("receipt item updates = %+v, want received quantity 3", repo.updatedItems)
	}
	if len(repo.statusUpdates) != 1 || repo.statusUpdates[0] != "FULFILLED" {
		t.Fatalf("repository status updates = %v, want [FULFILLED]", repo.statusUpdates)
	}
	assertSupplyRequestQACommit(t, txStats)
}

type supplyRequestQALifecycleRepository struct {
	Repository
	request          *SupplyRequest
	createdRequest   *SupplyRequest
	createdItems     []*SupplyRequestItem
	updatedItems     []*SupplyRequestItem
	statusUpdates    []string
	inventories      map[[2]string]*Inventory
	inventoryUpdates []struct {
		id       string
		quantity float64
	}
	createdInventories []*Inventory
	movements          []*InventoryMovement
}

func (r *supplyRequestQALifecycleRepository) CreateSupplyRequest(_ context.Context, _ *sqlx.Tx, req *SupplyRequest) error {
	r.createdRequest = req
	return nil
}

func (r *supplyRequestQALifecycleRepository) CreateSupplyRequestItem(_ context.Context, _ *sqlx.Tx, item *SupplyRequestItem) error {
	r.createdItems = append(r.createdItems, item)
	return nil
}

func (r *supplyRequestQALifecycleRepository) GetSupplyRequestByID(_ context.Context, _ string) (*SupplyRequest, error) {
	return r.request, nil
}

func (r *supplyRequestQALifecycleRepository) UpdateSupplyRequestItem(_ context.Context, _ *sqlx.Tx, item *SupplyRequestItem) error {
	r.updatedItems = append(r.updatedItems, item)
	return nil
}

func (r *supplyRequestQALifecycleRepository) UpdateSupplyRequestStatus(_ context.Context, _ *sqlx.Tx, _ string, status string) error {
	r.statusUpdates = append(r.statusUpdates, status)
	if r.request != nil {
		r.request.Status = status
	}
	return nil
}

func (r *supplyRequestQALifecycleRepository) GetInventoryByBranchAndGood(_ context.Context, branchID, goodID string) (*Inventory, error) {
	return r.inventories[[2]string{branchID, goodID}], nil
}

func (r *supplyRequestQALifecycleRepository) CreateInventory(_ context.Context, _ *sqlx.Tx, inv *Inventory) error {
	r.createdInventories = append(r.createdInventories, inv)
	if r.inventories == nil {
		r.inventories = make(map[[2]string]*Inventory)
	}
	r.inventories[[2]string{inv.BranchID, inv.GoodID}] = inv
	return nil
}

func (r *supplyRequestQALifecycleRepository) UpdateInventoryQuantity(_ context.Context, _ *sqlx.Tx, id string, quantity float64, updatedAt time.Time) error {
	r.inventoryUpdates = append(r.inventoryUpdates, struct {
		id       string
		quantity float64
	}{id: id, quantity: quantity})
	for _, inv := range r.inventories {
		if inv.ID == id {
			inv.Quantity = quantity
			inv.UpdatedAt = updatedAt
		}
	}
	return nil
}

func (r *supplyRequestQALifecycleRepository) CreateMovement(_ context.Context, _ *sqlx.Tx, mov *InventoryMovement) error {
	r.movements = append(r.movements, mov)
	return nil
}

type supplyRequestQATxStats struct {
	begins  int
	commits int
}

type supplyRequestQATxConnector struct {
	stats *supplyRequestQATxStats
}

func (c supplyRequestQATxConnector) Connect(context.Context) (driver.Conn, error) {
	return &supplyRequestQATxConn{stats: c.stats}, nil
}

func (c supplyRequestQATxConnector) Driver() driver.Driver {
	return supplyRequestQATxDriver{}
}

type supplyRequestQATxDriver struct{}

func (supplyRequestQATxDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("supply request QA connector must be used")
}

type supplyRequestQATxConn struct {
	stats *supplyRequestQATxStats
}

func (c *supplyRequestQATxConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("statement preparation is not supported by supply request QA driver")
}

func (*supplyRequestQATxConn) Close() error { return nil }

func (c *supplyRequestQATxConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *supplyRequestQATxConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.stats.begins++
	return &supplyRequestQATx{stats: c.stats}, nil
}

func (*supplyRequestQATxConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

type supplyRequestQATx struct {
	stats *supplyRequestQATxStats
}

func (*supplyRequestQATx) Rollback() error { return nil }

func (tx *supplyRequestQATx) Commit() error {
	tx.stats.commits++
	return nil
}

func newSupplyRequestQATransactionDB(t *testing.T) (*sqlx.DB, *supplyRequestQATxStats) {
	t.Helper()
	stats := &supplyRequestQATxStats{}
	db := sqlx.NewDb(sql.OpenDB(supplyRequestQATxConnector{stats: stats}), qaDriverName)
	t.Cleanup(func() { _ = db.Close() })
	return db, stats
}

func supplyRequestQALifecycleRouter(svc *Service, userID, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set(CtxUserID, userID)
		}
		if role != "" {
			c.Set(CtxUserRole, role)
		}
		c.Next()
	})
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func supplyRequestQALifecycleRequest(router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}

func assertSupplyRequestQACommit(t *testing.T, stats *supplyRequestQATxStats) {
	t.Helper()
	if stats.begins != 1 || stats.commits != 1 {
		t.Fatalf("fake transaction begins/commits = %d/%d, want 1/1", stats.begins, stats.commits)
	}
}
