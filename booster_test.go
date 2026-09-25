package booster

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBoosterController(t *testing.T) {
	ctrl := &Controller{
		Title: "Products",
		Table: "products",
		Columns: []Column{
			{Label: "Name", Name: "name", Type: TypeText, Searchable: true},
			{Label: "Price", Name: "price", Type: TypeMoney},
		},
		Forms: []Field{
			{Label: "Product Name", Name: "name", Type: InputText, Required: true},
			{Label: "Price", Name: "price", Type: InputMoney, Required: true},
		},
	}

	// 1. Test Index HTML Page
	req := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	rec := httptest.NewRecorder()
	ctrl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Products") {
		t.Fatalf("expected body to contain 'Products'")
	}

	// 2. Test Data JSON Endpoint
	reqData := httptest.NewRequest(http.MethodGet, "/admin/products/data", nil)
	reqData.Header.Set("Accept", "application/json")
	recData := httptest.NewRecorder()
	ctrl.ServeHTTP(recData, reqData)

	if recData.Code != http.StatusOK {
		t.Fatalf("expected status 200 on data, got %d", recData.Code)
	}
	if !strings.Contains(recData.Body.String(), "Standard Laptop Pro 15") {
		t.Fatalf("expected json data to contain products")
	}

	// 3. Test Add Form Page
	reqAdd := httptest.NewRequest(http.MethodGet, "/admin/products/add", nil)
	recAdd := httptest.NewRecorder()
	ctrl.ServeHTTP(recAdd, reqAdd)

	if recAdd.Code != http.StatusOK {
		t.Fatalf("expected status 200 on add form, got %d", recAdd.Code)
	}
	if !strings.Contains(recAdd.Body.String(), "Create New Products") {
		t.Fatalf("expected form title in html")
	}
}
