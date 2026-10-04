package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/controllers"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

func TestStarterMVCControllers(t *testing.T) {
	admin := booster.NewEngine("Starter Test Admin")

	productCtrl := controllers.NewAdminProductController()
	customerCtrl := controllers.NewAdminCustomerController()
	orderCtrl := controllers.NewAdminOrderController()

	admin.Register(productCtrl)
	admin.Register(customerCtrl)
	admin.Register(orderCtrl)

	// Auth session
	recAuth := httptest.NewRecorder()
	admin.Auth.SetSessionUser(recAuth, &booster.User{Name: "Admin Test", RoleName: "Superadmin"})
	cookie := recAuth.Result().Cookies()[0]

	// 1. Verify Listing uses Model data
	req := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	productCtrl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Apple MacBook Pro") {
		t.Fatalf("expected listing to contain Apple MacBook Pro from Product model")
	}

	// 2. Add New Product via POST
	form := url.Values{}
	form.Set("name", "iPad Pro M4 13-inch")
	form.Set("price", "21000000")
	form.Set("stock", "8")
	form.Set("description", "OLED iPad Pro")

	postReq := httptest.NewRequest(http.MethodPost, "/admin/products/add", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(cookie)
	postRec := httptest.NewRecorder()
	productCtrl.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303 after add, got %d", postRec.Code)
	}

	// Verify newly added item exists in ProductRepo
	rows, err := models.ProductRepo.FindAll(nil)
	if err != nil {
		t.Fatalf("failed to query ProductRepo: %v", err)
	}
	found := false
	for _, r := range rows {
		if r["name"] == "iPad Pro M4 13-inch" {
			found = true
			if r["status"] != "Active" {
				t.Fatalf("expected HookBeforeAdd to set status 'Active', got '%v'", r["status"])
			}
			if r["price"] != "Rp 21000000" {
				t.Fatalf("expected HookBeforeAdd to prefix 'Rp ', got '%v'", r["price"])
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected to find newly added product in ProductRepo")
	}
}

func TestMySQLDatabaseIntegration(t *testing.T) {
	db, err := models.InitDatabase()
	if err != nil || db == nil {
		t.Skip("MySQL not accessible in current environment, skipping live MySQL test")
		return
	}

	// 1. Verify tables exist and have seeded data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query products table in MySQL: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected products table to contain seeded records")
	}

	// 2. Verify ProductRepo uses live SQLStore
	rows, err := models.ProductRepo.FindAll(nil)
	if err != nil {
		t.Fatalf("ProductRepo.FindAll failed on MySQL: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected ProductRepo to return MySQL rows")
	}

	// 3. Test Insert via SQLStore DataProvider
	testName := "MySQL Integration Test Product"
	err = models.ProductRepo.Create(nil, map[string]interface{}{
		"name":        testName,
		"price":       "Rp 99.000",
		"stock":       5,
		"status":      "Active",
		"description": "Created during automated test",
	})
	if err != nil {
		t.Fatalf("failed to create product in MySQL: %v", err)
	}

	// 4. Verify directly in MySQL table
	var insertedID string
	err = db.QueryRow("SELECT id FROM products WHERE name = ?", testName).Scan(&insertedID)
	if err != nil {
		t.Fatalf("failed to find inserted row in MySQL: %v", err)
	}

	// 5. Clean up test record
	err = models.ProductRepo.Delete(nil, insertedID)
	if err != nil {
		t.Fatalf("failed to delete test product: %v", err)
	}
}

func TestSQLiteDatabaseIntegration(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_DATABASE", "tmp/test_sqlite.db")

	db, err := models.InitDatabase()
	if err != nil || db == nil {
		t.Fatalf("failed to init SQLite: %v", err)
	}
	defer db.Close()

	// 1. Verify tables exist
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query SQLite products table: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected SQLite products table to have seeded records")
	}

	// 2. Test ProductRepo with SQLite
	testName := "SQLite Test Product"
	err = models.ProductRepo.Create(nil, map[string]interface{}{
		"name":   testName,
		"price":  "Rp 50.000",
		"stock":  10,
		"status": "Active",
	})
	if err != nil {
		t.Fatalf("ProductRepo.Create failed on SQLite: %v", err)
	}

	// 3. Verify in SQLite table
	var insertedID string
	err = db.QueryRow("SELECT id FROM products WHERE name = ?", testName).Scan(&insertedID)
	if err != nil {
		t.Fatalf("failed to query inserted record in SQLite: %v", err)
	}

	// 4. Test Update
	err = models.ProductRepo.Update(nil, insertedID, map[string]interface{}{
		"price": "Rp 75.000",
	})
	if err != nil {
		t.Fatalf("ProductRepo.Update failed on SQLite: %v", err)
	}

	// 5. Test Delete
	err = models.ProductRepo.Delete(nil, insertedID)
	if err != nil {
		t.Fatalf("ProductRepo.Delete failed on SQLite: %v", err)
	}
}
