package menu

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func menuRouter(t *testing.T, role string) (*gin.Engine, *fakeMenuRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := newFakeMenuRepo()
	svc := NewService(NewHandlerDB(t), repo)
	router := gin.New()
	if role != "" {
		router.Use(func(c *gin.Context) {
			c.Set(CtxUserRole, role)
			c.Set(CtxUserID, "00000000-0000-4000-8000-000000000099")
			c.Next()
		})
	}
	NewHandler(svc).RegisterRoutes(router.Group("/api/v1"))
	return router, repo
}
func NewHandlerDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db := sqlx.NewDb(sql.OpenDB(menuConnector{&menuTxState{}}), "postgres")
	t.Cleanup(func() { _ = db.Close() })
	return db
}
func menuJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	return res
}
func menuErrorCode(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var body ErrorResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error: %v (%s)", err, res.Body.String())
	}
	return body.Error.Code
}

func TestMenuItemListAndGetHandlers(t *testing.T) {
	router, repo := menuRouter(t, "")
	repo.listedItems = []*MenuItem{{ID: "i", SKU: "SOUP", Name: "Soup", SellingPrice: 2.5, IsActive: true}}
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/menu/items", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var items []MenuItemResponse
	if err := json.Unmarshal(list.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SKU != "SOUP" || items[0].SellingPrice != 2.5 {
		t.Fatalf("list response=%#v", items)
	}
	repo.items["i"] = repo.listedItems[0]
	got := httptest.NewRecorder()
	router.ServeHTTP(got, httptest.NewRequest(http.MethodGet, "/api/v1/menu/items/i", nil))
	if got.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
	var item MenuItemResponse
	if err := json.Unmarshal(got.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID != "i" || item.Name != "Soup" {
		t.Fatalf("get response=%#v", item)
	}
	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/v1/menu/items/missing", nil))
	if missing.Code != http.StatusNotFound || menuErrorCode(t, missing) != "not_found" {
		t.Fatalf("missing response=(%d,%s)", missing.Code, missing.Body.String())
	}
	repo.errs["ListMenuItems"] = errors.New("list failed")
	failed := httptest.NewRecorder()
	router.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/api/v1/menu/items", nil))
	if failed.Code != http.StatusInternalServerError || menuErrorCode(t, failed) != "internal_error" {
		t.Fatalf("list error=(%d,%s)", failed.Code, failed.Body.String())
	}
}

func TestCreateAndUpdateMenuItemHandlers(t *testing.T) {
	router, repo := menuRouter(t, "")
	createBody := map[string]any{"actor_role": "CEO", "sku": "SOUP", "name": "Soup", "name_en": "Shorba", "category_id": "category", "selling_price": 4.5, "kds_station": "hot", "prep_time_minutes": 8, "description": "fresh"}
	created := menuJSON(t, router, http.MethodPost, "/api/v1/menu/items", createBody)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var item MenuItemResponse
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.ID == "" || item.SKU != "SOUP" || item.Name != "Soup" || item.SellingPrice != 4.5 || item.CategoryID == nil || *item.CategoryID != "category" {
		t.Fatalf("create response=%#v", item)
	}
	if repo.createdItem == nil || repo.createdItem.IsActive != true {
		t.Fatalf("created item not captured: %#v", repo.createdItem)
	}
	for _, body := range []map[string]any{
		{"actor_role": "CEO", "name": "Soup"},
		{"actor_role": "CEO", "sku": "S", "name": "Soup", "selling_price": -1},
		{"actor_role": "CEO", "sku": "S", "name": "Soup", "selling_price": 0},
	} {
		bad := menuJSON(t, router, http.MethodPost, "/api/v1/menu/items", body)
		if bad.Code != http.StatusBadRequest || menuErrorCode(t, bad) != "invalid_payload" {
			t.Fatalf("invalid create response=(%d,%s)", bad.Code, bad.Body.String())
		}
	}
	unauthorized := menuJSON(t, router, http.MethodPost, "/api/v1/menu/items", map[string]any{"actor_role": "BRANCH_MANAGER", "sku": "S", "name": "Soup", "selling_price": 1})
	if unauthorized.Code != http.StatusForbidden || menuErrorCode(t, unauthorized) != "forbidden" {
		t.Fatalf("unauthorized create=(%d,%s)", unauthorized.Code, unauthorized.Body.String())
	}

	repo.items["i"] = &MenuItem{ID: "i", Name: "Old", IsActive: true}
	updateBody := map[string]any{"actor_role": "CEO", "name": "Updated", "selling_price": 7, "category_id": "", "is_active": false}
	updated := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/items/i", updateBody)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	var updatedItem MenuItemResponse
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedItem); err != nil {
		t.Fatal(err)
	}
	if updatedItem.Name != "Updated" || updatedItem.SellingPrice != 7 || updatedItem.CategoryID != nil || updatedItem.IsActive {
		t.Fatalf("update response=%#v", updatedItem)
	}
	if repo.updatedItem == nil {
		t.Fatal("repository did not receive updated item")
	}
	if repo.updatedItem.CategoryID != nil {
		t.Fatalf("repository update CategoryID = %q, want nil after empty category_id", *repo.updatedItem.CategoryID)
	}

	categoryUpdateBody := map[string]any{"actor_role": "CEO", "name": "Updated", "selling_price": 7, "category_id": "cat-new", "is_active": true}
	categoryUpdated := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/items/i", categoryUpdateBody)
	if categoryUpdated.Code != http.StatusOK {
		t.Fatalf("category update status=%d body=%s", categoryUpdated.Code, categoryUpdated.Body.String())
	}
	if repo.updatedItem == nil {
		t.Fatal("repository did not receive category update")
	}
	if repo.updatedItem.CategoryID == nil {
		t.Fatal("repository update CategoryID is nil, want cat-new")
	}
	if *repo.updatedItem.CategoryID != "cat-new" {
		t.Fatalf("repository update CategoryID = %q, want cat-new", *repo.updatedItem.CategoryID)
	}
	missing := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/items/missing", updateBody)
	if missing.Code != http.StatusNotFound || menuErrorCode(t, missing) != "not_found" {
		t.Fatalf("missing update=(%d,%s)", missing.Code, missing.Body.String())
	}
	invalid := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/items/i", map[string]any{"actor_role": "CEO", "name": "", "selling_price": -1})
	if invalid.Code != http.StatusBadRequest || menuErrorCode(t, invalid) != "invalid_payload" {
		t.Fatalf("invalid update=(%d,%s)", invalid.Code, invalid.Body.String())
	}
	denied := menuJSON(t, menuRouterWithRole(t, "BRANCH_MANAGER", repo), http.MethodPatch, "/api/v1/menu/items/i", updateBody)
	if denied.Code != http.StatusForbidden || menuErrorCode(t, denied) != "forbidden" {
		t.Fatalf("unauthorized update=(%d,%s)", denied.Code, denied.Body.String())
	}
}

func menuRouterWithRole(t *testing.T, role string, repo *fakeMenuRepo) http.Handler {
	t.Helper()
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set(CtxUserRole, role); c.Next() })
	NewHandler(NewService(NewHandlerDB(t), repo)).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func TestCategoryHandlersListCreateValidationAndErrors(t *testing.T) {
	router, repo := menuRouter(t, "")
	repo.listedCategories = []*Category{{ID: "c", Name: "Drinks", SortOrder: 1, IsActive: true}}
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/menu/categories", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var categories []CategoryResponse
	if err := json.Unmarshal(list.Body.Bytes(), &categories); err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0].Name != "Drinks" {
		t.Fatalf("categories=%#v", categories)
	}
	created := menuJSON(t, router, http.MethodPost, "/api/v1/menu/categories", map[string]any{"name": "Dessert", "sort_order": -2})
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var cat CategoryResponse
	if err := json.Unmarshal(created.Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	if cat.Name != "Dessert" || cat.SortOrder != -2 || !cat.IsActive {
		t.Fatalf("category=%#v", cat)
	}
	bad := menuJSON(t, router, http.MethodPost, "/api/v1/menu/categories", map[string]any{"name": ""})
	if bad.Code != http.StatusBadRequest || menuErrorCode(t, bad) != "invalid_payload" {
		t.Fatalf("category validation=(%d,%s)", bad.Code, bad.Body.String())
	}
	repo.errs["ListCategories"] = errors.New("list categories")
	failed := httptest.NewRecorder()
	router.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/api/v1/menu/categories", nil))
	if failed.Code != http.StatusInternalServerError || menuErrorCode(t, failed) != "internal_error" {
		t.Fatalf("category list error=(%d,%s)", failed.Code, failed.Body.String())
	}
}

func TestMenuRequestHandlersCreateListApproveAndReject(t *testing.T) {
	router, repo := menuRouter(t, "")
	create := menuJSON(t, router, http.MethodPost, "/api/v1/menu/requests", map[string]any{"branch_id": "00000000-0000-4000-8000-000000000001", "item_name": "Soup", "requested_price": 3.25, "description": "hot", "justification": "customer demand"})
	if create.Code != http.StatusCreated {
		t.Fatalf("request create=(%d,%s)", create.Code, create.Body.String())
	}
	var req MenuRequestResponse
	if err := json.Unmarshal(create.Body.Bytes(), &req); err != nil {
		t.Fatal(err)
	}
	if req.ID == "" || req.Status != "PENDING" || req.ItemName != "Soup" || req.RequestedBy != "99999999-9999-9999-9999-999999999999" {
		t.Fatalf("created request=%#v", req)
	}
	if repo.createdRequest == nil {
		t.Fatal("request not persisted")
	}
	for _, body := range []map[string]any{
		{"branch_id": "bad", "item_name": "Soup", "requested_price": 1},
		{"branch_id": "00000000-0000-4000-8000-000000000001", "item_name": "", "requested_price": 1},
		{"branch_id": "00000000-0000-4000-8000-000000000001", "item_name": "Soup", "requested_price": 0},
		{"branch_id": "00000000-0000-4000-8000-000000000001", "item_name": "Soup", "requested_price": -1},
	} {
		bad := menuJSON(t, router, http.MethodPost, "/api/v1/menu/requests", body)
		if bad.Code != http.StatusBadRequest || menuErrorCode(t, bad) != "invalid_payload" {
			t.Fatalf("invalid request=(%d,%s)", bad.Code, bad.Body.String())
		}
	}
	repo.listedRequests = []*MenuItemRequest{{ID: "r", ItemName: "Tea", Status: "PENDING"}}
	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/v1/menu/requests", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	pendingID := "00000000-0000-4000-8000-000000000010"
	repo.requests[pendingID] = &MenuItemRequest{ID: pendingID, ItemName: "Pie", RequestedPrice: 5, Status: "PENDING"}
	approved := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/requests/"+pendingID+"/approve", map[string]any{"actor_role": "CEO", "sku": "PIE"})
	if approved.Code != http.StatusCreated {
		t.Fatalf("approve=(%d,%s)", approved.Code, approved.Body.String())
	}
	var approvedItem MenuItemResponse
	if err := json.Unmarshal(approved.Body.Bytes(), &approvedItem); err != nil {
		t.Fatal(err)
	}
	if approvedItem.SKU != "PIE" || approvedItem.Name != "Pie" || approvedItem.SellingPrice != 5 {
		t.Fatalf("approved item=%#v", approvedItem)
	}
	denied := menuJSON(t, menuRouterWithRole(t, "BRANCH_MANAGER", repo), http.MethodPatch, "/api/v1/menu/requests/"+pendingID+"/approve", map[string]any{"actor_role": "CEO", "sku": "X"})
	if denied.Code != http.StatusForbidden || menuErrorCode(t, denied) != "forbidden" {
		t.Fatalf("approve authorization=(%d,%s)", denied.Code, denied.Body.String())
	}
	missing := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/requests/missing/approve", map[string]any{"actor_role": "CEO", "sku": "X"})
	if missing.Code != http.StatusNotFound || menuErrorCode(t, missing) != "not_found" {
		t.Fatalf("missing approve=(%d,%s)", missing.Code, missing.Body.String())
	}
	invalidStateID := "00000000-0000-4000-8000-000000000011"
	repo.requests[invalidStateID] = &MenuItemRequest{ID: invalidStateID, Status: "APPROVED"}
	conflict := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/requests/"+invalidStateID+"/approve", map[string]any{"actor_role": "CEO", "sku": "X"})
	if conflict.Code != http.StatusConflict || menuErrorCode(t, conflict) != "invalid_status" {
		t.Fatalf("invalid state=(%d,%s)", conflict.Code, conflict.Body.String())
	}

	rejectID := "00000000-0000-4000-8000-000000000012"
	repo.requests[rejectID] = &MenuItemRequest{ID: rejectID, ItemName: "Cake", Status: "PENDING"}
	rejected := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/requests/"+rejectID+"/reject", map[string]any{"actor_role": "CEO", "reason": "not needed"})
	if rejected.Code != http.StatusOK {
		t.Fatalf("reject=(%d,%s)", rejected.Code, rejected.Body.String())
	}
	var rejectedReq MenuRequestResponse
	if err := json.Unmarshal(rejected.Body.Bytes(), &rejectedReq); err != nil {
		t.Fatal(err)
	}
	if rejectedReq.Status != "REJECTED" || rejectedReq.RejectionReason == nil || *rejectedReq.RejectionReason != "not needed" {
		t.Fatalf("rejected request=%#v", rejectedReq)
	}
	emptyReason := menuJSON(t, router, http.MethodPatch, "/api/v1/menu/requests/"+rejectID+"/reject", map[string]any{"actor_role": "CEO", "reason": ""})
	if emptyReason.Code != http.StatusBadRequest || menuErrorCode(t, emptyReason) != "invalid_payload" {
		t.Fatalf("empty reason=(%d,%s)", emptyReason.Code, emptyReason.Body.String())
	}
}
