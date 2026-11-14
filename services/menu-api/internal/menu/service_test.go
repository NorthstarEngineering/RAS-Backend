package menu

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"

	"github.com/jmoiron/sqlx"
)

type menuTxState struct {
	beginErr, commitErr        error
	begins, commits, rollbacks int
}
type menuConnector struct{ state *menuTxState }

func (c menuConnector) Connect(context.Context) (driver.Conn, error) { return menuConn{c.state}, nil }
func (c menuConnector) Driver() driver.Driver                        { return menuDriver{} }

type menuDriver struct{}

func (menuDriver) Open(string) (driver.Conn, error) { return nil, errors.New("connector required") }

type menuConn struct{ state *menuTxState }

func (c menuConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (c menuConn) Close() error                        { return nil }
func (c menuConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c menuConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.begins++
	if c.state.beginErr != nil {
		return nil, c.state.beginErr
	}
	return &menuTx{c.state}, nil
}

type menuTx struct{ state *menuTxState }

func (tx *menuTx) Commit() error   { tx.state.commits++; return tx.state.commitErr }
func (tx *menuTx) Rollback() error { tx.state.rollbacks++; return nil }

type fakeMenuRepo struct {
	Repository
	errs             map[string]error
	items            map[string]*MenuItem
	categories       []*Category
	requests         map[string]*MenuItemRequest
	listedItems      []*MenuItem
	listedRequests   []*MenuItemRequest
	listedCategories []*Category
	createdItem      *MenuItem
	updatedItem      *MenuItem
	createdCategory  *Category
	createdRequest   *MenuItemRequest
	updatedRequest   *MenuItemRequest
}

func newFakeMenuRepo() *fakeMenuRepo {
	return &fakeMenuRepo{errs: map[string]error{}, items: map[string]*MenuItem{}, requests: map[string]*MenuItemRequest{}}
}
func (r *fakeMenuRepo) CreateCategory(_ context.Context, _ *sqlx.Tx, c *Category) error {
	r.createdCategory = c
	return r.errs["CreateCategory"]
}
func (r *fakeMenuRepo) ListCategories(context.Context) ([]*Category, error) {
	return r.listedCategories, r.errs["ListCategories"]
}
func (r *fakeMenuRepo) CreateMenuItem(_ context.Context, _ *sqlx.Tx, i *MenuItem) error {
	r.createdItem = i
	return r.errs["CreateMenuItem"]
}
func (r *fakeMenuRepo) GetMenuItemByID(_ context.Context, id string) (*MenuItem, error) {
	if err := r.errs["GetMenuItemByID"]; err != nil {
		return nil, err
	}
	i := r.items[id]
	if i == nil {
		return nil, nil
	}
	copy := *i
	return &copy, nil
}
func (r *fakeMenuRepo) ListMenuItems(context.Context) ([]*MenuItem, error) {
	return r.listedItems, r.errs["ListMenuItems"]
}
func (r *fakeMenuRepo) UpdateMenuItem(_ context.Context, _ *sqlx.Tx, i *MenuItem) error {
	r.updatedItem = i
	return r.errs["UpdateMenuItem"]
}
func (r *fakeMenuRepo) CreateMenuItemRequest(_ context.Context, _ *sqlx.Tx, q *MenuItemRequest) error {
	r.createdRequest = q
	return r.errs["CreateMenuItemRequest"]
}
func (r *fakeMenuRepo) GetMenuItemRequestByID(_ context.Context, id string) (*MenuItemRequest, error) {
	if err := r.errs["GetMenuItemRequestByID"]; err != nil {
		return nil, err
	}
	q := r.requests[id]
	if q == nil {
		return nil, nil
	}
	copy := *q
	return &copy, nil
}
func (r *fakeMenuRepo) ListMenuItemRequests(context.Context) ([]*MenuItemRequest, error) {
	return r.listedRequests, r.errs["ListMenuItemRequests"]
}
func (r *fakeMenuRepo) UpdateMenuItemRequest(_ context.Context, _ *sqlx.Tx, q *MenuItemRequest) error {
	r.updatedRequest = q
	return r.errs["UpdateMenuItemRequest"]
}

func menuHarness(t *testing.T) (*Service, *fakeMenuRepo, *menuTxState) {
	t.Helper()
	state := &menuTxState{}
	repo := newFakeMenuRepo()
	db := sqlx.NewDb(sql.OpenDB(menuConnector{state}), "postgres")
	t.Cleanup(func() { _ = db.Close() })
	return NewService(db, repo), repo, state
}
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }

func TestMenuItemListGetAndRepositoryErrors(t *testing.T) {
	s, r, _ := menuHarness(t)
	want := &MenuItem{ID: "item-1", SKU: "SOUP", Name: "Soup", SellingPrice: 4.5, IsActive: true}
	r.listedItems = []*MenuItem{want}
	r.items["item-1"] = want
	got, err := s.ListMenuItems(context.Background())
	if err != nil || len(got) != 1 || got[0].SKU != "SOUP" {
		t.Fatalf("list=(%#v,%v)", got, err)
	}
	found, err := s.GetMenuItem(context.Background(), "item-1")
	if err != nil || found == nil || found.Name != "Soup" {
		t.Fatalf("get=(%#v,%v)", found, err)
	}
	if _, err = s.GetMenuItem(context.Background(), "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("missing item error=%v", err)
	}
	for method, call := range map[string]func() error{
		"list": func() error {
			r.errs["ListMenuItems"] = errors.New("list failed")
			_, e := s.ListMenuItems(context.Background())
			return e
		},
		"get": func() error {
			r.errs["GetMenuItemByID"] = errors.New("get failed")
			_, e := s.GetMenuItem(context.Background(), "item-1")
			return e
		},
	} {
		t.Run(method+" repository error", func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("expected repository error")
			}
		})
	}
}

func TestCreateMenuItemAuthorizationValidationAndFields(t *testing.T) {
	s, r, tx := menuHarness(t)
	if _, err := s.CreateMenuItem(context.Background(), CreateMenuItemInput{ActorRole: "BRANCH_MANAGER", SKU: "S", Name: "Soup"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v", err)
	}
	for _, in := range []CreateMenuItemInput{
		{ActorRole: "CEO", Name: "Soup", SellingPrice: 1},
		{ActorRole: "CEO", SKU: "S", SellingPrice: 1},
		{ActorRole: "CEO", SKU: "S", Name: "Soup", SellingPrice: -0.01},
	} {
		if _, err := s.CreateMenuItem(context.Background(), in); err == nil {
			t.Fatalf("expected invalid input error for %#v", in)
		}
	}
	got, err := s.CreateMenuItem(context.Background(), CreateMenuItemInput{ActorRole: "CEO", SKU: "SOUP", Name: "Soup", NameEn: "Shorba", CategoryID: "cat-1", SellingPrice: 0, KDSStation: "hot", PrepTimeMinutes: 7, Description: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.SKU != "SOUP" || got.Name != "Soup" || got.SellingPrice != 0 || !got.IsActive || got.CategoryID == nil || *got.CategoryID != "cat-1" || got.NameEn == nil || *got.NameEn != "Shorba" || got.KDSStation == nil || *got.KDSStation != "hot" || got.PrepTimeMinutes == nil || *got.PrepTimeMinutes != 7 || got.Description == nil || *got.Description != "daily" {
		t.Fatalf("unexpected item: %#v", got)
	}
	if r.createdItem != got || tx.commits != 1 {
		t.Fatalf("not persisted: repo=%#v tx=%#v", r.createdItem, tx)
	}
}

func TestCreateMenuItemTransactionAndRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		beginErr, repoErr, commitErr error
	}{
		{"begin", errors.New("begin"), nil, nil}, {"repository", nil, errors.New("insert"), nil}, {"commit", nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			r.errs["CreateMenuItem"] = tc.repoErr
			got, err := s.CreateMenuItem(context.Background(), CreateMenuItemInput{ActorRole: "CEO", SKU: "S", Name: "Soup"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestUpdateMenuItemAuthorizationMissingAndFieldUpdates(t *testing.T) {
	s, r, tx := menuHarness(t)
	input := UpdateMenuItemInput{ActorRole: "CEO", ID: "item", Name: "New", NameEn: "New EN", CategoryID: "", SellingPrice: 8.5, KDSStation: "cold", PrepTimeMinutes: 4, Description: "updated", IsActive: false}
	if _, err := s.UpdateMenuItem(context.Background(), UpdateMenuItemInput{ActorRole: "BRANCH_MANAGER", ID: "item"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v", err)
	}
	if _, err := s.UpdateMenuItem(context.Background(), input); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("missing error=%v", err)
	}
	r.items["item"] = &MenuItem{ID: "item", Name: "Old", CategoryID: stringPtr("cat-old"), IsActive: true}
	got, err := s.UpdateMenuItem(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "New" || got.NameEn == nil || *got.NameEn != "New EN" || got.CategoryID != nil || got.SellingPrice != 8.5 || got.KDSStation == nil || *got.KDSStation != "cold" || got.PrepTimeMinutes == nil || *got.PrepTimeMinutes != 4 || got.Description == nil || *got.Description != "updated" || got.IsActive || r.updatedItem != got || tx.commits != 1 {
		t.Fatalf("unexpected updated item=%#v repo=%#v tx=%#v", got, r.updatedItem, tx)
	}
	input.CategoryID = "cat-new"
	r.items["item"] = &MenuItem{ID: "item"}
	got, err = s.UpdateMenuItem(context.Background(), input)
	if err != nil || got.CategoryID == nil || *got.CategoryID != "cat-new" {
		t.Fatalf("category update=(%#v,%v)", got, err)
	}
}

func TestUpdateMenuItemRepositoryAndTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		lookupErr, updateErr, beginErr, commitErr error
	}{
		{"lookup", errors.New("lookup"), nil, nil, nil}, {"update", nil, errors.New("update"), nil, nil}, {"begin", nil, nil, errors.New("begin"), nil}, {"commit", nil, nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			r.items["i"] = &MenuItem{ID: "i"}
			r.errs["GetMenuItemByID"] = tc.lookupErr
			r.errs["UpdateMenuItem"] = tc.updateErr
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			got, err := s.UpdateMenuItem(context.Background(), UpdateMenuItemInput{ActorRole: "CEO", ID: "i", Name: "x"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestCategoryListAndCreation(t *testing.T) {
	s, r, tx := menuHarness(t)
	r.listedCategories = []*Category{{ID: "c", Name: "Drinks", SortOrder: 2, IsActive: true}}
	got, err := s.ListCategories(context.Background())
	if err != nil || len(got) != 1 || got[0].Name != "Drinks" {
		t.Fatalf("list=(%#v,%v)", got, err)
	}
	r.errs["ListCategories"] = errors.New("list categories")
	if _, err = s.ListCategories(context.Background()); err == nil {
		t.Fatal("expected list error")
	}
	r.errs["ListCategories"] = nil
	cat, err := s.CreateCategory(context.Background(), CreateCategoryInput{Name: "Dessert", SortOrder: -1})
	if err != nil {
		t.Fatal(err)
	}
	if cat.ID == "" || cat.Name != "Dessert" || cat.SortOrder != -1 || !cat.IsActive || r.createdCategory != cat || tx.commits != 1 {
		t.Fatalf("unexpected category=%#v repo=%#v tx=%#v", cat, r.createdCategory, tx)
	}
}

func TestCreateCategoryTransactionAndRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		beginErr, repoErr, commitErr error
	}{
		{"begin", errors.New("begin"), nil, nil}, {"repository", nil, errors.New("insert"), nil}, {"commit", nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			r.errs["CreateCategory"] = tc.repoErr
			got, err := s.CreateCategory(context.Background(), CreateCategoryInput{Name: "Drinks"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestCreateRequestValidationAndPersistence(t *testing.T) {
	s, r, tx := menuHarness(t)
	for _, in := range []CreateRequestInput{{ItemName: "", RequestedPrice: 1}, {ItemName: "Soup", RequestedPrice: 0}, {ItemName: "Soup", RequestedPrice: -1}} {
		if _, err := s.CreateRequest(context.Background(), in); err == nil {
			t.Fatalf("expected invalid input for %#v", in)
		}
	}
	got, err := s.CreateRequest(context.Background(), CreateRequestInput{BranchID: "branch", RequestedBy: "user", ItemName: "Soup", RequestedPrice: 4.25, Description: "hot", Justification: "requested"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Status != "PENDING" || got.ItemName != "Soup" || got.RequestedPrice != 4.25 || got.Description == nil || *got.Description != "hot" || got.Justification == nil || *got.Justification != "requested" || r.createdRequest != got || tx.commits != 1 {
		t.Fatalf("unexpected request=%#v repo=%#v tx=%#v", got, r.createdRequest, tx)
	}
}

func TestCreateRequestTransactionAndRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		beginErr, repoErr, commitErr error
	}{
		{"begin", errors.New("begin"), nil, nil}, {"repository", nil, errors.New("insert"), nil}, {"commit", nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			r.errs["CreateMenuItemRequest"] = tc.repoErr
			got, err := s.CreateRequest(context.Background(), CreateRequestInput{ItemName: "Soup", RequestedPrice: 1})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestApproveRequestAuthorizationMissingStateAndSuccess(t *testing.T) {
	s, r, tx := menuHarness(t)
	input := ApproveRequestInput{RequestID: "r", ActorRole: "CEO", SKU: "SOUP"}
	if _, err := s.ApproveRequest(context.Background(), ApproveRequestInput{RequestID: "r", ActorRole: "BRANCH_MANAGER"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v", err)
	}
	if _, err := s.ApproveRequest(context.Background(), input); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("missing request error=%v", err)
	}
	r.requests["r"] = &MenuItemRequest{ID: "r", ItemName: "Soup", RequestedPrice: 3, Status: "REJECTED"}
	if _, err := s.ApproveRequest(context.Background(), input); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("state error=%v", err)
	}
	r.requests["r"] = &MenuItemRequest{ID: "r", ItemName: "Soup", RequestedPrice: 3.5, Description: stringPtr("hot"), Status: "PENDING"}
	item, err := s.ApproveRequest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if item.ID == "" || item.SKU != "SOUP" || item.Name != "Soup" || item.SellingPrice != 3.5 || !item.IsActive || item.Description == nil || *item.Description != "hot" || r.updatedRequest == nil || r.updatedRequest.Status != "APPROVED" || tx.commits != 1 {
		t.Fatalf("approval item=%#v request=%#v tx=%#v", item, r.updatedRequest, tx)
	}
}

func TestApproveRequestRepositoryAndTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                                                 string
		lookupErr, updateErr, createErr, beginErr, commitErr error
	}{
		{"lookup", errors.New("lookup"), nil, nil, nil, nil}, {"request update", nil, errors.New("update"), nil, nil, nil}, {"item create", nil, nil, errors.New("create"), nil, nil}, {"begin", nil, nil, nil, errors.New("begin"), nil}, {"commit", nil, nil, nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			r.requests["r"] = &MenuItemRequest{ID: "r", Status: "PENDING"}
			r.errs["GetMenuItemRequestByID"] = tc.lookupErr
			r.errs["UpdateMenuItemRequest"] = tc.updateErr
			r.errs["CreateMenuItem"] = tc.createErr
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			got, err := s.ApproveRequest(context.Background(), ApproveRequestInput{RequestID: "r", ActorRole: "CEO", SKU: "S"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestRejectRequestAuthorizationValidationMissingStateAndSuccess(t *testing.T) {
	s, r, tx := menuHarness(t)
	input := RejectRequestInput{RequestID: "r", ActorRole: "CEO", Reason: "not approved"}
	if _, err := s.RejectRequest(context.Background(), RejectRequestInput{RequestID: "r", ActorRole: "BRANCH_MANAGER", Reason: "x"}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("unauthorized error=%v", err)
	}
	if _, err := s.RejectRequest(context.Background(), RejectRequestInput{RequestID: "r", ActorRole: "CEO"}); err == nil || err.Error() != "rejection reason is required" {
		t.Fatalf("empty reason error=%v", err)
	}
	if _, err := s.RejectRequest(context.Background(), input); !errors.Is(err, ErrRequestNotFound) {
		t.Fatalf("missing request error=%v", err)
	}
	r.requests["r"] = &MenuItemRequest{ID: "r", Status: "APPROVED"}
	if _, err := s.RejectRequest(context.Background(), input); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("state error=%v", err)
	}
	r.requests["r"] = &MenuItemRequest{ID: "r", ItemName: "Soup", Status: "PENDING"}
	got, err := s.RejectRequest(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "REJECTED" || got.RejectionReason == nil || *got.RejectionReason != "not approved" || r.updatedRequest.Status != "REJECTED" || tx.commits != 1 {
		t.Fatalf("unexpected rejection=%#v repo=%#v tx=%#v", got, r.updatedRequest, tx)
	}
}

func TestRejectRequestRepositoryAndTransactionErrors(t *testing.T) {
	for _, tc := range []struct {
		name                                      string
		lookupErr, updateErr, beginErr, commitErr error
	}{
		{"lookup", errors.New("lookup"), nil, nil, nil}, {"update", nil, errors.New("update"), nil, nil}, {"begin", nil, nil, errors.New("begin"), nil}, {"commit", nil, nil, nil, errors.New("commit")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, tx := menuHarness(t)
			r.requests["r"] = &MenuItemRequest{ID: "r", Status: "PENDING"}
			r.errs["GetMenuItemRequestByID"] = tc.lookupErr
			r.errs["UpdateMenuItemRequest"] = tc.updateErr
			tx.beginErr = tc.beginErr
			tx.commitErr = tc.commitErr
			got, err := s.RejectRequest(context.Background(), RejectRequestInput{RequestID: "r", ActorRole: "CEO", Reason: "reason"})
			if err == nil || got != nil {
				t.Fatalf("result=(%#v,%v), want error", got, err)
			}
		})
	}
}

func TestListRequestsAndRepositoryError(t *testing.T) {
	s, r, _ := menuHarness(t)
	r.listedRequests = []*MenuItemRequest{{ID: "r", Status: "PENDING", ItemName: "Soup"}}
	got, err := s.ListRequests(context.Background())
	if err != nil || len(got) != 1 || got[0].ItemName != "Soup" {
		t.Fatalf("list=(%#v,%v)", got, err)
	}
	r.errs["ListMenuItemRequests"] = errors.New("list requests")
	if _, err = s.ListRequests(context.Background()); err == nil {
		t.Fatal("expected list error")
	}
}

var _ driver.Connector = menuConnector{}
var _ driver.ConnBeginTx = menuConn{}
var _ = fmt.Sprintf
