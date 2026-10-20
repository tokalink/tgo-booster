package cb

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tokalink/tgo/pkg/transport/connect"
)

func newSuperadminCookie(t *testing.T, e *Engine) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	e.Auth.SetSessionUser(rec, &User{
		ID:          "1",
		Name:        "Super Admin",
		Email:       "super@example.com",
		RoleName:    "Superadmin",
		PrivilegeID: "1",
		Tenants:     []string{"*"},
	})
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	return cookies[0]
}

func TestWave2B_ModuleGeneratorValidationAndCodegen(t *testing.T) {
	t.Chdir(t.TempDir())

	e := NewEngine("GenTest")
	srv := connect.NewServer()
	e.Mount(srv, "/admin")
	h := srv.Handler()
	superCookie := newSuperadminCookie(t, e)

	postSave := func(body map[string]interface{}) *httptest.ResponseRecorder {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/admin/module_generator/save", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(superCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// 1. Invalid table names & system module names -> 400
	badTables := []string{
		"../../evil",
		"x;DROP TABLE y",
		"users",
		"privileges",
		"menus",
		"pages",
		"settings",
		"module_generator",
		"api_generator",
		"tenants",
		"logs",
		"email_templates",
		"dashboard",
		"login",
		"logout",
		"a b",
		"123abc",
	}
	for _, tbl := range badTables {
		rec := postSave(map[string]interface{}{
			"title": "Test",
			"table": tbl,
			"columns": []map[string]interface{}{
				{"label": "Name", "name": "name", "type": "text"},
			},
			"forms": []map[string]interface{}{
				{"label": "Name", "name": "name", "type": "text"},
			},
		})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("table %q: expected 400, got %d (%s)", tbl, rec.Code, rec.Body.String())
		}
	}

	// 2. Invalid column name -> 400
	recBadCol := postSave(map[string]interface{}{
		"title": "Safe Title",
		"table": "safe_items",
		"columns": []map[string]interface{}{
			{"label": "Name", "name": `name"); os.Exit(1); //`, "type": "text"},
		},
	})
	if recBadCol.Code != http.StatusBadRequest {
		t.Fatalf("bad column name: expected 400, got %d (%s)", recBadCol.Code, recBadCol.Body.String())
	}

	// 3. Invalid form name / DataTable / OrderBy / ProviderType -> 400
	recBadForm := postSave(map[string]interface{}{
		"title": "Safe Title",
		"table": "safe_items",
		"forms": []map[string]interface{}{
			{"label": "Name", "name": `name"); os.Exit(1); //`, "type": "text"},
		},
	})
	if recBadForm.Code != http.StatusBadRequest {
		t.Fatalf("bad form name: expected 400, got %d", recBadForm.Code)
	}

	recBadOrder := postSave(map[string]interface{}{
		"title":    "Safe Title",
		"table":    "safe_items",
		"order_by": "id DESC; DROP TABLE users",
	})
	if recBadOrder.Code != http.StatusBadRequest {
		t.Fatalf("bad order_by: expected 400, got %d", recBadOrder.Code)
	}

	recBadProvider := postSave(map[string]interface{}{
		"title":         "Safe Title",
		"table":         "safe_items",
		"provider_type": "evil_provider",
	})
	if recBadProvider.Code != http.StatusBadRequest {
		t.Fatalf("bad provider_type: expected 400, got %d", recBadProvider.Code)
	}

	recBadDT := postSave(map[string]interface{}{
		"title": "Safe Title",
		"table": "safe_items",
		"forms": []map[string]interface{}{
			{"label": "Ref", "name": "ref_id", "type": "select", "datatable": "a,b,c,d"},
		},
	})
	if recBadDT.Code != http.StatusBadRequest {
		t.Fatalf("bad data_table: expected 400, got %d", recBadDT.Code)
	}

	// 4. api/columns, api/module, delete with unsafe table -> 400; delete non-generated -> 404
	reqCols := httptest.NewRequest(http.MethodGet, "/admin/module_generator/api/columns?table=x%60;", nil)
	reqCols.AddCookie(superCookie)
	recCols := httptest.NewRecorder()
	h.ServeHTTP(recCols, reqCols)
	if recCols.Code != http.StatusBadRequest {
		t.Fatalf("api/columns unsafe table: expected 400, got %d", recCols.Code)
	}

	reqMod := httptest.NewRequest(http.MethodGet, "/admin/module_generator/api/module?table=x%60;", nil)
	reqMod.AddCookie(superCookie)
	recMod := httptest.NewRecorder()
	h.ServeHTTP(recMod, reqMod)
	if recMod.Code != http.StatusBadRequest {
		t.Fatalf("api/module unsafe table: expected 400, got %d", recMod.Code)
	}

	reqDelBad := httptest.NewRequest(http.MethodPost, "/admin/module_generator/delete?table=x%60;", nil)
	reqDelBad.AddCookie(superCookie)
	recDelBad := httptest.NewRecorder()
	h.ServeHTTP(recDelBad, reqDelBad)
	if recDelBad.Code != http.StatusBadRequest {
		t.Fatalf("delete unsafe table: expected 400, got %d", recDelBad.Code)
	}

	reqDelNotGen := httptest.NewRequest(http.MethodPost, "/admin/module_generator/delete?table=not_generated_mod", nil)
	reqDelNotGen.AddCookie(superCookie)
	recDelNotGen := httptest.NewRecorder()
	h.ServeHTTP(recDelNotGen, reqDelNotGen)
	if recDelNotGen.Code != http.StatusNotFound {
		t.Fatalf("delete non-generated module: expected 404, got %d", recDelNotGen.Code)
	}

	// 5. Title with newline injection, Label/Placeholder with quotes & backticks, Icon with onload
	if err := os.MkdirAll(filepath.Join("app", "controllers"), 0755); err != nil {
		t.Fatalf("mkdir app/controllers: %v", err)
	}

	recSave := postSave(map[string]interface{}{
		"title":         "Foo\n}\nfunc init(){panic(1)}",
		"table":         "custom_widgets",
		"icon":          "<svg onload=\"alert(1)\"><path d=\"M1 1\"/></svg>` + os.Getenv(\"SECRET\") + `",
		"primary_key":   "id",
		"order_by":      "id DESC",
		"provider_type": "memory",
		"columns": []map[string]interface{}{
			{"label": "Quotes \" and `backtick`", "name": "widget_name", "type": "text", "searchable": true, "sortable": true},
		},
		"forms": []map[string]interface{}{
			{
				"label":       "Label \"with\" `backtick`\nnewline",
				"name":        "widget_name",
				"type":        "text",
				"required":    true,
				"placeholder": "Enter \"value\" `here`",
			},
			{
				"label": "Status \"quote\"",
				"name":  "status",
				"type":  "select",
				"options": []map[string]string{
					{"value": "a\"b`c", "label": "Label \"a\" `b`"},
				},
			},
		},
	})
	if recSave.Code != http.StatusOK {
		t.Fatalf("expected 200 saving module with special chars in title/labels, got %d: %s", recSave.Code, recSave.Body.String())
	}

	genPath := filepath.Join("app", "controllers", "admin_custom_widgets_controller.go")
	codeBytes, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("read generated controller: %v", err)
	}
	codeStr := string(codeBytes)
	if !strings.HasPrefix(codeStr, "// Code generated by TGo Booster Module Generator. DO NOT EDIT.") {
		t.Fatalf("generated file missing header marker:\n%s", codeStr)
	}
	if strings.Contains(codeStr, "\nfunc init") {
		t.Fatalf("generated code must not contain injected func init line:\n%s", codeStr)
	}
	if strings.Contains(strings.ToLower(codeStr), "onload") {
		t.Fatalf("generated code must not contain onload in icon:\n%s", codeStr)
	}
	if _, err := format.Source(codeBytes); err != nil {
		t.Fatalf("generated code failed go/format.Source: %v\n%s", err, codeStr)
	}
	fset := token.NewFileSet()
	parsedFile, err := parser.ParseFile(fset, genPath, codeBytes, parser.AllErrors)
	if err != nil {
		t.Fatalf("parser.ParseFile failed: %v\n%s", err, codeStr)
	}
	for _, decl := range parsedFile.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "init" {
			t.Fatalf("generated AST must not contain func init declaration:\n%s", codeStr)
		}
	}

	savedCtrl := e.FindController("custom_widgets")
	if savedCtrl == nil {
		t.Fatal("expected custom_widgets controller to be registered")
	}
	if strings.Contains(strings.ToLower(savedCtrl.Icon), "onload") {
		t.Fatalf("expected saved controller Icon to be sanitized without onload, got %q", savedCtrl.Icon)
	}

	// 6. Hand-written file (without marker) must NOT be overwritten; response has "file_skipped": true
	handPath := filepath.Join("app", "controllers", "admin_hand_written_controller.go")
	handContent := "package controllers\n\n// Hand-written controller\nvar HandWritten = true\n"
	if err := os.WriteFile(handPath, []byte(handContent), 0644); err != nil {
		t.Fatalf("write hand-written controller: %v", err)
	}

	recHand := postSave(map[string]interface{}{
		"title":         "Hand Written",
		"table":         "hand_written",
		"provider_type": "memory",
		"columns": []map[string]interface{}{
			{"label": "Name", "name": "name", "type": "text"},
		},
		"forms": []map[string]interface{}{
			{"label": "Name", "name": "name", "type": "text"},
		},
	})
	if recHand.Code != http.StatusOK {
		t.Fatalf("expected 200 when saving module with existing hand-written file, got %d: %s", recHand.Code, recHand.Body.String())
	}
	var handResp map[string]interface{}
	if err := json.Unmarshal(recHand.Body.Bytes(), &handResp); err != nil {
		t.Fatalf("unmarshal hand response: %v", err)
	}
	if handResp["file_skipped"] != true {
		t.Fatalf("expected file_skipped=true in response, got %+v", handResp)
	}
	afterHand, err := os.ReadFile(handPath)
	if err != nil {
		t.Fatalf("read hand-written file after save: %v", err)
	}
	if string(afterHand) != handContent {
		t.Fatalf("hand-written file was modified! got:\n%s", string(afterHand))
	}
	if e.FindController("hand_written") == nil {
		t.Fatal("expected hand_written controller to still be registered at runtime")
	}
}

func TestWave2B_TenantStrictSQLiteHTTPAndREST(t *testing.T) {
	t.Chdir(t.TempDir())

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	store := NewSQLStore(db, "invoices", "id").SetDriver("sqlite").SetTenantScoped(true)
	if err := store.AutoMigrate(
		Field{Name: "title", Type: InputText},
	); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}

	// Non-strict mode: nil/empty tenant falls back to "default"
	if err := store.Create(nil, map[string]interface{}{"title": "Default Invoice"}); err != nil {
		t.Fatalf("non-strict Create with nil ctx: %v", err)
	}
	defRows, err := store.FindAll(nil)
	if err != nil || len(defRows) != 1 {
		t.Fatalf("non-strict FindAll expected 1 default row, got %d (err=%v)", len(defRows), err)
	}
	defaultID := defRows[0]["id"].(string)

	// Enable strict tenancy on store
	store.SetStrictTenancy(true)

	// Empty tenant context must return ErrTenantRequired on FindAll/FindByID/Create/Update/Delete
	emptyCtx := &Context{TenantSlug: ""}
	if rows, err := store.FindAll(emptyCtx); !errors.Is(err, ErrTenantRequired) || len(rows) != 0 {
		t.Fatalf("strict FindAll with empty tenant: expected ErrTenantRequired, got rows=%v err=%v", rows, err)
	}
	if _, err := store.FindByID(emptyCtx, defaultID); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("strict FindByID with empty tenant: expected ErrTenantRequired, got %v", err)
	}
	if err := store.Create(emptyCtx, map[string]interface{}{"title": "Should Fail"}); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("strict Create with empty tenant: expected ErrTenantRequired, got %v", err)
	}
	if err := store.Update(emptyCtx, defaultID, map[string]interface{}{"title": "Should Fail"}); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("strict Update with empty tenant: expected ErrTenantRequired, got %v", err)
	}
	if err := store.Delete(emptyCtx, defaultID); !errors.Is(err, ErrTenantRequired) {
		t.Fatalf("strict Delete with empty tenant: expected ErrTenantRequired, got %v", err)
	}

	// Seed tenant-a and tenant-b rows
	ctxA := &Context{TenantSlug: "tenant-a"}
	ctxB := &Context{TenantSlug: "tenant-b"}
	if err := store.Create(ctxA, map[string]interface{}{"title": "Invoice A"}); err != nil {
		t.Fatalf("Create tenant-a: %v", err)
	}
	if err := store.Create(ctxB, map[string]interface{}{"title": "Invoice B"}); err != nil {
		t.Fatalf("Create tenant-b: %v", err)
	}

	// Wildcard "*" still reads all rows
	allRows, err := store.FindAll(&Context{TenantSlug: "*"})
	if err != nil || len(allRows) != 3 {
		t.Fatalf("wildcard FindAll expected 3 rows, got %d (err=%v)", len(allRows), err)
	}

	// Turning strict off restores "default" fallback
	store.SetStrictTenancy(false)
	defAgain, err := store.FindAll(emptyCtx)
	if err != nil || len(defAgain) != 1 || defAgain[0]["title"] != "Default Invoice" {
		t.Fatalf("expected non-strict fallback to read Default Invoice, got %v (err=%v)", defAgain, err)
	}

	// HTTP & REST test: Engine.StrictTenancy = true set AFTER Mount
	// Use a fresh SQLStore without SetStrictTenancy pre-configured so initDefaults applies it per request.
	freshStore := NewSQLStore(db, "invoices", "id").SetDriver("sqlite")
	e := NewEngine("StrictTenantApp")
	e.MultiTenancyEnabled = true

	invCtrl := &Controller{
		Title:        "Invoices",
		Table:        "invoices",
		DataProvider: freshStore,
		PublicAPI:    true,
	}
	invCtrl.AddCol("ID", "id", ColText, false, true).
		AddCol("Title", "title", ColText, true, true)
	invCtrl.AddForm("Title", "title", InputText, true, "")

	// Register and Mount BEFORE setting StrictTenancy = true
	e.Register(invCtrl)

	srv := connect.NewServer()
	e.Mount(srv, "/admin")
	e.appendRole(&Role{
		ID:   "role-staff",
		Name: "Staff",
		Slug: "staff",
		Permissions: map[string]PermissionMatrix{
			"invoices": {IsVisible: true, CanRead: true, CanCreate: true, CanUpdate: true, CanDelete: true},
		},
	})
	h := srv.Handler()

	// Create two API tokens: one bound to tenant-a, one without tenant
	tokBound := &APIToken{
		Name:        "Tenant A Client",
		Environment: "production",
		Token:       GenerateSecureAPIToken("production"),
		Scopes:      []string{"read:invoices", "write:invoices"},
		Tenant:      "tenant-a",
		Status:      "active",
	}
	if err := e.SaveAPIToken(tokBound); err != nil {
		t.Fatalf("SaveAPIToken bound: %v", err)
	}
	tokUnbound := &APIToken{
		Name:        "Unbound Client",
		Environment: "production",
		Token:       GenerateSecureAPIToken("production"),
		Scopes:      []string{"read:invoices", "write:invoices"},
		Tenant:      "",
		Status:      "active",
	}
	if err := e.SaveAPIToken(tokUnbound); err != nil {
		t.Fatalf("SaveAPIToken unbound: %v", err)
	}

	// Set StrictTenancy = true AFTER Mount (as Axera does)
	e.StrictTenancy = true

	// 1. REST BEFORE admin page is ever opened: token bound to tenant-a must only see Invoice A
	reqRESTBound := httptest.NewRequest(http.MethodGet, "/api/v1/invoices", nil)
	reqRESTBound.Header.Set("Authorization", "Bearer "+tokBound.Token)
	recRESTBound := httptest.NewRecorder()
	h.ServeHTTP(recRESTBound, reqRESTBound)
	if recRESTBound.Code != http.StatusOK {
		t.Fatalf("REST bound token: expected 200, got %d (%s)", recRESTBound.Code, recRESTBound.Body.String())
	}
	if !strings.Contains(recRESTBound.Body.String(), "Invoice A") ||
		strings.Contains(recRESTBound.Body.String(), "Invoice B") ||
		strings.Contains(recRESTBound.Body.String(), "Default Invoice") {
		t.Fatalf("REST bound token did not filter by tenant-a: %s", recRESTBound.Body.String())
	}

	// 2. REST with unbound token + StrictTenancy -> 403 "token API tidak terikat tenant"
	reqRESTUnbound := httptest.NewRequest(http.MethodGet, "/api/v1/invoices", nil)
	reqRESTUnbound.Header.Set("Authorization", "Bearer "+tokUnbound.Token)
	recRESTUnbound := httptest.NewRecorder()
	h.ServeHTTP(recRESTUnbound, reqRESTUnbound)
	if recRESTUnbound.Code != http.StatusForbidden {
		t.Fatalf("REST unbound token in strict mode: expected 403, got %d (%s)", recRESTUnbound.Code, recRESTUnbound.Body.String())
	}
	if !strings.Contains(recRESTUnbound.Body.String(), "token API tidak terikat tenant") {
		t.Fatalf("expected error message 'token API tidak terikat tenant', got: %s", recRESTUnbound.Body.String())
	}

	// 3. HTTP: non-superadmin without tenant opening tenant-scoped module -> must NOT see "Default Invoice"
	recNoTenantAuth := httptest.NewRecorder()
	e.Auth.SetSessionUser(recNoTenantAuth, &User{
		ID:          "99",
		Name:        "No Tenant Staff",
		Email:       "notenant@example.com",
		RoleName:    "Staff",
		PrivilegeID: "role-staff",
		Tenants:     nil,
	})
	noTenantCookie := recNoTenantAuth.Result().Cookies()[0]

	reqHTTP := httptest.NewRequest(http.MethodGet, "/admin/invoices", nil)
	reqHTTP.AddCookie(noTenantCookie)
	recHTTP := httptest.NewRecorder()
	h.ServeHTTP(recHTTP, reqHTTP)
	if recHTTP.Code != http.StatusOK {
		t.Fatalf("HTTP /admin/invoices: expected 200, got %d", recHTTP.Code)
	}
	if strings.Contains(recHTTP.Body.String(), "Default Invoice") || strings.Contains(recHTTP.Body.String(), "Invoice A") {
		t.Fatalf("non-superadmin without tenant must not see default or tenant rows in HTML")
	}

	reqJSON := httptest.NewRequest(http.MethodGet, "/admin/invoices/data", nil)
	reqJSON.AddCookie(noTenantCookie)
	recJSON := httptest.NewRecorder()
	h.ServeHTTP(recJSON, reqJSON)
	if recJSON.Code != http.StatusOK {
		t.Fatalf("HTTP /admin/invoices/data: expected 200, got %d", recJSON.Code)
	}
	if strings.Contains(recJSON.Body.String(), "Default Invoice") || strings.Contains(recJSON.Body.String(), "Invoice A") {
		t.Fatalf("non-superadmin without tenant must not see default or tenant rows in JSON: %s", recJSON.Body.String())
	}
}

func TestWave2B_UsersCrossTenantIsolation(t *testing.T) {
	t.Chdir(t.TempDir())

	e := NewEngine("UserIsolationApp")
	e.MultiTenancyEnabled = true
	e.StrictTenancy = true

	e.appendRole(&Role{
		ID:   "role-mgr",
		Name: "Tenant Manager",
		Slug: "tenant_manager",
		Permissions: map[string]PermissionMatrix{
			"users": {IsVisible: true, CanRead: true, CanCreate: true, CanUpdate: true, CanDelete: true},
		},
	})

	um := e.UserManager
	// Create actor (tenant "a"), userAOnly (tenant "a"), userAB (tenants "a", "b"), userB (tenant "b")
	actorAcc, err := um.CreateUser("Actor A", "actor-a@example.com", "Secret123!", "Tenant Manager", "role-mgr", "", "Active")
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}
	_ = um.SetTenants(actorAcc.ID, []string{"a"}, "a")

	userAOnly, err := um.CreateUser("User A Only", "only-a@example.com", "Secret123!", "Tenant Manager", "role-mgr", "", "Active")
	if err != nil {
		t.Fatalf("create userAOnly: %v", err)
	}
	_ = um.SetTenants(userAOnly.ID, []string{"a"}, "a")

	userAB, err := um.CreateUser("User A and B", "ab@example.com", "Secret123!", "Tenant Manager", "role-mgr", "", "Active")
	if err != nil {
		t.Fatalf("create userAB: %v", err)
	}
	_ = um.SetTenants(userAB.ID, []string{"a", "b"}, "a")

	userB, err := um.CreateUser("User B Only", "only-b@example.com", "Secret123!", "Tenant Manager", "role-mgr", "", "Active")
	if err != nil {
		t.Fatalf("create userB: %v", err)
	}
	_ = um.SetTenants(userB.ID, []string{"b"}, "b")

	dp := um.NewDataProvider()
	actorCtx := &Context{
		User: &User{
			ID:           actorAcc.ID,
			Name:         actorAcc.Name,
			Email:        actorAcc.Email,
			RoleName:     "Tenant Manager",
			PrivilegeID:  "role-mgr",
			Tenants:      []string{"a"},
			ActiveTenant: "a",
		},
		TenantSlug: "a",
	}

	rows, err := dp.FindAll(actorCtx)
	if err != nil {
		t.Fatalf("FindAll as actor A: %v", err)
	}

	seenEmails := map[string]bool{}
	for _, r := range rows {
		seenEmails[r["email"].(string)] = true
	}

	if !seenEmails["actor-a@example.com"] {
		t.Fatalf("expected actor to see own account, got %v", seenEmails)
	}
	if !seenEmails["only-a@example.com"] {
		t.Fatalf("expected actor to see tenant-a-only account, got %v", seenEmails)
	}
	if seenEmails["ab@example.com"] {
		t.Fatalf("actor must NOT see multi-tenant account (a,b): %v", seenEmails)
	}
	if seenEmails["only-b@example.com"] {
		t.Fatalf("actor must NOT see tenant-b account: %v", seenEmails)
	}
	for _, r := range rows {
		if strings.EqualFold(r["role"].(string), "Superadmin") || r["id"] == "1" {
			t.Fatalf("actor must NOT see superadmin account: %v", r)
		}
	}

	// FindByID for tenant-b, multi-tenant (a,b), and superadmin (#1) must fail with "user not found"
	for _, forbiddenID := range []string{userB.ID, userAB.ID, "1"} {
		if rec, err := dp.FindByID(actorCtx, forbiddenID); err == nil || rec != nil {
			t.Fatalf("expected FindByID(%s) to fail for non-superadmin, got rec=%v err=%v", forbiddenID, rec, err)
		}
	}

	// FindByID for self and userAOnly must succeed
	if rec, err := dp.FindByID(actorCtx, actorAcc.ID); err != nil || rec["email"] != "actor-a@example.com" {
		t.Fatalf("expected FindByID(self) to succeed, got rec=%v err=%v", rec, err)
	}
	if rec, err := dp.FindByID(actorCtx, userAOnly.ID); err != nil || rec["email"] != "only-a@example.com" {
		t.Fatalf("expected FindByID(userAOnly) to succeed, got rec=%v err=%v", rec, err)
	}

	// Internal call (nil ctx) and Superadmin call still return all accounts
	allInternal, err := dp.FindAll(nil)
	if err != nil || len(allInternal) < 5 {
		t.Fatalf("expected internal FindAll(nil) to return all accounts, got %d (err=%v)", len(allInternal), err)
	}
}

func TestWave2B_InitDefaultsConcurrentRace(t *testing.T) {
	t.Chdir(t.TempDir())

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	sqlStore := NewSQLStore(db, "tasks", "id").SetDriver("sqlite").SetTenantScoped(true)
	if err := sqlStore.AutoMigrate(Field{Name: "name", Type: InputText}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	_ = sqlStore.Create(&Context{TenantSlug: "t1"}, map[string]interface{}{"name": "Task 1"})

	e := NewEngine("RaceTestApp")
	e.MultiTenancyEnabled = true

	ctrl := &Controller{
		Title:        "Tasks",
		Table:        "tasks",
		DataProvider: sqlStore,
		PublicAPI:    true,
	}
	ctrl.AddCol("ID", "id", ColText, false, true).
		AddCol("Name", "name", ColText, true, true)
	ctrl.AddForm("Name", "name", InputText, true, "")
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv, "/admin")
	e.StrictTenancy = true
	h := srv.Handler()

	cookie := newSuperadminCookie(t, e)

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/admin/tasks/data", nil)
			req.AddCookie(cookie)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent GET /admin/tasks/data status=%d", rec.Code)
			}
		}()
	}
	wg.Wait()
}
