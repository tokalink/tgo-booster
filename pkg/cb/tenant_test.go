package cb

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestTenantManager_CRUDAndPersistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tgo_tenant_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	tm := &TenantManager{
		filePath: filepath.Join(tmpDir, "tenants.json"),
		tenants:  make([]*Tenant, 0),
	}
	tm.load()

	// 1. Verify default seed
	all := tm.GetAll()
	if len(all) < 2 {
		t.Fatalf("expected at least 2 default tenants, got %d", len(all))
	}

	def := tm.GetBySlug("default")
	if def == nil || def.Plan != "Enterprise" {
		t.Fatalf("expected default enterprise tenant, got %+v", def)
	}

	// 2. Create custom tenant with domain
	newTenant := &Tenant{
		Name:       "Kriva Digital Agency",
		Slug:       "kriva-digital",
		Domain:     "kriva.agency",
		Subdomain:  "kriva",
		Plan:       "Pro",
		Status:     "Active",
		OwnerEmail: "ceo@kriva.agency",
		MaxUsers:   50,
	}
	if err := tm.Save(newTenant); err != nil {
		t.Fatalf("failed to save tenant: %v", err)
	}

	// 3. Find by domain, subdomain, slug
	byDomain := tm.GetByDomain("kriva.agency")
	if byDomain == nil || byDomain.Slug != "kriva-digital" {
		t.Fatalf("expected to find by domain 'kriva.agency', got %+v", byDomain)
	}

	bySub := tm.GetBySubdomain("kriva")
	if bySub == nil || bySub.Name != "Kriva Digital Agency" {
		t.Fatalf("expected to find by subdomain 'kriva', got %+v", bySub)
	}

	// 4. Persistence reload test
	tm2 := &TenantManager{
		filePath: filepath.Join(tmpDir, "tenants.json"),
		tenants:  make([]*Tenant, 0),
	}
	tm2.load()
	if tFound := tm2.GetBySlug("kriva-digital"); tFound == nil || tFound.OwnerEmail != "ceo@kriva.agency" {
		t.Fatalf("expected persisted tenant after reload, got %+v", tFound)
	}
}

func TestMultiTenancy_MemoryStore_Isolation(t *testing.T) {
	store := NewMemoryStore()
	store.SetTenantScoped(true)

	ctxTenantA := &Context{TenantSlug: "tenant_alpha"}
	ctxTenantB := &Context{TenantSlug: "tenant_beta"}
	ctxSuper := &Context{TenantSlug: "*"}

	// Tenant A creates 2 items
	if err := store.Create(ctxTenantA, map[string]interface{}{"name": "Alpha Item 1", "price": 100}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctxTenantA, map[string]interface{}{"name": "Alpha Item 2", "price": 200}); err != nil {
		t.Fatal(err)
	}

	// Tenant B creates 1 item
	if err := store.Create(ctxTenantB, map[string]interface{}{"name": "Beta Item 1", "price": 999}); err != nil {
		t.Fatal(err)
	}

	// 1. Tenant A FindAll
	rowsA, err := store.FindAll(ctxTenantA)
	if err != nil || len(rowsA) != 2 {
		t.Fatalf("expected 2 items for Tenant A, got %d (err: %v)", len(rowsA), err)
	}

	// 2. Tenant B FindAll
	rowsB, err := store.FindAll(ctxTenantB)
	if err != nil || len(rowsB) != 1 {
		t.Fatalf("expected 1 item for Tenant B, got %d (err: %v)", len(rowsB), err)
	}
	if rowsB[0]["name"] != "Beta Item 1" {
		t.Fatalf("unexpected item for Tenant B: %v", rowsB[0])
	}

	// 3. Superadmin sees all 3 items
	rowsAll, err := store.FindAll(ctxSuper)
	if err != nil || len(rowsAll) != 3 {
		t.Fatalf("expected 3 items for superadmin, got %d", len(rowsAll))
	}

	// 4. Cross-tenant tampering prevention
	// Tenant A tries to access or delete Tenant B's record (id: 3)
	if _, err := store.FindByID(ctxTenantA, "3"); err == nil {
		t.Fatal("Tenant A should NOT be able to find Tenant B's record")
	}
	if err := store.Update(ctxTenantA, "3", map[string]interface{}{"name": "Hacked"}); err == nil {
		t.Fatal("Tenant A should NOT be able to update Tenant B's record")
	}
	if err := store.Delete(ctxTenantA, "3"); err == nil {
		t.Fatal("Tenant A should NOT be able to delete Tenant B's record")
	}

	// Verify Tenant B's record is intact
	recB, err := store.FindByID(ctxTenantB, "3")
	if err != nil || recB["name"] != "Beta Item 1" {
		t.Fatalf("Tenant B's record should remain untouched, got %v (err: %v)", recB, err)
	}
}

func TestMultiTenancy_SQLStore_Isolation(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLStore(db, "invoices", "id")
	store.SetTenantScoped(true)

	// 1. AutoMigrate table with tenant_id
	err = store.AutoMigrate(
		Field{Name: "invoice_no", Type: InputText},
		Field{Name: "amount", Type: InputNumber},
	)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	ctxOrg1 := &Context{TenantSlug: "org_tokalink"}
	ctxOrg2 := &Context{TenantSlug: "org_acme"}
	ctxGlobal := &Context{TenantSlug: "*"}

	// 2. Insert records
	if err := store.Create(ctxOrg1, map[string]interface{}{"invoice_no": "INV-001", "amount": 5000}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctxOrg1, map[string]interface{}{"invoice_no": "INV-002", "amount": 8000}); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctxOrg2, map[string]interface{}{"invoice_no": "INV-999", "amount": 12000}); err != nil {
		t.Fatal(err)
	}

	// 3. Verify Org1 FindAll
	org1Rows, err := store.FindAll(ctxOrg1)
	if err != nil || len(org1Rows) != 2 {
		t.Fatalf("expected 2 invoices for org1, got %d (err: %v)", len(org1Rows), err)
	}

	// 4. Verify Org2 FindAll
	org2Rows, err := store.FindAll(ctxOrg2)
	if err != nil || len(org2Rows) != 1 {
		t.Fatalf("expected 1 invoice for org2, got %d (err: %v)", len(org2Rows), err)
	}
	if org2Rows[0]["invoice_no"] != "INV-999" {
		t.Fatalf("unexpected invoice for org2: %v", org2Rows[0])
	}

	// 5. Global FindAll
	allRows, err := store.FindAll(ctxGlobal)
	if err != nil || len(allRows) != 3 {
		t.Fatalf("expected 3 total invoices for global context, got %d", len(allRows))
	}

	// 6. Cross-tenant tampering test
	// Org1 tries to delete Org2's invoice (ID: 3)
	if err := store.Delete(ctxOrg1, "3"); err != nil {
		t.Fatal(err)
	}
	// Org2's invoice must still exist!
	org2Rec, err := store.FindByID(ctxOrg2, "3")
	if err != nil || org2Rec["invoice_no"] != "INV-999" {
		t.Fatalf("Org 2 invoice should still exist after Org 1 attempted delete: %v (err: %v)", org2Rec, err)
	}
}

func TestEngine_ResolveTenant_Strategies(t *testing.T) {
	engine := NewEngine("Test SaaS Platform")
	engine.EnableMultiTenancy(true)

	// Add test tenant with domain & subdomain
	_ = engine.TenantManager.Save(&Tenant{
		Name:      "Acme Cloud Services",
		Slug:      "acme-cloud",
		Domain:    "saas.acme.com",
		Subdomain: "acme",
		Plan:      "Enterprise",
		Status:    "Active",
	})

	// 1. Query parameter resolution
	reqQuery := httptest.NewRequest("GET", "/admin?tenant=acme-cloud", nil)
	if slug := engine.ResolveTenant(reqQuery, nil); slug != "acme-cloud" {
		t.Fatalf("expected 'acme-cloud' from query param, got %s", slug)
	}

	// 2. Custom Domain resolution
	reqCustomDomain := httptest.NewRequest("GET", "/admin/orders", nil)
	reqCustomDomain.Host = "saas.acme.com:8080"
	if slug := engine.ResolveTenant(reqCustomDomain, nil); slug != "acme-cloud" {
		t.Fatalf("expected 'acme-cloud' from custom domain, got %s", slug)
	}

	// 3. Subdomain resolution
	reqSubdomain := httptest.NewRequest("GET", "/admin/orders", nil)
	reqSubdomain.Host = "acme.tgo.io"
	if slug := engine.ResolveTenant(reqSubdomain, nil); slug != "acme-cloud" {
		t.Fatalf("expected 'acme-cloud' from subdomain, got %s", slug)
	}

	// 4. HTTP Header resolution
	reqHeader := httptest.NewRequest("GET", "/api/v1/data", nil)
	reqHeader.Header.Set("X-Tenant-Slug", "acme-cloud")
	if slug := engine.ResolveTenant(reqHeader, nil); slug != "acme-cloud" {
		t.Fatalf("expected 'acme-cloud' from header, got %s", slug)
	}

	// 5. Session Cookie resolution
	reqCookie := httptest.NewRequest("GET", "/admin", nil)
	reqCookie.AddCookie(&http.Cookie{Name: "cb_active_tenant", Value: "acme-cloud"})
	if slug := engine.ResolveTenant(reqCookie, nil); slug != "acme-cloud" {
		t.Fatalf("expected 'acme-cloud' from cookie, got %s", slug)
	}

	// 6. Default fallback
	reqPlain := httptest.NewRequest("GET", "/admin", nil)
	if slug := engine.ResolveTenant(reqPlain, nil); slug != "default" {
		t.Fatalf("expected 'default' fallback, got %s", slug)
	}
}

func TestTenantSwitch_HTTPHandler(t *testing.T) {
	engine := NewEngine("Test SaaS Switcher")
	engine.EnableMultiTenancy(true)

	// Ensure demo-enterprise exists
	_ = engine.TenantManager.Save(&Tenant{
		Name:   "Demo Enterprise",
		Slug:   "demo-enterprise",
		Plan:   "Pro",
		Status: "Active",
	})

	// Create test server with registered switch handler
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/tenants/switch/", engine.handleTenantSwitch)
	mux.HandleFunc("/admin/tenants/switch", engine.handleTenantSwitch)

	user := &User{
		ID:       "1",
		Name:     "Admin User",
		RoleName: "Superadmin",
		Tenants:  []string{"*"},
	}

	req := httptest.NewRequest("GET", "/admin/tenants/switch/demo-enterprise", nil)
	rec := httptest.NewRecorder()

	// Attach session
	engine.Auth.SetSessionUser(rec, user)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req)

	// Verify redirect
	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d", rec2.Code)
	}

	// Verify cb_active_tenant cookie was issued
	var activeCookie *http.Cookie
	for _, c := range rec2.Result().Cookies() {
		if c.Name == "cb_active_tenant" {
			activeCookie = c
			break
		}
	}

	if activeCookie == nil || activeCookie.Value != "demo-enterprise" {
		t.Fatalf("expected cb_active_tenant=demo-enterprise cookie, got %v", activeCookie)
	}
}

// Regresi keamanan: domain/subdomain tidak boleh menentukan tenant bagi pengguna yang bukan anggotanya
// (Host: b.domain dengan sesi tenant A tidak boleh membuka data tenant B).
func TestEngine_ResolveTenant_HostRequiresMembership(t *testing.T) {
	engine := NewEngine("Test SaaS Platform")
	engine.EnableMultiTenancy(true)
	_ = engine.TenantManager.Save(&Tenant{Name: "A", Slug: "tenant-a", Subdomain: "a", Plan: "Free", Status: "Active"})
	_ = engine.TenantManager.Save(&Tenant{Name: "B", Slug: "tenant-b", Subdomain: "b", Domain: "b.example.com", Plan: "Free", Status: "Active"})
	userA := &User{ID: "1", Email: "a@x.id", Tenants: []string{"tenant-a"}, ActiveTenant: "tenant-a"}

	for _, host := range []string{"b.tgo.io", "b.example.com"} {
		r := httptest.NewRequest("GET", "/admin/orders", nil)
		r.Host = host
		if slug := engine.ResolveTenant(r, userA); slug != "tenant-a" {
			t.Fatalf("host %s: pengguna tenant-a tidak boleh dilayani sebagai %q", host, slug)
		}
	}
	// anggota B tetap dilayani lewat hostnya sendiri
	userB := &User{ID: "2", Email: "b@x.id", Tenants: []string{"tenant-b"}}
	r := httptest.NewRequest("GET", "/admin/orders", nil)
	r.Host = "b.tgo.io"
	if slug := engine.ResolveTenant(r, userB); slug != "tenant-b" {
		t.Fatalf("anggota tenant-b seharusnya dilayani sebagai tenant-b, dapat %q", slug)
	}
}
