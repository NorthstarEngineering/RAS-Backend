package inventory

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

type testTxState struct {
	beginErr, commitErr, execErr error
	begins, commits, rollbacks   int
	execs                        []string
}
type testConnector struct{ state *testTxState }

func (c testConnector) Connect(context.Context) (driver.Conn, error) { return &testConn{c.state}, nil }
func (c testConnector) Driver() driver.Driver                        { return testDriver{c.state} }

type testDriver struct{ state *testTxState }

func (d testDriver) Open(string) (driver.Conn, error) { return &testConn{d.state}, nil }

type testConn struct{ state *testTxState }

func (c *testConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unsupported") }
func (c *testConn) Close() error                        { return nil }
func (c *testConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *testConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.begins++
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	return &testTx{c.state}, nil
}
func (c *testConn) ExecContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.execs = append(c.state.execs, q)
	if c.state.execErr != nil {
		return nil, c.state.execErr
	}
	return driver.RowsAffected(1), nil
}

type testTx struct{ state *testTxState }

func (tx *testTx) Commit() error   { tx.state.commits++; return tx.state.commitErr }
func (tx *testTx) Rollback() error { tx.state.rollbacks++; return nil }

var _ driver.Connector = testConnector{}
var _ driver.ConnBeginTx = (*testConn)(nil)
var _ driver.ExecerContext = (*testConn)(nil)

type fakeRepo struct {
	errs      map[string]error
	inv       map[string]*Inventory
	goods     map[string]*Good
	reqs      map[string]*SupplyRequest
	items     map[string][]*SupplyRequestItem
	movements []*InventoryMovement
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{errs: map[string]error{}, inv: map[string]*Inventory{}, goods: map[string]*Good{}, reqs: map[string]*SupplyRequest{}, items: map[string][]*SupplyRequestItem{}}
}
func (r *fakeRepo) fail(op string) error { return r.errs[op] }
func invKey(b, g string) string          { return b + "|" + g }
func copyInv(v *Inventory) *Inventory {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
func copyGood(v *Good) *Good {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}
func copyItem(v *SupplyRequestItem) *SupplyRequestItem {
	if v == nil {
		return nil
	}
	c := *v
	if v.ApprovedQty != nil {
		x := *v.ApprovedQty
		c.ApprovedQty = &x
	}
	if v.ReceivedQty != nil {
		x := *v.ReceivedQty
		c.ReceivedQty = &x
	}
	return &c
}
func copyReq(v *SupplyRequest) *SupplyRequest {
	if v == nil {
		return nil
	}
	c := *v
	if v.Notes != nil {
		x := *v.Notes
		c.Notes = &x
	}
	c.Items = nil
	return &c
}
func (r *fakeRepo) GetInventoryByBranchAndGood(_ context.Context, b, g string) (*Inventory, error) {
	if e := r.fail("getInventory"); e != nil {
		return nil, e
	}
	return copyInv(r.inv[invKey(b, g)]), nil
}
func (r *fakeRepo) CreateInventory(_ context.Context, _ *sqlx.Tx, v *Inventory) error {
	if e := r.fail("createInventory"); e != nil {
		return e
	}
	r.inv[invKey(v.BranchID, v.GoodID)] = copyInv(v)
	return nil
}
func (r *fakeRepo) UpdateInventoryQuantity(_ context.Context, _ *sqlx.Tx, id string, q float64, at time.Time) error {
	if e := r.fail("updateInventory"); e != nil {
		return e
	}
	for _, v := range r.inv {
		if v.ID == id {
			v.Quantity = q
			v.UpdatedAt = at
			return nil
		}
	}
	return ErrInventoryNotFound
}
func (r *fakeRepo) CreateMovement(_ context.Context, _ *sqlx.Tx, v *InventoryMovement) error {
	if e := r.fail("createMovement"); e != nil {
		return e
	}
	c := *v
	r.movements = append(r.movements, &c)
	return nil
}
func (r *fakeRepo) CreateGood(_ context.Context, _ *sqlx.Tx, v *Good) error {
	if e := r.fail("createGood"); e != nil {
		return e
	}
	r.goods[v.ID] = copyGood(v)
	return nil
}
func (r *fakeRepo) GetGoodByID(_ context.Context, id string) (*Good, error) {
	if e := r.fail("getGood"); e != nil {
		return nil, e
	}
	return copyGood(r.goods[id]), nil
}
func (r *fakeRepo) ListGoods(context.Context) ([]*Good, error) {
	if e := r.fail("listGoods"); e != nil {
		return nil, e
	}
	out := []*Good{}
	for _, v := range r.goods {
		out = append(out, copyGood(v))
	}
	return out, nil
}
func (r *fakeRepo) CreateSupplyRequest(_ context.Context, _ *sqlx.Tx, v *SupplyRequest) error {
	if e := r.fail("createRequest"); e != nil {
		return e
	}
	r.reqs[v.ID] = copyReq(v)
	return nil
}
func (r *fakeRepo) GetSupplyRequestByID(_ context.Context, id string) (*SupplyRequest, error) {
	if e := r.fail("getRequest"); e != nil {
		return nil, e
	}
	v := copyReq(r.reqs[id])
	if v != nil {
		for _, i := range r.items[id] {
			v.Items = append(v.Items, copyItem(i))
		}
	}
	return v, nil
}
func (r *fakeRepo) ListSupplyRequests(context.Context) ([]*SupplyRequest, error) {
	if e := r.fail("listRequests"); e != nil {
		return nil, e
	}
	out := []*SupplyRequest{}
	for _, v := range r.reqs {
		out = append(out, copyReq(v))
	}
	return out, nil
}
func (r *fakeRepo) UpdateSupplyRequestStatus(_ context.Context, _ *sqlx.Tx, id, status string) error {
	if e := r.fail("updateStatus"); e != nil {
		return e
	}
	if v := r.reqs[id]; v != nil {
		v.Status = status
	}
	return nil
}
func (r *fakeRepo) CreateSupplyRequestItem(_ context.Context, _ *sqlx.Tx, v *SupplyRequestItem) error {
	if e := r.fail("createRequestItem"); e != nil {
		return e
	}
	r.items[v.RequestID] = append(r.items[v.RequestID], copyItem(v))
	return nil
}
func (r *fakeRepo) UpdateSupplyRequestItem(_ context.Context, _ *sqlx.Tx, v *SupplyRequestItem) error {
	if e := r.fail("updateRequestItem"); e != nil {
		return e
	}
	for i, old := range r.items[v.RequestID] {
		if old.ID == v.ID {
			r.items[v.RequestID][i] = copyItem(v)
			return nil
		}
	}
	return nil
}
func (r *fakeRepo) ListSupplyRequestItems(_ context.Context, id string) ([]*SupplyRequestItem, error) {
	if e := r.fail("listRequestItems"); e != nil {
		return nil, e
	}
	out := []*SupplyRequestItem{}
	for _, v := range r.items[id] {
		out = append(out, copyItem(v))
	}
	return out, nil
}

var _ Repository = (*fakeRepo)(nil)

type testHarness struct {
	svc  *Service
	repo *fakeRepo
	tx   *testTxState
}

func newTestHarness(t *testing.T) *testHarness {
	t.Helper()
	repo := newFakeRepo()
	state := &testTxState{}
	db := sqlx.NewDb(sql.OpenDB(testConnector{state}), "test")
	t.Cleanup(func() { _ = db.Close() })
	return &testHarness{NewService(db, repo), repo, state}
}
func putInventory(r *fakeRepo, b, g string, q float64) *Inventory {
	v := &Inventory{ID: "inv-" + b + "-" + g, BranchID: b, GoodID: g, Quantity: q, UpdatedAt: time.Unix(1, 0)}
	r.inv[invKey(b, g)] = v
	return v
}
func putRequest(r *fakeRepo, id, b, status string, items ...*SupplyRequestItem) *SupplyRequest {
	v := &SupplyRequest{ID: id, BranchID: b, Status: status, CreatedBy: "creator", Priority: "NORMAL", CreatedAt: time.Unix(2, 0)}
	r.reqs[id] = copyReq(v)
	for _, i := range items {
		i.RequestID = id
		r.items[id] = append(r.items[id], copyItem(i))
	}
	v.Items = items
	return v
}
func floatPtr(v float64) *float64 { return &v }

func TestMovementTypeIsValid(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  MovementType
		want bool
	}{{"IN", TypeIn, true}, {"OUT", TypeOut, true}, {"ADJUSTMENT", TypeAdjustment, true}, {"unknown", "bad", false}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.typ.IsValid(); got != tc.want {
				t.Fatalf("IsValid()=%v want %v", got, tc.want)
			}
		})
	}
}
func TestRecordMovementValidatesInputs(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   RecordMovementInput
		want error
	}{
		{"invalid type", RecordMovementInput{Type: "bad", Quantity: 1}, ErrInvalidMovementType},
		{"zero quantity", RecordMovementInput{Type: "IN"}, ErrInvalidQuantity},
		{"negative quantity", RecordMovementInput{Type: "IN", Quantity: -1}, ErrInvalidQuantity},
		{"OUT reason required", RecordMovementInput{Type: "OUT", Quantity: 1}, ErrReasonRequired},
		{"ADJUSTMENT reason required", RecordMovementInput{Type: "ADJUSTMENT", Quantity: 1}, ErrReasonRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			got, err := h.svc.RecordMovement(context.Background(), tc.in)
			if got != nil || !errors.Is(err, tc.want) {
				t.Fatalf("got (%v,%v), want nil and %v", got, err, tc.want)
			}
			if h.tx.begins != 0 {
				t.Fatal("invalid input began a transaction")
			}
		})
	}
}
func TestRecordMovementAppliesSupportedTypes(t *testing.T) {
	for _, tc := range []struct {
		name, typ, reason  string
		initial, qty, want float64
		newInventory       bool
	}{
		{"IN adds quantity", "IN", "", 2, 3, 5, false}, {"OUT allows exact zero", "OUT", "use", 5, 5, 0, false},
		{"ADJUSTMENT sets absolute quantity", "ADJUSTMENT", "count", 2, 7, 7, false}, {"IN creates inventory", "IN", "", 0, 4, 4, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.newInventory {
				putInventory(h.repo, "b", "g", tc.initial)
			}
			got, err := h.svc.RecordMovement(context.Background(), RecordMovementInput{BranchID: "b", ActorID: "actor", GoodID: "g", Type: tc.typ, Quantity: tc.qty, Reason: tc.reason})
			if err != nil {
				t.Fatal(err)
			}
			if got.Inventory.Quantity != tc.want || got.Inventory.BranchID != "b" || got.Inventory.GoodID != "g" {
				t.Fatalf("inventory response=%#v", got.Inventory)
			}
			if got.Movement.Type != tc.typ || got.Movement.Quantity != tc.qty || got.Movement.Reason != tc.reason || got.Movement.CreatedBy != "actor" {
				t.Fatalf("movement response=%#v", got.Movement)
			}
			if len(h.repo.movements) != 1 || h.tx.commits != 1 {
				t.Fatalf("movement count=%d commits=%d", len(h.repo.movements), h.tx.commits)
			}
		})
	}
}
func TestRecordMovementRejectsInsufficientStock(t *testing.T) {
	h := newTestHarness(t)
	putInventory(h.repo, "b", "g", 2)
	got, err := h.svc.RecordMovement(context.Background(), RecordMovementInput{BranchID: "b", GoodID: "g", Type: "OUT", Quantity: 3, Reason: "issue"})
	if got != nil || !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("got (%v,%v), want insufficient stock", got, err)
	}
	if len(h.repo.movements) != 0 || h.tx.commits != 0 {
		t.Fatal("insufficient movement was persisted")
	}
}
func TestRecordMovementPropagatesFailures(t *testing.T) {
	failure := errors.New("movement storage failure")
	for _, tc := range []struct {
		name, op                        string
		begin, commit, missingInventory bool
	}{
		{"begin", "", true, false, false}, {"inventory lookup", "getInventory", false, false, false},
		{"create inventory", "createInventory", false, false, true}, {"update inventory", "updateInventory", false, false, false},
		{"create movement", "createMovement", false, false, false}, {"commit", "", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.missingInventory {
				putInventory(h.repo, "b", "g", 1)
			}
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			_, err := h.svc.RecordMovement(context.Background(), RecordMovementInput{BranchID: "b", GoodID: "g", Type: "IN", Quantity: 1})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}

func TestCreateGoodValidationAndOptionalFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   CreateGoodInput
	}{
		{"missing name", CreateGoodInput{SKU: "s", Unit: "kg"}}, {"missing SKU", CreateGoodInput{Name: "n", Unit: "kg"}},
		{"missing unit", CreateGoodInput{SKU: "s", Name: "n"}}, {"negative cost", CreateGoodInput{SKU: "s", Name: "n", Unit: "kg", UnitCost: -1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			got, err := h.svc.CreateGood(context.Background(), tc.in)
			if got != nil || !errors.Is(err, ErrInvalidQuantity) {
				t.Fatalf("got (%v,%v), want ErrInvalidQuantity", got, err)
			}
			if h.tx.begins != 0 {
				t.Fatal("invalid good began a transaction")
			}
		})
	}
	h := newTestHarness(t)
	got, err := h.svc.CreateGood(context.Background(), CreateGoodInput{SKU: "flour", Name: "Flour", Category: "dry", Description: "white", Unit: "kg", UnitCost: 0, ReorderLevel: 2, MaxCapacity: 10})
	if err != nil {
		t.Fatal(err)
	}
	if got.SKU != "flour" || got.Name != "Flour" || !got.IsActive || got.UnitCost != 0 || got.Category == nil || *got.Category != "dry" || got.Description == nil || *got.Description != "white" || got.MaxCapacity == nil || *got.MaxCapacity != 10 {
		t.Fatalf("unexpected good: %#v", got)
	}
	if got.UpdatedAt.IsZero() || got.CreatedAt.IsZero() || h.tx.commits != 1 {
		t.Fatal("creation timestamps or commit missing")
	}
	if got2, err := h.svc.CreateGood(context.Background(), CreateGoodInput{SKU: "s2", Name: "N", Unit: "kg"}); err != nil || got2.Category != nil || got2.Description != nil || got2.MaxCapacity != nil {
		t.Fatalf("empty optional fields should be nil: (%#v,%v)", got2, err)
	}
}
func TestCreateGoodPropagatesFailures(t *testing.T) {
	failure := errors.New("good write failed")
	for _, tc := range []struct {
		name, op      string
		begin, commit bool
	}{
		{"begin", "", true, false}, {"repository", "createGood", false, false}, {"commit", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			_, err := h.svc.CreateGood(context.Background(), CreateGoodInput{SKU: "s", Name: "n", Unit: "kg"})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}
func TestGetAndListGoods(t *testing.T) {
	h := newTestHarness(t)
	h.repo.goods["g"] = &Good{ID: "g", SKU: "s", Name: "N"}
	got, err := h.svc.GetGoodByID(context.Background(), "g")
	if err != nil || got.ID != "g" {
		t.Fatalf("found good=(%v,%v)", got, err)
	}
	missing, err := h.svc.GetGoodByID(context.Background(), "missing")
	if missing != nil || !errors.Is(err, ErrGoodNotFound) {
		t.Fatalf("missing good=(%v,%v)", missing, err)
	}
	list, err := h.svc.ListGoods(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != "g" {
		t.Fatalf("goods=(%v,%v)", list, err)
	}
	h.repo.errs["getGood"] = errors.New("get failed")
	if _, err = h.svc.GetGoodByID(context.Background(), "g"); err == nil {
		t.Fatal("GetGoodByID swallowed repository error")
	}
	h.repo.errs["listGoods"] = errors.New("list failed")
	if _, err = h.svc.ListGoods(context.Background()); err == nil {
		t.Fatal("ListGoods swallowed repository error")
	}
}

func TestCreateSupplyRequestValidatesAndPersistsMultipleItems(t *testing.T) {
	h := newTestHarness(t)
	for _, in := range []CreateSupplyRequestInput{{Items: []SupplyRequestItemInput{{GoodID: "g", RequestedQty: 1}}}, {BranchID: "b"}} {
		if got, err := h.svc.CreateSupplyRequest(context.Background(), in); got != nil || err == nil {
			t.Fatalf("invalid request=(%v,%v)", got, err)
		}
	}
	if h.tx.begins != 0 {
		t.Fatal("invalid request began a transaction")
	}
	got, err := h.svc.CreateSupplyRequest(context.Background(), CreateSupplyRequestInput{BranchID: "b", CreatedBy: "actor", Notes: "weekly", Priority: "URGENT", Items: []SupplyRequestItemInput{{GoodID: "g1", RequestedQty: 2}, {GoodID: "g2", RequestedQty: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "PENDING" || got.BranchID != "b" || got.CreatedBy != "actor" || got.Priority != "URGENT" || got.Notes == nil || *got.Notes != "weekly" || len(got.Items) != 2 || got.Items[1].GoodID != "g2" || got.Items[1].RequestedQty != 4 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if len(h.repo.items[got.ID]) != 2 || h.tx.commits != 1 {
		t.Fatal("request and items were not persisted")
	}
}
func TestCreateSupplyRequestPropagatesFailures(t *testing.T) {
	failure := errors.New("request write failed")
	for _, tc := range []struct {
		name, op      string
		begin, commit bool
	}{
		{"begin", "", true, false}, {"request row", "createRequest", false, false}, {"request item", "createRequestItem", false, false}, {"commit", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			_, err := h.svc.CreateSupplyRequest(context.Background(), CreateSupplyRequestInput{BranchID: "b", Items: []SupplyRequestItemInput{{GoodID: "g", RequestedQty: 1}}})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}

func TestApproveSupplyRequestRejectsUnauthorizedMissingAndWrongState(t *testing.T) {
	for _, tc := range []struct {
		name, role, status string
		missing            bool
		want               error
	}{
		{"unauthorized", "BRANCH_MANAGER", "PENDING", false, ErrForbiddenAction},
		{"missing", "CEO", "", true, errors.New("supply request not found")},
		{"wrong state", "CEO", "SHIPPED", false, errors.New("request is not pending")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.missing {
				putRequest(h.repo, "r", "b", tc.status)
			}
			_, err := h.svc.ApproveSupplyRequest(context.Background(), ApproveSupplyRequestInput{RequestID: "r", ActorRole: tc.role})
			if err == nil || err.Error() != tc.want.Error() {
				t.Fatalf("error=%v want %v", err, tc.want)
			}
			if h.tx.begins != 0 {
				t.Fatal("rejected approval began transaction")
			}
		})
	}
}
func TestApproveSupplyRequestUpdatesSelectedItems(t *testing.T) {
	for _, role := range []string{"CEO", "SUPPLY_MANAGER"} {
		t.Run(role, func(t *testing.T) {
			h := newTestHarness(t)
			putRequest(h.repo, "r", "b", "PENDING", &SupplyRequestItem{ID: "i1", GoodID: "g1", RequestedQty: 4}, &SupplyRequestItem{ID: "i2", GoodID: "g2", RequestedQty: 3})
			got, err := h.svc.ApproveSupplyRequest(context.Background(), ApproveSupplyRequestInput{RequestID: "r", ActorRole: role, Items: []ApproveItemInput{{ItemID: "i1", ApprovedQty: 2}, {ItemID: "unknown", ApprovedQty: 7}}})
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "APPROVED" || got.Items[0].ApprovedQty == nil || *got.Items[0].ApprovedQty != 2 || got.Items[1].ApprovedQty != nil {
				t.Fatalf("unexpected approval: %#v", got)
			}
			if h.repo.reqs["r"].Status != "APPROVED" || h.tx.commits != 1 {
				t.Fatal("approval status not persisted")
			}
		})
	}
}
func TestApproveSupplyRequestPropagatesFailures(t *testing.T) {
	failure := errors.New("approval write failed")
	for _, tc := range []struct {
		name, op      string
		begin, commit bool
	}{
		{"read", "getRequest", false, false}, {"begin", "", true, false}, {"item update", "updateRequestItem", false, false},
		{"status update", "updateStatus", false, false}, {"commit", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			putRequest(h.repo, "r", "b", "PENDING", &SupplyRequestItem{ID: "i", GoodID: "g"})
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			_, err := h.svc.ApproveSupplyRequest(context.Background(), ApproveSupplyRequestInput{RequestID: "r", ActorRole: "CEO", Items: []ApproveItemInput{{ItemID: "i", ApprovedQty: 1}}})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}

func TestRejectSupplyRequestRejectsUnauthorizedInvalidAndWrongState(t *testing.T) {
	for _, tc := range []struct {
		name, role, reason, status string
		missing                    bool
		want                       string
	}{
		{"unauthorized", "BRANCH_MANAGER", "reason", "PENDING", false, ErrForbiddenAction.Error()},
		{"empty reason", "CEO", "", "PENDING", false, "rejection reason is required"},
		{"missing request", "CEO", "reason", "", true, "supply request not found"},
		{"wrong state", "CEO", "reason", "APPROVED", false, "request is not pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.missing {
				putRequest(h.repo, "r", "b", tc.status)
			}
			_, err := h.svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{RequestID: "r", ActorRole: tc.role, Reason: tc.reason})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if h.tx.begins != 0 {
				t.Fatal("rejected request began transaction")
			}
		})
	}
}
func TestRejectSupplyRequestStoresReasonAndStatus(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "PENDING")
	got, err := h.svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{RequestID: "r", ActorRole: "SUPPLY_MANAGER", Reason: "out of range"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "REJECTED" || got.Notes == nil || *got.Notes != "out of range" || h.repo.reqs["r"].Status != "REJECTED" {
		t.Fatalf("unexpected rejected request: %#v", got)
	}
	if len(h.tx.execs) != 1 {
		t.Fatalf("notes update executions=%d want 1", len(h.tx.execs))
	}
}
func TestRejectSupplyRequestPropagatesFailures(t *testing.T) {
	failure := errors.New("rejection write failed")
	for _, tc := range []struct {
		name, op            string
		begin, commit, exec bool
	}{
		{"read", "getRequest", false, false, false}, {"begin", "", true, false, false}, {"status update", "updateStatus", false, false, false},
		{"notes SQL", "", false, false, true}, {"commit", "", false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			putRequest(h.repo, "r", "b", "PENDING")
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.exec {
				h.tx.execErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			_, err := h.svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{RequestID: "r", ActorRole: "CEO", Reason: "reason"})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}

func TestShipSupplyRequestRejectsUnauthorizedMissingAndWrongState(t *testing.T) {
	for _, tc := range []struct {
		name, role, status string
		missing            bool
		want               string
	}{
		{"unauthorized", "BRANCH_MANAGER", "APPROVED", false, ErrForbiddenAction.Error()},
		{"missing", "CEO", "", true, "request not found"},
		{"wrong state", "CEO", "PENDING", false, "request must be approved first"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.missing {
				putRequest(h.repo, "r", "b", tc.status)
			}
			err := h.svc.ShipSupplyRequest(context.Background(), tc.role, "r")
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if h.tx.begins != 0 {
				t.Fatal("invalid shipment began transaction")
			}
		})
	}
}
func TestShipSupplyRequestDeductsStockForApprovedItems(t *testing.T) {
	h := newTestHarness(t)
	central := "99999999-9999-9999-9999-999999999999"
	putInventory(h.repo, central, "g1", 8)
	putInventory(h.repo, central, "g2", 2)
	putRequest(h.repo, "r", "branch", "APPROVED", &SupplyRequestItem{ID: "i1", GoodID: "g1", ApprovedQty: floatPtr(3)}, &SupplyRequestItem{ID: "i2", GoodID: "g2", ApprovedQty: floatPtr(1)}, &SupplyRequestItem{ID: "i3", GoodID: "g3"})
	if err := h.svc.ShipSupplyRequest(context.Background(), "SUPPLY_MANAGER", "r"); err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey(central, "g1")].Quantity != 5 || h.repo.inv[invKey(central, "g2")].Quantity != 1 {
		t.Fatal("central inventory quantities are incorrect")
	}
	if len(h.repo.movements) != 2 || h.repo.movements[0].Type != TypeOut || h.repo.movements[0].Quantity != 3 || h.repo.movements[0].Reason != "Supply Shipment to Branch" || h.repo.movements[1].Quantity != 1 {
		t.Fatalf("shipment movements=%#v", h.repo.movements)
	}
	if h.repo.reqs["r"].Status != "SHIPPED" || h.tx.commits != 1 {
		t.Fatal("shipment status not committed")
	}
}
func TestShipSupplyRequestRejectsInsufficientStock(t *testing.T) {
	h := newTestHarness(t)
	central := "99999999-9999-9999-9999-999999999999"
	putInventory(h.repo, central, "g", 2)
	putRequest(h.repo, "r", "b", "APPROVED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(3)})
	if err := h.svc.ShipSupplyRequest(context.Background(), "CEO", "r"); !errors.Is(err, ErrInsufficientStock) {
		t.Fatalf("error=%v want insufficient stock", err)
	}
	if h.repo.reqs["r"].Status != "APPROVED" || len(h.repo.movements) != 0 {
		t.Fatal("insufficient shipment was recorded")
	}
}
func TestShipSupplyRequestPropagatesFailures(t *testing.T) {
	failure := errors.New("shipment write failed")
	central := "99999999-9999-9999-9999-999999999999"
	for _, tc := range []struct {
		name, op      string
		begin, commit bool
	}{
		{"begin", "", true, false}, {"inventory read", "getInventory", false, false}, {"stock update", "updateInventory", false, false},
		{"movement write", "createMovement", false, false}, {"status update", "updateStatus", false, false}, {"commit", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			putInventory(h.repo, central, "g", 5)
			putRequest(h.repo, "r", "b", "APPROVED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(1)})
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			err := h.svc.ShipSupplyRequest(context.Background(), "CEO", "r")
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}

func TestReceiveSupplyRequestRejectsUnauthorizedMissingAndWrongState(t *testing.T) {
	for _, tc := range []struct {
		name, role, status string
		missing            bool
		want               string
	}{
		{"unauthorized", "SUPPLY_MANAGER", "SHIPPED", false, ErrForbiddenAction.Error()},
		{"missing", "CEO", "", true, "request not found"},
		{"wrong state", "CEO", "APPROVED", false, "request must be shipped first"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			if !tc.missing {
				putRequest(h.repo, "r", "b", tc.status)
			}
			err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{RequestID: "r", ActorRole: tc.role})
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want %q", err, tc.want)
			}
			if h.tx.begins != 0 {
				t.Fatal("invalid receipt began transaction")
			}
		})
	}
}
func TestReceiveSupplyRequestAddsPartialAndApprovedQuantities(t *testing.T) {
	h := newTestHarness(t)
	putInventory(h.repo, "branch", "g1", 4)
	putRequest(h.repo, "r", "branch", "SHIPPED",
		&SupplyRequestItem{ID: "i1", GoodID: "g1", RequestedQty: 5, ApprovedQty: floatPtr(4)},
		&SupplyRequestItem{ID: "i2", GoodID: "g2", RequestedQty: 3, ApprovedQty: floatPtr(3)})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{RequestID: "r", ActorRole: "BRANCH_MANAGER", Items: []ReceiveItemInput{{ItemID: "i1", ReceivedQty: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey("branch", "g1")].Quantity != 6 || h.repo.inv[invKey("branch", "g2")].Quantity != 3 {
		t.Fatal("received inventory quantities are incorrect")
	}
	if len(h.repo.movements) != 2 || h.repo.movements[0].Type != TypeIn || h.repo.movements[0].Quantity != 2 || h.repo.movements[1].Quantity != 3 {
		t.Fatalf("receipt movements=%#v", h.repo.movements)
	}
	got, _ := h.repo.GetSupplyRequestByID(context.Background(), "r")
	if got.Status != "FULFILLED" || got.Items[0].ReceivedQty == nil || *got.Items[0].ReceivedQty != 2 || got.Items[1].ReceivedQty == nil || *got.Items[1].ReceivedQty != 3 {
		t.Fatalf("unexpected received request: %#v", got)
	}
}
func TestReceiveSupplyRequestPropagatesFailures(t *testing.T) {
	failure := errors.New("receipt write failed")
	for _, tc := range []struct {
		name, op      string
		begin, commit bool
	}{
		{"begin", "", true, false}, {"inventory read", "getInventory", false, false}, {"create inventory", "createInventory", false, false},
		{"inventory update", "updateInventory", false, false}, {"movement", "createMovement", false, false},
		{"item update", "updateRequestItem", false, false}, {"status update", "updateStatus", false, false}, {"commit", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t)
			putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(1)})
			if tc.op != "getInventory" && tc.op != "createInventory" {
				putInventory(h.repo, "b", "g", 1)
			}
			if tc.begin {
				h.tx.beginErr = failure
			}
			if tc.commit {
				h.tx.commitErr = failure
			}
			if tc.op != "" {
				h.repo.errs[tc.op] = failure
			}
			err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{{ItemID: "i", ReceivedQty: 1}}})
			if !errors.Is(err, failure) {
				t.Fatalf("error=%v want %v", err, failure)
			}
		})
	}
}
func TestResponseMappersNormalizeUTCAndKeepItems(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("UTC+3", 3*60*60))
	inv := (&Inventory{ID: "i", BranchID: "b", GoodID: "g", UpdatedAt: at}).ToResponse()
	mov := (&InventoryMovement{ID: "m", InventoryID: "i", Type: TypeOut, CreatedAt: at}).ToResponse()
	req := (&SupplyRequest{ID: "r", Status: "APPROVED", CreatedAt: at, Items: []*SupplyRequestItem{{ID: "ri", GoodID: "g", RequestedQty: 2, ApprovedQty: floatPtr(1)}}}).ToResponse()
	want := "2026-01-02T00:04:05Z"
	if inv.UpdatedAt != want || mov.CreatedAt != want || req.CreatedAt != want {
		t.Fatalf("timestamps inv=%q movement=%q request=%q want %q", inv.UpdatedAt, mov.CreatedAt, req.CreatedAt, want)
	}
	if req.Status != "APPROVED" || len(req.Items) != 1 || req.Items[0].ApprovedQty == nil || *req.Items[0].ApprovedQty != 1 {
		t.Fatalf("request response lost fields: %#v", req)
	}
}

func TestRejectSupplyRequestReturnsDatabaseExecError(t *testing.T) {
	h := newTestHarness(t)
	failure := errors.New("notes update failed")
	h.tx.execErr = failure
	putRequest(h.repo, "r", "b", "PENDING")
	_, err := h.svc.RejectSupplyRequest(context.Background(), RejectSupplyRequestInput{RequestID: "r", ActorRole: "CEO", Reason: "reason"})
	if !errors.Is(err, failure) {
		t.Fatalf("error=%v want %v", err, failure)
	}
}

func TestListSupplyRequests(t *testing.T) {
	h := newTestHarness(t)
	notes := "weekly request"
	h.repo.reqs["r1"] = &SupplyRequest{ID: "r1", BranchID: "b1", Status: "PENDING", CreatedBy: "manager-1", Priority: "URGENT", Notes: &notes, CreatedAt: time.Unix(10, 0)}
	putRequest(h.repo, "r2", "b2", "SHIPPED", &SupplyRequestItem{ID: "i2", GoodID: "g2", RequestedQty: 4})

	got, err := h.svc.ListSupplyRequests(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("ListSupplyRequests()=(%v,%v), want two requests", got, err)
	}
	byID := make(map[string]*SupplyRequest, len(got))
	for _, req := range got {
		byID[req.ID] = req
	}
	first := byID["r1"]
	if first == nil || first.BranchID != "b1" || first.Status != "PENDING" || first.CreatedBy != "manager-1" || first.Priority != "URGENT" || first.Notes == nil || *first.Notes != notes {
		t.Fatalf("request fields were not preserved: %#v", first)
	}
	second := byID["r2"]
	if second == nil || second.BranchID != "b2" || second.Status != "SHIPPED" {
		t.Fatalf("second request fields were not preserved: %#v", second)
	}
	// The production ListSupplyRequests repository query does not load request items.
	if len(second.Items) != 0 {
		t.Fatalf("list unexpectedly included items: %#v", second.Items)
	}

	h.repo.errs["listRequests"] = errors.New("list failed")
	if _, err := h.svc.ListSupplyRequests(context.Background()); err == nil {
		t.Fatal("ListSupplyRequests swallowed repository error")
	}
}

func TestCreateGoodFieldHandling(t *testing.T) {
	h := newTestHarness(t)
	got, err := h.svc.CreateGood(context.Background(), CreateGoodInput{
		SKU: "oil", Name: "Oil", Unit: "l", UnitCost: 4.25,
		ReorderLevel: -2, MaxCapacity: -1, InitialQty: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitCost != 4.25 {
		t.Fatalf("positive UnitCost = %v, want 4.25", got.UnitCost)
	}
	if got.ReorderLevel != -2 {
		t.Fatalf("ReorderLevel = %v, want implementation-preserved -2", got.ReorderLevel)
	}
	if got.MaxCapacity != nil {
		t.Fatalf("nonpositive MaxCapacity should map to nil in the current service, got %v", *got.MaxCapacity)
	}
	// InitialQty is accepted by the input but CreateGood does not create inventory.
	if len(h.repo.inv) != 0 {
		t.Fatalf("CreateGood created inventory for InitialQty: %#v", h.repo.inv)
	}
}

func TestReceiveSupplyRequestZeroQuantityFallsBackToApprovedQuantity(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(5)})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{
		RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{{ItemID: "i", ReceivedQty: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey("b", "g")] == nil || h.repo.inv[invKey("b", "g")].Quantity != 5 {
		t.Fatalf("explicit zero did not fall back to approved quantity: %#v", h.repo.inv)
	}
	if len(h.repo.movements) != 1 || h.repo.movements[0].Quantity != 5 {
		t.Fatalf("movement quantity = %#v, want approved quantity 5", h.repo.movements)
	}
}

func TestReceiveSupplyRequestNegativeQuantityIsSkippedByService(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g"})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{
		RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{{ItemID: "i", ReceivedQty: -2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.repo.movements) != 0 || len(h.repo.inv) != 0 {
		t.Fatalf("negative receipt changed inventory: movements=%#v inventory=%#v", h.repo.movements, h.repo.inv)
	}
	if h.repo.reqs["r"].Status != "FULFILLED" || h.tx.commits != 1 {
		t.Fatal("service did not follow its current zero-or-negative receipt path")
	}
}

func TestReceiveSupplyRequestEmptyItemListFallsBackToApprovedQuantities(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED",
		&SupplyRequestItem{ID: "i1", GoodID: "g1", ApprovedQty: floatPtr(2)},
		&SupplyRequestItem{ID: "i2", GoodID: "g2", ApprovedQty: floatPtr(3)})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{RequestID: "r", ActorRole: "BRANCH_MANAGER"})
	if err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey("b", "g1")].Quantity != 2 || h.repo.inv[invKey("b", "g2")].Quantity != 3 {
		t.Fatalf("empty receipt did not use approved quantities: %#v", h.repo.inv)
	}
	if len(h.repo.movements) != 2 || h.repo.reqs["r"].Status != "FULFILLED" {
		t.Fatalf("unexpected empty-list fallback result: movements=%d status=%s", len(h.repo.movements), h.repo.reqs["r"].Status)
	}
}

func TestReceiveSupplyRequestIgnoresUnknownItemID(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g"})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{
		RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{{ItemID: "unknown", ReceivedQty: 7}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(h.repo.movements) != 0 || len(h.repo.inv) != 0 || h.repo.reqs["r"].Status != "FULFILLED" {
		t.Fatalf("unknown item changed receipt state: movements=%#v inventory=%#v request=%#v", h.repo.movements, h.repo.inv, h.repo.reqs["r"])
	}
}

func TestReceiveSupplyRequestUsesExplicitQuantityWithoutApprovedQuantity(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: nil})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{
		RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{{ItemID: "i", ReceivedQty: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey("b", "g")] == nil || h.repo.inv[invKey("b", "g")].Quantity != 2 || len(h.repo.movements) != 1 {
		t.Fatalf("explicit receipt without approval was not applied: inventory=%#v movements=%#v", h.repo.inv, h.repo.movements)
	}
}

func TestReceiveSupplyRequestDuplicateItemIDsUseLastQuantity(t *testing.T) {
	h := newTestHarness(t)
	putRequest(h.repo, "r", "b", "SHIPPED", &SupplyRequestItem{ID: "i", GoodID: "g", ApprovedQty: floatPtr(10)})
	err := h.svc.ReceiveSupplyRequest(context.Background(), ReceiveSupplyRequestInput{
		RequestID: "r", ActorRole: "CEO", Items: []ReceiveItemInput{
			{ItemID: "i", ReceivedQty: 2},
			{ItemID: "i", ReceivedQty: 3},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.repo.inv[invKey("b", "g")] == nil || h.repo.inv[invKey("b", "g")].Quantity != 3 || len(h.repo.movements) != 1 || h.repo.movements[0].Quantity != 3 {
		t.Fatalf("duplicate IDs did not use the final quantity: inventory=%#v movements=%#v", h.repo.inv, h.repo.movements)
	}
}

func TestRecordMovementRollsBackDriverTransactionOnRepositoryError(t *testing.T) {
	h := newTestHarness(t)
	putInventory(h.repo, "b", "g", 2)
	h.repo.errs["createMovement"] = errors.New("movement insert failed")
	_, err := h.svc.RecordMovement(context.Background(), RecordMovementInput{BranchID: "b", GoodID: "g", Type: "IN", Quantity: 1})
	if err == nil {
		t.Fatal("RecordMovement returned nil error")
	}
	if h.tx.begins != 1 || h.tx.rollbacks != 1 || h.tx.commits != 0 {
		t.Fatalf("driver transaction counts: begin=%d rollback=%d commit=%d", h.tx.begins, h.tx.rollbacks, h.tx.commits)
	}
	// This checks the driver's Rollback call only; fakeRepo state is not transactional.
}
