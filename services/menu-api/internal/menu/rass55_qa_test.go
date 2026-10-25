package menu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type rass55MenuRepositoryStub struct {
	Repository
	item *MenuItem
}

func (r *rass55MenuRepositoryStub) GetMenuItemByID(_ context.Context, id string) (*MenuItem, error) {
	if r.item != nil && r.item.ID == id {
		return r.item, nil
	}
	return nil, nil
}

func newRASS55TestRouter(repo Repository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(NewService(nil, repo)).RegisterRoutes(router.Group("/api/v1"))
	return router
}

func assertRASS55Error(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, status, response.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(strings.NewReader(response.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != code {
		t.Errorf("error code = %q, want %q", body.Error.Code, code)
	}
}

func TestRASS55DirectMenuItemCreationRejectsInvalidPayloads(t *testing.T) {
	router := newRASS55TestRouter(&rass55MenuRepositoryStub{})
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"sku":`},
		{name: "missing SKU", body: `{"name":"Item","selling_price":5}`},
		{name: "missing name", body: `{"sku":"SKU-1","selling_price":5}`},
		{name: "negative selling price", body: `{"sku":"SKU-1","name":"Item","selling_price":-1}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/menu/items", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			assertRASS55Error(t, response, http.StatusBadRequest, "invalid_payload")
		})
	}
}

func TestRASS55CreateMenuRequestRejectsInvalidPayloads(t *testing.T) {
	router := newRASS55TestRouter(&rass55MenuRepositoryStub{})
	cases := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"branch_id":`},
		{name: "invalid branch UUID", body: `{"branch_id":"not-a-uuid","item_name":"Item","requested_price":5}`},
		{name: "zero requested price", body: `{"branch_id":"11111111-1111-4111-8111-111111111111","item_name":"Item","requested_price":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/menu/requests", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			assertRASS55Error(t, response, http.StatusBadRequest, "invalid_payload")
		})
	}
}

func TestRASS55CEOOnlyHandlersRejectExplicitNonCEOActorRole(t *testing.T) {
	router := newRASS55TestRouter(&rass55MenuRepositoryStub{})
	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "direct item creation",
			method: http.MethodPost,
			path:   "/api/v1/menu/items",
			body:   `{"sku":"SKU-1","name":"Item","selling_price":5,"actor_role":"Branch Manager"}`,
		},
		{
			name:   "request approval",
			method: http.MethodPatch,
			path:   "/api/v1/menu/requests/request-1/approve",
			body:   `{"sku":"SKU-1","actor_role":"Branch Manager"}`,
		},
		{
			name:   "request rejection",
			method: http.MethodPatch,
			path:   "/api/v1/menu/requests/request-1/reject",
			body:   `{"reason":"No","actor_role":"Branch Manager"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			assertRASS55Error(t, response, http.StatusForbidden, "forbidden")
		})
	}
}

func TestRASS55GetMissingMenuItemMapsToNotFound(t *testing.T) {
	router := newRASS55TestRouter(&rass55MenuRepositoryStub{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/menu/items/missing-item", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	assertRASS55Error(t, response, http.StatusNotFound, "not_found")
}

func TestRASS55RoleFallbackDefaultsToCEOWithoutAuthenticatedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	if got := getRole(ctx, ""); got != "CEO" {
		t.Fatalf("role with no authenticated context or request role = %q, want CEO fallback", got)
	}
}
