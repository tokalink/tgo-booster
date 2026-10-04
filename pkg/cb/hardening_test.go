package cb

import (
	"bytes"
	"context"
	"database/sql"
	"html/template"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestUserManager_AuthenticationAndHashing(t *testing.T) {
	// Clean up temporary test data
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	engine := NewEngine("Test App")
	um := engine.UserManager

	// 1. Verify default seeded accounts exist
	admin := um.FindByEmail("admin@tgo.io")
	if admin == nil {
		t.Fatalf("expected admin@tgo.io to be seeded by default")
	}
	if admin.PasswordHash == "" || admin.Salt == "" {
		t.Fatalf("expected admin to have non-empty salt and password hash")
	}

	// 2. Authenticate with correct credentials
	u, err := um.Authenticate("admin@tgo.io", "admin123", "127.0.0.1", 5, 15)
	if err != nil {
		t.Fatalf("expected successful auth with valid credentials: %v", err)
	}
	if u.Email != "admin@tgo.io" {
		t.Fatalf("expected email admin@tgo.io, got %s", u.Email)
	}

	// 3. Authenticate with incorrect credentials
	_, err = um.Authenticate("admin@tgo.io", "wrongpassword", "127.0.0.1", 5, 15)
	if err == nil {
		t.Fatalf("expected authentication error on invalid password")
	}

	// 4. Rate-limiting / Brute-force lockout test
	st := engine.GetSettings()
	st.MaxLoginAttempts = 3
	st.LockoutDurationMin = 15
	_ = engine.SaveSettings(&st)

	// Trigger 3 failed attempts
	for i := 0; i < 3; i++ {
		_, _ = um.Authenticate("manager@tgo.io", "badpass", "127.0.0.1", 3, 15)
	}

	// 4th attempt should be blocked by rate limiter
	_, err = um.Authenticate("manager@tgo.io", "badpass", "127.0.0.1", 3, 15)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected account lockout message, got: %v", err)
	}

	// 5. Test UserDataProvider CRUD
	udp := um.NewDataProvider()
	ctx := &Context{Ctx: context.Background()}

	// Create new user
	newUserData := map[string]interface{}{
		"name":     "Dev Ops",
		"email":    "devops@tgo.io",
		"role":     "Developer",
		"password": "SecretPassword99!",
		"status":   "Active",
	}
	err = udp.Create(ctx, newUserData)
	if err != nil {
		t.Fatalf("failed to create user via provider: %v", err)
	}

	// Authenticate the newly created user
	devUser, err := um.Authenticate("devops@tgo.io", "SecretPassword99!", "127.0.0.1", 5, 15)
	if err != nil {
		t.Fatalf("failed to authenticate newly created user: %v", err)
	}
	if devUser.Name != "Dev Ops" {
		t.Fatalf("expected 'Dev Ops', got '%s'", devUser.Name)
	}

	// Delete user
	err = udp.Delete(ctx, devUser.ID)
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}
	if um.FindByEmail("devops@tgo.io") != nil {
		t.Fatalf("expected deleted user to no longer exist")
	}
}

func TestAuthSession_HMACSignatureTampering(t *testing.T) {
	engine := NewEngine("Session Test")

	rec := httptest.NewRecorder()
	testUser := &User{
		ID:          "42",
		Name:        "Test Operator",
		Email:       "operator@tgo.io",
		RoleName:    "Operator",
		PrivilegeID: "3",
	}

	// Set signed cookie
	engine.Auth.SetSessionUser(rec, testUser)
	cookie := rec.Result().Cookies()[0]
	if cookie.Name != "cb_session_token" {
		t.Fatalf("expected cookie name 'cb_session_token', got '%s'", cookie.Name)
	}

	// Verify valid cookie
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	user := engine.Auth.GetSessionUser(req)
	if user == nil || user.ID != "42" || user.Email != "operator@tgo.io" {
		t.Fatalf("failed to validate authentic signed session cookie")
	}

	// Tamper with cookie payload (e.g. attempt privilege escalation to Superadmin)
	parts := strings.Split(cookie.Value, ".")
	if len(parts) == 2 {
		tamperedValue := parts[0] + "tampered." + parts[1]
		tamperedCookie := &http.Cookie{
			Name:  "cb_session_token",
			Value: tamperedValue,
		}
		tamperedReq := httptest.NewRequest(http.MethodGet, "/admin", nil)
		tamperedReq.AddCookie(tamperedCookie)
		tamperedUser := engine.Auth.GetSessionUser(tamperedReq)
		if tamperedUser != nil {
			t.Fatalf("security violation: tampered cookie should have been rejected!")
		}
	}
}

func TestAuditLogger_PersistenceAndCap(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	engine := NewEngine("Audit Test")
	al := engine.AuditLogger

	req := httptest.NewRequest(http.MethodPost, "/admin/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	// Log several test actions
	al.Log(req, "CREATE", "TestModule", "Record #101 created")
	al.Log(req, "UPDATE", "TestModule", "Record #101 changed status to Active")
	al.Log(req, "DELETE", "TestModule", "Record #101 deleted")

	// Read via DataProvider
	ctx := &Context{Ctx: context.Background()}
	logs, err := al.NewDataProvider().FindAll(ctx)
	if err != nil {
		t.Fatalf("failed to query audit logs: %v", err)
	}
	if len(logs) < 3 {
		t.Fatalf("expected at least 3 audit logs, got %d", len(logs))
	}

	// Verify latest log is at the top
	latest := logs[0]
	if latest["action"] != "DELETE" {
		t.Fatalf("expected latest action to be 'DELETE', got '%v'", latest["action"])
	}

	// Verify persistence file exists
	if _, err := os.Stat(filepath.Join("data", "logs.json")); os.IsNotExist(err) {
		t.Fatalf("data/logs.json was not created")
	}
}

func TestCSVImport_ControllerHandler(t *testing.T) {
	engine := NewEngine("Import Test")

	memStore := NewMemoryStore()
	ctrl := &Controller{
		Title:        "Employees",
		Table:        "employees",
		PrimaryKey:   "id",
		DataProvider: memStore,
	}
	ctrl.AddCol("ID", "id", ColText, false, true).
		AddCol("Name", "name", ColText, true, true).
		AddCol("Department", "department", ColText, true, true).
		AddCol("Salary", "salary", ColNumber, true, true)

	ctrl.AddForm("Name", "name", InputText, true, "").
		AddForm("Department", "department", InputText, true, "").
		AddForm("Salary", "salary", InputNumber, true, "")

	engine.Register(ctrl)

	// Prepare CSV payload in multipart/form-data
	csvBody := `name,department,salary
Alice Smith,Engineering,95000
Bob Jones,Marketing,78000
Charlie Brown,Design,82000`

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("csv_file", "employees.csv")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	_, _ = part.Write([]byte(csvBody))
	_ = w.Close()

	// Authenticate as Superadmin
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &User{Name: "Super Admin", RoleName: "Superadmin"})
	cookie := recAuth.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/admin/employees/import-csv", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	ctrl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from import-csv, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"success":true`) {
		t.Fatalf("expected success true in response: %s", rec.Body.String())
	}

	// Verify imported rows in memory store
	ctx := &Context{Ctx: context.Background()}
	records, err := memStore.FindAll(ctx)
	if err != nil {
		t.Fatalf("failed to query records: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records imported, got %d", len(records))
	}
}

func TestEmailTemplates_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	engine := NewEngine("Email Test")
	etm := engine.EmailTemplates

	// Verify default seeds
	welcome := etm.FindBySlug("auth.welcome_user")
	if welcome == nil {
		t.Fatalf("expected seeded auth.welcome_user template")
	}

	// Test DataProvider Create
	edp := etm.NewDataProvider()
	ctx := &Context{Ctx: context.Background()}

	newTmpl := map[string]interface{}{
		"title":   "Invoice Generated",
		"slug":    "billing.invoice_ready",
		"subject": "Your Invoice #{{invoice_id}} is ready",
		"content": "<p>Hello {{customer_name}}, your invoice is ready for download.</p>",
		"status":  "Active",
	}
	err := edp.Create(ctx, newTmpl)
	if err != nil {
		t.Fatalf("failed to create email template: %v", err)
	}

	created := etm.FindBySlug("billing.invoice_ready")
	if created == nil {
		t.Fatalf("expected to find newly created email template by slug")
	}
	if created.Subject != "Your Invoice #{{invoice_id}} is ready" {
		t.Fatalf("unexpected subject: %s", created.Subject)
	}
}

func TestCustomPage_GetIndex_GetDetail_Overrides(t *testing.T) {
	engine := NewEngine("Custom Page Test")

	memStore := NewMemoryStore(
		map[string]interface{}{"id": "1", "name": "Enterprise Plan", "price": 999},
		map[string]interface{}{"id": "2", "name": "Startup Plan", "price": 199},
	)

	ctrl := &Controller{
		Title:        "Subscriptions",
		Table:        "subscriptions",
		PrimaryKey:   "id",
		DataProvider: memStore,
	}

	// 1. Override GetIndex
	ctrl.GetIndex = func(w http.ResponseWriter, r *http.Request) bool {
		ctrl.RenderView(w, r, "Custom Subscriptions Dashboard", "<h1>Custom Subscriptions Board</h1>")
		return true
	}

	// 2. Override GetDetail
	ctrl.GetDetail = func(w http.ResponseWriter, r *http.Request, id string) bool {
		rec := ctrl.FindRecord(r, id)
		if rec == nil {
			http.NotFound(w, r)
			return true
		}
		ctrl.RenderView(w, r, "Custom Detail #"+id, template.HTML("<h2>Plan: "+rec["name"].(string)+"</h2>"))
		return true
	}

	// 3. Add Custom Action
	ctrl.AddCustomAction("analytics", func(w http.ResponseWriter, r *http.Request, subPath string) {
		ctrl.RenderView(w, r, "Subscription Analytics", template.HTML("<div>Metric sub-path: "+subPath+"</div>"))
	})

	engine.Register(ctrl)

	// Auth cookie
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &User{Name: "Super Admin", RoleName: "Superadmin"})
	cookie := recAuth.Result().Cookies()[0]

	// Test 1: GetIndex Override
	reqIndex := httptest.NewRequest(http.MethodGet, "/admin/subscriptions", nil)
	reqIndex.AddCookie(cookie)
	recIndex := httptest.NewRecorder()
	ctrl.ServeHTTP(recIndex, reqIndex)

	if recIndex.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for custom GetIndex, got %d", recIndex.Code)
	}
	if !strings.Contains(recIndex.Body.String(), "Custom Subscriptions Board") {
		t.Fatalf("expected custom GetIndex body to contain custom text")
	}

	// Test 2: GetDetail Override
	reqDetail := httptest.NewRequest(http.MethodGet, "/admin/subscriptions/detail/1", nil)
	reqDetail.AddCookie(cookie)
	recDetail := httptest.NewRecorder()
	ctrl.ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for custom GetDetail, got %d", recDetail.Code)
	}
	if !strings.Contains(recDetail.Body.String(), "Plan: Enterprise Plan") {
		t.Fatalf("expected custom GetDetail body to contain plan name")
	}

	// Test 3: Custom Action /analytics/q3
	reqAction := httptest.NewRequest(http.MethodGet, "/admin/subscriptions/analytics/q3", nil)
	reqAction.AddCookie(cookie)
	recAction := httptest.NewRecorder()
	ctrl.ServeHTTP(recAction, reqAction)

	if recAction.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for custom action, got %d", recAction.Code)
	}
	if !strings.Contains(recAction.Body.String(), "Metric sub-path: q3") {
		t.Fatalf("expected custom action body to contain sub-path")
	}

	// Test 4: Standalone Custom Admin Page on Engine
	engine.AddCustomPage("/reports/revenue", "Executive Revenue Report", "<svg></svg>", func(w http.ResponseWriter, r *http.Request) template.HTML {
		return template.HTML("<div class='revenue-kpi'>Total Revenue: $500,000</div>")
	}, true)

	// Verify menu item was added
	foundMenu := false
	for _, m := range engine.menus {
		if m.Path == "/admin/reports/revenue" {
			foundMenu = true
			break
		}
	}
	if !foundMenu {
		t.Fatalf("expected menu item for custom admin page")
	}
}

func TestRenderViewFile_HTMLTemplateFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a real HTML template file
	detailFile := filepath.Join(tmpDir, "order_detail.html")
	htmlTemplateContent := `<div class="order-detail-card">
	<h2>Order #{{.Order.id}}</h2>
	<p>Customer: <strong>{{.Order.customer_name}}</strong></p>
	<p>Total: <strong>{{.Order.total_amount}}</strong></p>
	<ul>
		{{range .Items}}
		<li>{{.name}} - {{.qty}} pcs ({{.subtotal}})</li>
		{{end}}
	</ul>
</div>`
	if err := os.WriteFile(detailFile, []byte(htmlTemplateContent), 0644); err != nil {
		t.Fatalf("failed to create temp template file: %v", err)
	}

	engine := NewEngine("Template File Test")
	memStore := NewMemoryStore(
		map[string]interface{}{"id": "101", "customer_name": "John Doe", "total_amount": "Rp 5.000.000"},
	)

	ctrl := &Controller{
		Title:        "Orders",
		Table:        "orders",
		PrimaryKey:   "id",
		DataProvider: memStore,
	}

	// 2. Wire GetDetail to render from the external HTML file!
	ctrl.GetDetail = func(w http.ResponseWriter, r *http.Request, id string) bool {
		order := ctrl.FindRecord(r, id)
		if order == nil {
			http.NotFound(w, r)
			return true
		}

		data := map[string]interface{}{
			"Order": order,
			"Items": []map[string]interface{}{
				{"name": "Mechanical Keyboard", "qty": 1, "subtotal": "Rp 2.000.000"},
				{"name": "Noise Cancelling Headset", "qty": 1, "subtotal": "Rp 3.000.000"},
			},
		}

		err := ctrl.RenderViewFile(w, r, "Order Detail #"+id, detailFile, data)
		if err != nil {
			t.Fatalf("RenderViewFile returned error: %v", err)
		}
		return true
	}

	engine.Register(ctrl)

	// Auth cookie
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &User{Name: "Super Admin", RoleName: "Superadmin"})
	cookie := recAuth.Result().Cookies()[0]

	// Request /admin/orders/detail/101
	req := httptest.NewRequest(http.MethodGet, "/admin/orders/detail/101", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()

	ctrl.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from detail page, got %d: %s", rec.Code, rec.Body.String())
	}

	respBody := rec.Body.String()
	if !strings.Contains(respBody, "Order #101") {
		t.Fatalf("expected template output to contain 'Order #101'")
	}
	if !strings.Contains(respBody, "Customer: <strong>John Doe</strong>") {
		t.Fatalf("expected template output to contain customer name from data")
	}
	if !strings.Contains(respBody, "Noise Cancelling Headset - 1 pcs") {
		t.Fatalf("expected template output to contain rendered item list from HTML file")
	}

	// 3. Test AddCustomPageWithFile
	pageFile := filepath.Join(tmpDir, "report_quarterly.html")
	pageHTMLContent := `<div class="quarterly-card">
	<h3>Quarterly Executive Report</h3>
	<span class="badge">{{.Quarter}}</span>
	<p>Total Revenue: {{.Revenue}}</p>
</div>`
	if err := os.WriteFile(pageFile, []byte(pageHTMLContent), 0644); err != nil {
		t.Fatalf("failed to create temp page template: %v", err)
	}

	engine.AddCustomPageWithFile("/reports/q4", "Q4 Report", "<svg></svg>", pageFile, func(r *http.Request) interface{} {
		return map[string]interface{}{
			"Quarter": "Q4-2026",
			"Revenue": "Rp 750.000.000",
		}
	}, true)

	reqPage := httptest.NewRequest(http.MethodGet, "/admin/reports/q4", nil)
	reqPage.AddCookie(cookie)
	recPage := httptest.NewRecorder()

	for _, cp := range engine.customAdminPages {
		if cp.Path == "/admin/reports/q4" {
			htmlContent := cp.Handler(recPage, reqPage)
			engine.RenderLayout(recPage, reqPage, cp.Title, htmlContent)
			break
		}
	}

	if !strings.Contains(recPage.Body.String(), "Quarterly Executive Report") || !strings.Contains(recPage.Body.String(), "Q4-2026") {
		t.Fatalf("expected custom page with file to render template correctly")
	}
}

func TestSQLStore_AutoMigrate_SQLite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_automigrate.db")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open sqlite database: %v", err)
	}
	defer db.Close()

	engine := NewEngine("AutoMigrate App")
	engine.SetDB(db)

	sqlStore := NewSQLStore(db, "blog_posts", "id").SetDriver("sqlite3")
	ctrl := &Controller{
		Title:        "Blog Posts",
		Table:        "blog_posts",
		PrimaryKey:   "id",
		DataProvider: sqlStore,
	}
	ctrl.AddCol("ID", "id", ColText, false, true).
		AddCol("Title", "title", ColText, true, true).
		AddCol("Views", "views", ColNumber, true, true)

	ctrl.AddForm("Title", "title", InputText, true, "").
		AddForm("Slug", "slug", InputText, false, "").
		AddForm("Content", "content", InputWYSIWYG, false, "").
		AddForm("Views", "views", InputNumber, false, "").
		AddForm("Status", "status", InputSelect, true, "")

	engine.Register(ctrl)

	// Test AutoMigrate on controller
	if err := ctrl.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Verify table exists by inserting and finding a record
	ctx := &Context{Ctx: context.Background()}
	err = sqlStore.Create(ctx, map[string]interface{}{
		"title":   "Hello TGo Booster",
		"slug":    "hello-tgo-booster",
		"content": "<p>TGo Booster as base framework!</p>",
		"views":   42,
		"status":  "Published",
	})
	if err != nil {
		t.Fatalf("failed to insert record into auto-migrated table: %v", err)
	}

	records, err := sqlStore.FindAll(ctx)
	if err != nil {
		t.Fatalf("failed to query records from auto-migrated table: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["title"] != "Hello TGo Booster" {
		t.Fatalf("expected title 'Hello TGo Booster', got '%v'", records[0]["title"])
	}

	// Test Engine.AutoMigrateAll
	if err := engine.AutoMigrateAll(); err != nil {
		t.Fatalf("Engine.AutoMigrateAll failed: %v", err)
	}
}


