package booster

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tokalink/tgo/pkg/transport/connect"
	"github.com/tokalink/tgo-booster/pkg/cb"
)

func TestBoosterFullFlow(t *testing.T) {
	engine := cb.NewEngine("Test Admin")

	ctrl := &cb.Controller{
		Title: "Products",
		Table: "products",
	}
	ctrl.
		AddCol("ID", "id", cb.ColText, false, true).
		AddCol("Name", "name", cb.ColText, true, true)

	ctrl.
		AddForm("Name", "name", cb.InputText, true, "Product name")

	engine.Register(ctrl)

	// 1. Test Login View
	reqLogin := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	recLogin := httptest.NewRecorder()
	engine.Auth.ServeLogin(recLogin, reqLogin)

	if recLogin.Code != http.StatusOK {
		t.Fatalf("expected status 200 on login, got %d", recLogin.Code)
	}
	if !strings.Contains(recLogin.Body.String(), "Sign in to access your administrative control plane") {
		t.Fatalf("expected login body to contain sign in text")
	}

	// 2. Test Login Authentication Success
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{Name: "Super Admin", RoleName: "Admin"})
	cookie := recAuth.Result().Cookies()[0]

	// 3. Test Dashboard View with Session
	reqDash := httptest.NewRequest(http.MethodGet, "/admin", nil)
	reqDash.AddCookie(cookie)
	recDash := httptest.NewRecorder()
	engine.Dashboard.ServeHTTP(recDash, reqDash)

	if recDash.Code != http.StatusOK {
		t.Fatalf("expected status 200 on dashboard, got %d: %s", recDash.Code, recDash.Body.String())
	}
	if !strings.Contains(recDash.Body.String(), "Gross Revenue") {
		t.Fatalf("expected dashboard body to contain KPI stats")
	}

	// 4. Test CRUD Module Index View with Session
	reqCrud := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	reqCrud.AddCookie(cookie)
	recCrud := httptest.NewRecorder()
	ctrl.ServeHTTP(recCrud, reqCrud)

	if recCrud.Code != http.StatusOK {
		t.Fatalf("expected status 200 on crud index, got %d", recCrud.Code)
	}
	if !strings.Contains(recCrud.Body.String(), "Products") {
		t.Fatalf("expected crud html to contain title")
	}
}

func TestModuleGeneratorCRUDBoosterFlow(t *testing.T) {
	engine := cb.NewEngine("Test Admin")

	ctrl := &cb.Controller{
		Title: "Products",
		Table: "products",
	}
	ctrl.
		AddCol("ID", "id", cb.ColText, false, true).
		AddCol("Product Name", "name", cb.ColText, true, true)
	ctrl.
		AddForm("Product Name", "name", cb.InputText, true, "Product name")

	engine.Register(ctrl)

	// Create mock HTTP server with connect.NewServer
	server := connect.NewServer()
	engine.Mount(server, "/admin")

	handler := server.Handler()

	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{Name: "Super Administrator", RoleName: "Superadmin", PrivilegeID: "1"})
	cookie := recAuth.Result().Cookies()[0]

	// 1. Test GET /admin/module_generator UI
	reqStudio := httptest.NewRequest(http.MethodGet, "/admin/module_generator", nil)
	reqStudio.AddCookie(cookie)
	recStudio := httptest.NewRecorder()
	handler.ServeHTTP(recStudio, reqStudio)

	if recStudio.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /admin/module_generator, got %d", recStudio.Code)
	}
	body := recStudio.Body.String()
	if !strings.Contains(body, "Module Studio Generator") {
		t.Fatalf("expected Module Studio Generator title in HTML, got: %s", body[:min(len(body), 500)])
	}
	if !strings.Contains(body, "CRUDBooster") {
		t.Fatalf("expected CRUDBooster branding in HTML")
	}
	if !strings.Contains(body, "Active Module Directory") {
		t.Fatalf("expected Active Module Directory table in HTML")
	}

	// 2. Test GET /admin/module_generator/api/tables
	reqTables := httptest.NewRequest(http.MethodGet, "/admin/module_generator/api/tables", nil)
	reqTables.AddCookie(cookie)
	recTables := httptest.NewRecorder()
	handler.ServeHTTP(recTables, reqTables)

	if recTables.Code != http.StatusOK {
		t.Fatalf("expected status 200 on api/tables, got %d", recTables.Code)
	}
	if !strings.Contains(recTables.Body.String(), "products") {
		t.Fatalf("expected products table in api/tables response: %s", recTables.Body.String())
	}

	// 3. Test GET /admin/module_generator/api/columns?table=products
	reqCols := httptest.NewRequest(http.MethodGet, "/admin/module_generator/api/columns?table=products", nil)
	reqCols.AddCookie(cookie)
	recCols := httptest.NewRecorder()
	handler.ServeHTTP(recCols, reqCols)

	if recCols.Code != http.StatusOK {
		t.Fatalf("expected status 200 on api/columns, got %d", recCols.Code)
	}
	if !strings.Contains(recCols.Body.String(), "Product Name") {
		t.Fatalf("expected Product Name column in api/columns response: %s", recCols.Body.String())
	}

	// 4. Test GET /admin/module_generator/api/module?table=products
	reqMod := httptest.NewRequest(http.MethodGet, "/admin/module_generator/api/module?table=products", nil)
	reqMod.AddCookie(cookie)
	recMod := httptest.NewRecorder()
	handler.ServeHTTP(recMod, reqMod)

	if recMod.Code != http.StatusOK {
		t.Fatalf("expected status 200 on api/module, got %d", recMod.Code)
	}
	if !strings.Contains(recMod.Body.String(), `"title":"Products"`) {
		t.Fatalf("expected Products title in api/module response: %s", recMod.Body.String())
	}

	// 5. Test POST /admin/module_generator/save to hot-deploy a new module (suppliers)
	payload := `{
		"title": "Suppliers",
		"table": "suppliers",
		"icon": "<svg><rect/></svg>",
		"primary_key": "id",
		"order_by": "id DESC",
		"provider_type": "memory",
		"columns": [
			{"label": "ID", "name": "id", "type": "text", "searchable": false, "sortable": true},
			{"label": "Supplier Name", "name": "name", "type": "text", "searchable": true, "sortable": true},
			{"label": "Status", "name": "status", "type": "badge", "searchable": false, "sortable": false}
		],
		"forms": [
			{"label": "Supplier Name", "name": "name", "type": "text", "required": true, "placeholder": "e.g. Acme Corp"},
			{"label": "Status", "name": "status", "type": "select", "required": true, "options": [{"value":"Active","label":"Active"}]}
		]
	}`

	reqSave := httptest.NewRequest(http.MethodPost, "/admin/module_generator/save", strings.NewReader(payload))
	reqSave.Header.Set("Content-Type", "application/json")
	reqSave.AddCookie(cookie)
	recSave := httptest.NewRecorder()
	handler.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("expected status 200 on module save, got %d: %s", recSave.Code, recSave.Body.String())
	}
	if !strings.Contains(recSave.Body.String(), `"success":true`) {
		t.Fatalf("expected success true in save response: %s", recSave.Body.String())
	}

	// Verify controller is registered
	suppCtrl := engine.FindController("suppliers")
	if suppCtrl == nil {
		t.Fatalf("expected suppliers controller to be dynamically registered")
	}

	// 6. Test that newly deployed module's routes work immediately!
	reqNewModule := httptest.NewRequest(http.MethodGet, "/admin/suppliers", nil)
	reqNewModule.AddCookie(cookie)
	recNewModule := httptest.NewRecorder()
	server.Handler().ServeHTTP(recNewModule, reqNewModule)

	if recNewModule.Code != http.StatusOK {
		t.Fatalf("expected status 200 on newly deployed /admin/suppliers, got %d", recNewModule.Code)
	}
	if !strings.Contains(recNewModule.Body.String(), "Suppliers") {
		t.Fatalf("expected HTML to contain Suppliers title")
	}

	// 7. Test newly deployed module's JSON data endpoint
	reqNewData := httptest.NewRequest(http.MethodGet, "/admin/suppliers/data", nil)
	reqNewData.AddCookie(cookie)
	recNewData := httptest.NewRecorder()
	server.Handler().ServeHTTP(recNewData, reqNewData)

	if recNewData.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /admin/suppliers/data, got %d", recNewData.Code)
	}
	if !strings.Contains(recNewData.Body.String(), "Suppliers Sample") {
		t.Fatalf("expected seed sample records in new module: %s", recNewData.Body.String())
	}

	// 8. Test DELETE /admin/module_generator/delete?table=suppliers
	reqDel := httptest.NewRequest(http.MethodPost, "/admin/module_generator/delete?table=suppliers", nil)
	reqDel.AddCookie(cookie)
	recDel := httptest.NewRecorder()
	server.Handler().ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected status 200 on delete, got %d", recDel.Code)
	}
	if engine.FindController("suppliers") != nil {
		t.Fatalf("expected suppliers controller to be unregistered")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestPrivilegeRolesRBACFlow(t *testing.T) {
	engine := cb.NewEngine("Test Admin")
	ctrl := &cb.Controller{
		Title: "Products",
		Table: "products",
		DataProvider: cb.NewMemoryStore(
			map[string]interface{}{"id": "1", "name": "Mechanical Keyboard"},
		),
	}
	ctrl.AddCol("ID", "id", cb.ColText, false, true).
		AddCol("Product Name", "name", cb.ColText, true, true)
	ctrl.AddForm("Product Name", "name", cb.InputText, true, "Product name")
	engine.Register(ctrl)

	server := connect.NewServer()
	engine.Mount(server, "/admin")
	handler := server.Handler()

	// 1. Authenticate as Super Admin
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{ID: "1", Name: "Super Admin", RoleName: "Superadmin", PrivilegeID: "1"})
	adminCookie := recAuth.Result().Cookies()[0]

	// 2. Test GET /admin/privileges (Privileges Management Page)
	reqPriv := httptest.NewRequest(http.MethodGet, "/admin/privileges", nil)
	reqPriv.AddCookie(adminCookie)
	recPriv := httptest.NewRecorder()
	handler.ServeHTTP(recPriv, reqPriv)

	if recPriv.Code != http.StatusOK {
		t.Fatalf("expected status 200 on /admin/privileges, got %d: %s", recPriv.Code, recPriv.Body.String())
	}
	if !strings.Contains(recPriv.Body.String(), "Privileges & Roles Management") {
		t.Fatalf("expected page to contain Privileges & Roles title")
	}
	if !strings.Contains(recPriv.Body.String(), "Super Administrator") {
		t.Fatalf("expected roles list to include Super Administrator")
	}

	// 3. Test GET /admin/privileges/api/role?id=2 (Operations Manager)
	reqRoleAPI := httptest.NewRequest(http.MethodGet, "/admin/privileges/api/role?id=2", nil)
	reqRoleAPI.AddCookie(adminCookie)
	recRoleAPI := httptest.NewRecorder()
	handler.ServeHTTP(recRoleAPI, reqRoleAPI)

	if recRoleAPI.Code != http.StatusOK {
		t.Fatalf("expected status 200 on role api, got %d", recRoleAPI.Code)
	}
	var role2 cb.Role
	if err := json.Unmarshal(recRoleAPI.Body.Bytes(), &role2); err != nil {
		t.Fatalf("failed to decode role json: %v", err)
	}
	if role2.Slug != "ops_manager" {
		t.Fatalf("expected slug ops_manager, got %s", role2.Slug)
	}

	// 4. Test POST /admin/privileges/save (Create new role "Inventory Clerk" with custom permissions)
	newRolePayload := map[string]interface{}{
		"name":          "Inventory Clerk",
		"slug":          "clerk",
		"is_superadmin": false,
		"description":   "Warehouse inventory updates only without create or delete rights",
		"permissions": map[string]cb.PermissionMatrix{
			"products": {
				IsVisible: true,
				CanCreate: false,
				CanRead:   true,
				CanUpdate: true,
				CanDelete: false,
			},
		},
	}
	payloadBytes, _ := json.Marshal(newRolePayload)
	reqSaveRole := httptest.NewRequest(http.MethodPost, "/admin/privileges/save", bytes.NewReader(payloadBytes))
	reqSaveRole.Header.Set("Content-Type", "application/json")
	reqSaveRole.AddCookie(adminCookie)
	recSaveRole := httptest.NewRecorder()
	handler.ServeHTTP(recSaveRole, reqSaveRole)

	if recSaveRole.Code != http.StatusOK {
		t.Fatalf("expected status 200 on save role, got %d: %s", recSaveRole.Code, recSaveRole.Body.String())
	}

	savedRole := engine.FindRole("clerk")
	if savedRole == nil {
		t.Fatalf("expected saved role clerk to exist in engine")
	}

	// 5. Test Live Role Switcher to "clerk"
	reqSwitch := httptest.NewRequest(http.MethodPost, "/admin/privileges/switch?role=clerk", nil)
	reqSwitch.AddCookie(adminCookie)
	recSwitch := httptest.NewRecorder()
	handler.ServeHTTP(recSwitch, reqSwitch)

	if recSwitch.Code != http.StatusOK {
		t.Fatalf("expected status 200 on role switch, got %d: %s", recSwitch.Code, recSwitch.Body.String())
	}
	if len(recSwitch.Result().Cookies()) == 0 {
		t.Fatalf("expected session cookie on role switch")
	}
	clerkCookie := recSwitch.Result().Cookies()[0]

	// 6. Test RBAC Enforcement as "clerk":
	// a) Can READ products -> 200 OK
	reqRead := httptest.NewRequest(http.MethodGet, "/admin/products", nil)
	reqRead.AddCookie(clerkCookie)
	recRead := httptest.NewRecorder()
	handler.ServeHTTP(recRead, reqRead)
	if recRead.Code != http.StatusOK {
		t.Fatalf("expected status 200 on clerk reading products, got %d", recRead.Code)
	}

	// b) Cannot CREATE products -> 403 Forbidden
	reqCreate := httptest.NewRequest(http.MethodPost, "/admin/products/add", strings.NewReader("name=New+Keyboard"))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreate.AddCookie(clerkCookie)
	recCreate := httptest.NewRecorder()
	handler.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 on clerk creating products, got %d", recCreate.Code)
	}

	// c) Cannot DELETE products -> 403 Forbidden
	reqDelete := httptest.NewRequest(http.MethodGet, "/admin/products/delete/1", nil)
	reqDelete.AddCookie(clerkCookie)
	recDelete := httptest.NewRecorder()
	handler.ServeHTTP(recDelete, reqDelete)
	if recDelete.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 on clerk deleting products, got %d", recDelete.Code)
	}

	// 7. Test Deleting Clerk Role & Superadmin protection
	// Superadmin cannot be deleted
	reqDelSuper := httptest.NewRequest(http.MethodPost, "/admin/privileges/delete?id=1", nil)
	reqDelSuper.AddCookie(adminCookie)
	recDelSuper := httptest.NewRecorder()
	handler.ServeHTTP(recDelSuper, reqDelSuper)
	if recDelSuper.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when trying to delete superadmin, got %d", recDelSuper.Code)
	}

	// Clerk can be deleted
	reqDelClerk := httptest.NewRequest(http.MethodPost, "/admin/privileges/delete?id="+savedRole.ID, nil)
	reqDelClerk.AddCookie(adminCookie)
	recDelClerk := httptest.NewRecorder()
	handler.ServeHTTP(recDelClerk, reqDelClerk)
	if recDelClerk.Code != http.StatusOK {
		t.Fatalf("expected 200 on deleting clerk, got %d", recDelClerk.Code)
	}
	if engine.FindRole("clerk") != nil {
		t.Fatalf("expected clerk role to be deleted")
	}
}

func TestMenuManagementHierarchicalFlow(t *testing.T) {
	origMenus, _ := os.ReadFile("data/menus.json")
	t.Cleanup(func() {
		if len(origMenus) > 0 {
			_ = os.WriteFile("data/menus.json", origMenus, 0644)
		}
	})

	// 1. Setup clean environment and engine
	engine := NewEngine("Test Store")
	server := connect.NewServer()

	prodCtrl := &cb.Controller{
		Title: "Products",
		Table: "products",
		Icon:  `<svg width="18" height="18"><path d="M12 2v20"/></svg>`,
		DataProvider: cb.NewMemoryStore(
			map[string]interface{}{"id": "1", "name": "Mechanical Keyboard", "price": 120.0},
		),
	}
	custCtrl := &cb.Controller{
		Title: "Customers",
		Table: "customers",
		Icon:  `<svg width="18" height="18"><circle cx="12" cy="12" r="10"/></svg>`,
		DataProvider: cb.NewMemoryStore(
			map[string]interface{}{"id": "1", "name": "Budi Santoso"},
		),
	}

	engine.Register(prodCtrl)
	engine.Register(custCtrl)
	engine.Mount(server, "/admin")

	handler := server.Handler()

	// Authenticate as Superadmin
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{ID: "1", Name: "Super Admin", RoleName: "Superadmin", PrivilegeID: "1"})
	adminCookie := recAuth.Result().Cookies()[0]

	// 2. Test Access to Menu Management Studio (/admin/menus)
	reqStudio := httptest.NewRequest(http.MethodGet, "/admin/menus", nil)
	reqStudio.AddCookie(adminCookie)
	recStudio := httptest.NewRecorder()
	handler.ServeHTTP(recStudio, reqStudio)

	if recStudio.Code != http.StatusOK {
		t.Fatalf("expected 200 on Menu Studio, got %d", recStudio.Code)
	}
	studioHTML := recStudio.Body.String()
	if !strings.Contains(studioHTML, "Menu Management") || !strings.Contains(studioHTML, "Navigation Hierarchy Tree") {
		t.Fatalf("expected studio html to contain navigation tree headers")
	}

	// 3. Test API GET /admin/menus/api/menu
	reqAPI := httptest.NewRequest(http.MethodGet, "/admin/menus/api/menu?id=products", nil)
	reqAPI.AddCookie(adminCookie)
	recAPI := httptest.NewRecorder()
	handler.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on menu API for products, got %d", recAPI.Code)
	}
	if !strings.Contains(recAPI.Body.String(), "Products") {
		t.Fatalf("expected json response with Products title, got %s", recAPI.Body.String())
	}

	// 4. Test Create Section Group Header
	groupPayload := `{"id":"group_finance","title":"Finance & Billing","type":"header"}`
	reqGroup := httptest.NewRequest(http.MethodPost, "/admin/menus/save", strings.NewReader(groupPayload))
	reqGroup.Header.Set("Content-Type", "application/json")
	reqGroup.AddCookie(adminCookie)
	recGroup := httptest.NewRecorder()
	handler.ServeHTTP(recGroup, reqGroup)

	if recGroup.Code != http.StatusOK {
		t.Fatalf("expected 200 on creating header group, got %d: %s", recGroup.Code, recGroup.Body.String())
	}

	// 5. Test Create Dropdown Folder
	folderPayload := `{"id":"invoicing_folder","title":"Invoicing Center","type":"dropdown","icon":"<svg></svg>","badge":"NEW","badge_color":"warning"}`
	reqFolder := httptest.NewRequest(http.MethodPost, "/admin/menus/save", strings.NewReader(folderPayload))
	reqFolder.Header.Set("Content-Type", "application/json")
	reqFolder.AddCookie(adminCookie)
	recFolder := httptest.NewRecorder()
	handler.ServeHTTP(recFolder, reqFolder)

	if recFolder.Code != http.StatusOK {
		t.Fatalf("expected 200 on creating dropdown folder, got %d: %s", recFolder.Code, recFolder.Body.String())
	}

	// 6. Test Create Sub-menus inside Dropdown Folder
	sub1Payload := `{"id":"invoices_sub","title":"Customer Invoices","type":"url","path":"/admin/invoices","parent_id":"invoicing_folder"}`
	reqSub1 := httptest.NewRequest(http.MethodPost, "/admin/menus/save", strings.NewReader(sub1Payload))
	reqSub1.Header.Set("Content-Type", "application/json")
	reqSub1.AddCookie(adminCookie)
	recSub1 := httptest.NewRecorder()
	handler.ServeHTTP(recSub1, reqSub1)

	if recSub1.Code != http.StatusOK {
		t.Fatalf("expected 200 on adding sub-menu 1, got %d: %s", recSub1.Code, recSub1.Body.String())
	}

	sub2Payload := `{"id":"receipts_sub","title":"Payment Receipts","type":"url","path":"/admin/receipts","parent_id":"invoicing_folder","roles":["ops_manager"]}`
	reqSub2 := httptest.NewRequest(http.MethodPost, "/admin/menus/save", strings.NewReader(sub2Payload))
	reqSub2.Header.Set("Content-Type", "application/json")
	reqSub2.AddCookie(adminCookie)
	recSub2 := httptest.NewRecorder()
	handler.ServeHTTP(recSub2, reqSub2)

	if recSub2.Code != http.StatusOK {
		t.Fatalf("expected 200 on adding sub-menu 2, got %d: %s", recSub2.Code, recSub2.Body.String())
	}

	// Verify dropdown now contains 2 sub-menus in Studio view
	reqVerify := httptest.NewRequest(http.MethodGet, "/admin/menus", nil)
	reqVerify.AddCookie(adminCookie)
	recVerify := httptest.NewRecorder()
	handler.ServeHTTP(recVerify, reqVerify)

	verifyHTML := recVerify.Body.String()
	if !strings.Contains(verifyHTML, "Customer Invoices") || !strings.Contains(verifyHTML, "Payment Receipts") {
		t.Fatalf("expected studio to render sub-menus")
	}

	// 7. Test Reorder Sub-menus (Move Up receipts_sub)
	reqReorder := httptest.NewRequest(http.MethodPost, "/admin/menus/reorder?id=receipts_sub&direction=up", nil)
	reqReorder.AddCookie(adminCookie)
	recReorder := httptest.NewRecorder()
	handler.ServeHTTP(recReorder, reqReorder)

	if recReorder.Code != http.StatusOK {
		t.Fatalf("expected 200 on reorder sub-menu, got %d: %s", recReorder.Code, recReorder.Body.String())
	}

	// 8. Test Delete Sub-menu item
	reqDel := httptest.NewRequest(http.MethodPost, "/admin/menus/delete?id=receipts_sub", nil)
	reqDel.AddCookie(adminCookie)
	recDel := httptest.NewRecorder()
	handler.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete sub-menu, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// 9. Test Reset Menus to Default Configuration
	reqReset := httptest.NewRequest(http.MethodPost, "/admin/menus/reset", nil)
	reqReset.AddCookie(adminCookie)
	recReset := httptest.NewRecorder()
	handler.ServeHTTP(recReset, reqReset)

	if recReset.Code != http.StatusOK {
		t.Fatalf("expected 200 on reset menus, got %d: %s", recReset.Code, recReset.Body.String())
	}

	// Verify that custom group_finance is gone and defaults are restored
	reqPostReset := httptest.NewRequest(http.MethodGet, "/admin/menus", nil)
	reqPostReset.AddCookie(adminCookie)
	recPostReset := httptest.NewRecorder()
	handler.ServeHTTP(recPostReset, reqPostReset)

	postResetHTML := recPostReset.Body.String()
	if strings.Contains(postResetHTML, "group_finance") {
		t.Fatalf("expected group_finance to be cleared after reset")
	}
	if !strings.Contains(postResetHTML, "Platform") || !strings.Contains(postResetHTML, "Business Modules") {
		t.Fatalf("expected default groups to be restored")
	}
}

func TestMenuHeaderChildrenAndDragDropReorder(t *testing.T) {
	origMenus, _ := os.ReadFile("data/menus.json")
	t.Cleanup(func() {
		if len(origMenus) > 0 {
			_ = os.WriteFile("data/menus.json", origMenus, 0644)
		}
	})

	engine := NewEngine("Test Engine")
	server := connect.NewServer()
	engine.Mount(server, "/admin")
	handler := server.Handler()

	// Authenticate superadmin
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{
		ID:          "1",
		Name:        "Super Admin",
		RoleName:    "Superadmin",
		PrivilegeID: "1",
	})
	adminCookie := recAuth.Result().Cookies()[0]

	// 1. Save Dashboard with ParentID="group_platform" (Header)
	dashPayload := map[string]interface{}{
		"id":        "dashboard",
		"title":     "Executive Dashboard",
		"type":      "url",
		"path":      "/admin",
		"parent_id": "group_platform",
		"icon":      "<svg></svg>",
		"order":     1,
		"roles":     []string{},
	}
	dashJSON, _ := json.Marshal(dashPayload)
	reqSaveDash := httptest.NewRequest(http.MethodPost, "/admin/menus/save", bytes.NewReader(dashJSON))
	reqSaveDash.Header.Set("Content-Type", "application/json")
	reqSaveDash.AddCookie(adminCookie)
	recSaveDash := httptest.NewRecorder()
	handler.ServeHTTP(recSaveDash, reqSaveDash)

	if recSaveDash.Code != http.StatusOK {
		t.Fatalf("expected 200 on save dashboard under header, got %d: %s", recSaveDash.Code, recSaveDash.Body.String())
	}

	// 2. Verify that Executive Dashboard renders in Sidebar and Menus Studio under Platform
	reqMenus := httptest.NewRequest(http.MethodGet, "/admin/menus", nil)
	reqMenus.AddCookie(adminCookie)
	recMenus := httptest.NewRecorder()
	handler.ServeHTTP(recMenus, reqMenus)

	menusHTML := recMenus.Body.String()
	if !strings.Contains(menusHTML, "Executive Dashboard") {
		t.Fatalf("expected Executive Dashboard to appear in /admin/menus even when nested under group_platform")
	}

	// 3. Test In-place update without losing order
	updatePayload := map[string]interface{}{
		"id":        "dashboard",
		"title":     "Updated Executive Dashboard",
		"type":      "url",
		"path":      "/admin",
		"parent_id": "group_platform",
		"order":     1,
	}
	updateJSON, _ := json.Marshal(updatePayload)
	reqUpdate := httptest.NewRequest(http.MethodPost, "/admin/menus/save", bytes.NewReader(updateJSON))
	reqUpdate.Header.Set("Content-Type", "application/json")
	reqUpdate.AddCookie(adminCookie)
	recUpdate := httptest.NewRecorder()
	handler.ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 on in-place update, got %d", recUpdate.Code)
	}

	// 4. Test Drag & Drop Reorder Tree API (/admin/menus/reorder_tree)
	reorderTreePayload := map[string]interface{}{
		"tree": []map[string]interface{}{
			{
				"id":        "dashboard",
				"parent_id": "",
				"order":     1,
				"children":  []interface{}{},
			},
			{
				"id":        "group_platform",
				"parent_id": "",
				"order":     2,
				"children":  []interface{}{},
			},
		},
	}
	reorderJSON, _ := json.Marshal(reorderTreePayload)
	reqReorderTree := httptest.NewRequest(http.MethodPost, "/admin/menus/reorder_tree", bytes.NewReader(reorderJSON))
	reqReorderTree.Header.Set("Content-Type", "application/json")
	reqReorderTree.AddCookie(adminCookie)
	recReorderTree := httptest.NewRecorder()
	handler.ServeHTTP(recReorderTree, reqReorderTree)

	if recReorderTree.Code != http.StatusOK {
		t.Fatalf("expected 200 on reorder_tree, got %d: %s", recReorderTree.Code, recReorderTree.Body.String())
	}

	// Verify order updated
	reqPostReorder := httptest.NewRequest(http.MethodGet, "/admin/menus", nil)
	reqPostReorder.AddCookie(adminCookie)
	recPostReorder := httptest.NewRecorder()
	handler.ServeHTTP(recPostReorder, reqPostReorder)

	postReorderHTML := recPostReorder.Body.String()
	if !strings.Contains(postReorderHTML, "Updated Executive Dashboard") {
		t.Fatalf("expected Updated Executive Dashboard to persist after tree reorder")
	}
}

func TestCustomPagesAndSEOPixelFlow(t *testing.T) {
	server := connect.NewServer()
	engine := cb.NewEngine("TGo Enterprise Booster")
	engine.Mount(server)
	handler := server.Handler()

	// 1. Verify Seed Pages are Loaded & Publicly Accessible via Custom Route URLs
	// Test GET /about-us
	reqAbout := httptest.NewRequest(http.MethodGet, "/about-us", nil)
	recAbout := httptest.NewRecorder()
	handler.ServeHTTP(recAbout, reqAbout)

	if recAbout.Code != http.StatusOK {
		t.Fatalf("expected 200 on /about-us, got %d", recAbout.Code)
	}
	bodyAbout := recAbout.Body.String()
	if !strings.Contains(bodyAbout, "<title>About Us | Enterprise Next-Gen Cloud Platform</title>") {
		t.Fatalf("expected SEO title in /about-us HTML, got: %s", bodyAbout)
	}
	if !strings.Contains(bodyAbout, "name=\"description\" content=\"Learn about our company history") {
		t.Fatalf("expected SEO description in /about-us HTML")
	}
	if !strings.Contains(bodyAbout, "property=\"og:image\"") {
		t.Fatalf("expected OpenGraph image in /about-us HTML")
	}

	// 2. Test GET /promo-special (Verification of Marketing Pixels: Meta, GTM, TikTok)
	reqPromo := httptest.NewRequest(http.MethodGet, "/promo-special", nil)
	recPromo := httptest.NewRecorder()
	handler.ServeHTTP(recPromo, reqPromo)

	if recPromo.Code != http.StatusOK {
		t.Fatalf("expected 200 on /promo-special, got %d", recPromo.Code)
	}
	bodyPromo := recPromo.Body.String()
	if !strings.Contains(bodyPromo, "Special Promo 2026: Save 50%") {
		t.Fatalf("expected promo title in HTML")
	}
	// Verify Meta Pixel Injection
	if !strings.Contains(bodyPromo, "fbq('init', '987654321012345')") || !strings.Contains(bodyPromo, "fbq('track', 'PageView')") {
		t.Fatalf("expected Meta Facebook Pixel code to be injected into /promo-special")
	}
	// Verify GTM container injection
	if !strings.Contains(bodyPromo, "GTM-PROMO26") {
		t.Fatalf("expected GTM container ID to be injected into /promo-special")
	}
	// Verify TikTok Pixel Injection
	if !strings.Contains(bodyPromo, "ttq.load('CTIKTOK2026EXAMPLE')") || !strings.Contains(bodyPromo, "ttq.page()") {
		t.Fatalf("expected TikTok Pixel code to be injected into /promo-special")
	}

	// 3. Test Universal Prefix Serving (/p/{slug} and /page/{slug})
	reqSlug := httptest.NewRequest(http.MethodGet, "/p/privacy", nil)
	recSlug := httptest.NewRecorder()
	handler.ServeHTTP(recSlug, reqSlug)

	if recSlug.Code != http.StatusOK {
		t.Fatalf("expected 200 on /p/privacy, got %d", recSlug.Code)
	}
	if !strings.Contains(recSlug.Body.String(), "Privacy Policy &amp; GDPR Compliance") && !strings.Contains(recSlug.Body.String(), "Privacy Policy & GDPR Compliance") {
		t.Fatalf("expected privacy policy content on /p/privacy")
	}

	// 4. Test Admin Studio Access (/admin/pages)
	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{ID: "1", Name: "Super Admin", RoleName: "Super Administrator", PrivilegeID: "1"})
	adminCookie := recAuth.Result().Cookies()[0]

	reqPagesAdmin := httptest.NewRequest(http.MethodGet, "/admin/pages", nil)
	reqPagesAdmin.AddCookie(adminCookie)
	recPagesAdmin := httptest.NewRecorder()
	handler.ServeHTTP(recPagesAdmin, reqPagesAdmin)

	if recPagesAdmin.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/pages, got %d: %s", recPagesAdmin.Code, recPagesAdmin.Body.String())
	}
	bodyAdmin := recPagesAdmin.Body.String()
	if !strings.Contains(bodyAdmin, "Custom Pages & SEO Studio") {
		t.Fatalf("expected Pages Studio title in admin UI")
	}
	if !strings.Contains(bodyAdmin, "/about-us") || !strings.Contains(bodyAdmin, "/promo-special") {
		t.Fatalf("expected table rows with routes in admin UI")
	}

	// 5. Test Fetch Single Page API (/admin/pages/api/page?id=2)
	reqAPI := httptest.NewRequest(http.MethodGet, "/admin/pages/api/page?id=2", nil)
	reqAPI.AddCookie(adminCookie)
	recAPI := httptest.NewRecorder()
	handler.ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on api/page, got %d", recAPI.Code)
	}
	var pageResp cb.Page
	if err := json.NewDecoder(recAPI.Body).Decode(&pageResp); err != nil {
		t.Fatalf("failed to decode page JSON: %v", err)
	}
	if pageResp.Slug != "promo-special" || pageResp.PixelMetaID != "987654321012345" {
		t.Fatalf("unexpected page JSON data: %+v", pageResp)
	}

	// 6. Test Create New Page with Custom Route, SEO, and Pixels via POST /admin/pages/save
	newPagePayload := map[string]interface{}{
		"title":             "Black Friday Exclusive Deal",
		"slug":              "black-friday",
		"route_url":         "/black-friday-2026",
		"template":          "landing",
		"status":            "published",
		"author":            "Growth Hacker",
		"meta_title":        "Black Friday 2026: 80% Discount Platform",
		"meta_description":  "Massive flash sale. Claim instant access now.",
		"canonical_url":     "https://example.com/black-friday-2026",
		"robots":            "index, follow",
		"pixel_meta_id":     "555444333222",
		"pixel_tiktok_id":   "TTK998877",
		"content":           "<div class=\"bf-hero\"><h1>Black Friday Massive Offer</h1></div>",
		"custom_css":        ".bf-hero { background: #000; color: #ff0; }",
	}
	newPageJSON, _ := json.Marshal(newPagePayload)
	reqSave := httptest.NewRequest(http.MethodPost, "/admin/pages/save", bytes.NewReader(newPageJSON))
	reqSave.Header.Set("Content-Type", "application/json")
	reqSave.AddCookie(adminCookie)
	recSave := httptest.NewRecorder()
	handler.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on save page, got %d: %s", recSave.Code, recSave.Body.String())
	}
	var saveResp map[string]interface{}
	_ = json.NewDecoder(recSave.Body).Decode(&saveResp)
	createdID := saveResp["id"].(string)

	// Verify that the new custom route /black-friday-2026 is immediately live!
	reqNewRoute := httptest.NewRequest(http.MethodGet, "/black-friday-2026", nil)
	recNewRoute := httptest.NewRecorder()
	handler.ServeHTTP(recNewRoute, reqNewRoute)

	if recNewRoute.Code != http.StatusOK {
		t.Fatalf("expected 200 on newly registered route /black-friday-2026, got %d", recNewRoute.Code)
	}
	bodyNewRoute := recNewRoute.Body.String()
	if !strings.Contains(bodyNewRoute, "Black Friday 2026: 80% Discount Platform") {
		t.Fatalf("expected new page SEO title in HTML")
	}
	if !strings.Contains(bodyNewRoute, "fbq('init', '555444333222')") {
		t.Fatalf("expected custom Meta Pixel in new page HTML")
	}
	if !strings.Contains(bodyNewRoute, "ttq.load('TTK998877')") {
		t.Fatalf("expected custom TikTok Pixel in new page HTML")
	}
	if !strings.Contains(bodyNewRoute, ".bf-hero { background: #000; color: #ff0; }") {
		t.Fatalf("expected custom CSS injected in new page HTML")
	}

	// 7. Test Draft Page Protection (Public 404 vs Admin Preview)
	draftPayload := map[string]interface{}{
		"title":     "Secret Unreleased Feature",
		"slug":      "secret-feature",
		"route_url": "/secret-feature",
		"status":    "draft",
		"content":   "<h1>Classified</h1>",
	}
	draftJSON, _ := json.Marshal(draftPayload)
	reqDraftSave := httptest.NewRequest(http.MethodPost, "/admin/pages/save", bytes.NewReader(draftJSON))
	reqDraftSave.Header.Set("Content-Type", "application/json")
	reqDraftSave.AddCookie(adminCookie)
	recDraftSave := httptest.NewRecorder()
	handler.ServeHTTP(recDraftSave, reqDraftSave)

	if recDraftSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on draft save, got %d", recDraftSave.Code)
	}

	// Public unauthenticated request to draft page must return 404
	reqPubDraft := httptest.NewRequest(http.MethodGet, "/secret-feature", nil)
	recPubDraft := httptest.NewRecorder()
	handler.ServeHTTP(recPubDraft, reqPubDraft)
	if recPubDraft.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unauthenticated visitor on draft page, got %d", recPubDraft.Code)
	}

	// Authenticated admin request to draft page must return 200 with Preview Notice banner
	reqAdminDraft := httptest.NewRequest(http.MethodGet, "/secret-feature", nil)
	reqAdminDraft.AddCookie(adminCookie)
	recAdminDraft := httptest.NewRecorder()
	handler.ServeHTTP(recAdminDraft, reqAdminDraft)
	if recAdminDraft.Code != http.StatusOK {
		t.Fatalf("expected 200 for authenticated admin on draft page, got %d", recAdminDraft.Code)
	}
	if !strings.Contains(recAdminDraft.Body.String(), "PREVIEW MODE") {
		t.Fatalf("expected preview banner on draft page for admin view")
	}

	// 8. Test Delete Page via POST /admin/pages/delete
	delPayload := map[string]interface{}{"id": createdID}
	delJSON, _ := json.Marshal(delPayload)
	reqDel := httptest.NewRequest(http.MethodPost, "/admin/pages/delete", bytes.NewReader(delJSON))
	reqDel.Header.Set("Content-Type", "application/json")
	reqDel.AddCookie(adminCookie)
	recDel := httptest.NewRecorder()
	handler.ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete page, got %d", recDel.Code)
	}
	if engine.FindPage(createdID) != nil {
		t.Fatalf("expected page %s to be deleted from engine", createdID)
	}

	// 9. Test Creating and Serving Root Homepage (Route /)
	homePayload := map[string]interface{}{
		"title":     "Selamat Datang di Halaman Utama",
		"slug":      "home",
		"route_url": "/",
		"status":    "published",
		"content":   "<h1>Ini adalah Halaman Utama Kami</h1>",
	}
	homeJSON, _ := json.Marshal(homePayload)
	reqHomeSave := httptest.NewRequest(http.MethodPost, "/admin/pages/save", bytes.NewReader(homeJSON))
	reqHomeSave.Header.Set("Content-Type", "application/json")
	reqHomeSave.AddCookie(adminCookie)
	recHomeSave := httptest.NewRecorder()
	handler.ServeHTTP(recHomeSave, reqHomeSave)

	if recHomeSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on homepage save, got %d", recHomeSave.Code)
	}

	// Verify that accessing root / directly serves the custom homepage!
	reqHomeGet := httptest.NewRequest(http.MethodGet, "/", nil)
	recHomeGet := httptest.NewRecorder()
	handler.ServeHTTP(recHomeGet, reqHomeGet)

	if recHomeGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on root / homepage, got %d", recHomeGet.Code)
	}
	if !strings.Contains(recHomeGet.Body.String(), "Ini adalah Halaman Utama Kami") {
		t.Fatalf("expected custom homepage content on /")
	}
}

func TestPlatformSettingsStudioFlow(t *testing.T) {
	server := connect.NewServer()
	engine := cb.NewEngine("TGo Booster Settings Test")
	engine.Mount(server)
	handler := server.Handler()

	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{
		ID:          "admin-root",
		Email:       "root@internal.net",
		Name:        "Root Superadmin",
		PrivilegeID: "superadmin",
		RoleName:    "Super Administrator",
	})
	adminCookie := recAuth.Result().Cookies()[0]

	// 1. GET /admin/settings should render Settings Studio
	reqStudio := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	reqStudio.AddCookie(adminCookie)
	recStudio := httptest.NewRecorder()
	handler.ServeHTTP(recStudio, reqStudio)

	if recStudio.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/settings, got %d", recStudio.Code)
	}
	bodyStudio := recStudio.Body.String()
	if !strings.Contains(bodyStudio, "Platform Settings Studio") {
		t.Fatalf("expected Settings Studio title in response HTML")
	}
	if !strings.Contains(bodyStudio, "General & Branding") || !strings.Contains(bodyStudio, "System Diagnostics") {
		t.Fatalf("expected settings tabs in response HTML")
	}

	// 2. GET /admin/settings/api/system-info
	reqSys := httptest.NewRequest(http.MethodGet, "/admin/settings/api/system-info", nil)
	reqSys.AddCookie(adminCookie)
	recSys := httptest.NewRecorder()
	handler.ServeHTTP(recSys, reqSys)

	if recSys.Code != http.StatusOK {
		t.Fatalf("expected 200 on system-info API, got %d", recSys.Code)
	}
	var diag cb.SystemDiagnostics
	if err := json.NewDecoder(recSys.Body).Decode(&diag); err != nil {
		t.Fatalf("failed to decode system-info JSON: %v", err)
	}
	if diag.GoVersion == "" || diag.OS == "" || diag.NumCPU <= 0 {
		t.Fatalf("invalid diagnostics data returned: %+v", diag)
	}

	// 3. POST /admin/settings/save to update app settings
	savePayload := cb.AppSettings{
		AppName:               "Acme Global Cloud",
		AppTagline:            "Enterprise Cloud Orchestration",
		CompanyName:           "Acme Industries International",
		CopyrightText:         "© 2026 Acme Global Corp",
		ThemeMode:             "dark",
		ThemeSkin:             "emerald",
		AccentColor:           "#10b981",
		SidebarStyle:          "compact",
		SessionTimeoutMinutes: 60,
		MaxLoginAttempts:      3,
		DefaultPageSize:       50,
		SMTPHost:              "smtp.enterprise.internal",
		SMTPPort:              587,
		MailDriver:            "log",
		MaintenanceMode:       false,
		DebugMode:             true,
	}
	saveJSON, _ := json.Marshal(savePayload)
	reqSave := httptest.NewRequest(http.MethodPost, "/admin/settings/save", bytes.NewReader(saveJSON))
	reqSave.Header.Set("Content-Type", "application/json")
	reqSave.AddCookie(adminCookie)
	recSave := httptest.NewRecorder()
	handler.ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusOK {
		t.Fatalf("expected 200 on settings save, got %d: %s", recSave.Code, recSave.Body.String())
	}
	if engine.AppName != "Acme Global Cloud" {
		t.Fatalf("expected engine.AppName to be updated live to 'Acme Global Cloud', got %s", engine.AppName)
	}

	// Verify persistence in GetSettings
	curSettings := engine.GetSettings()
	if curSettings.ThemeSkin != "emerald" || curSettings.AccentColor != "#10b981" || curSettings.DefaultPageSize != 50 {
		t.Fatalf("settings not persisted in engine: %+v", curSettings)
	}

	// 4. POST /admin/settings/test-mail
	mailPayload := map[string]string{"target_email": "infra-ops@acme.com"}
	mailJSON, _ := json.Marshal(mailPayload)
	reqMail := httptest.NewRequest(http.MethodPost, "/admin/settings/test-mail", bytes.NewReader(mailJSON))
	reqMail.Header.Set("Content-Type", "application/json")
	reqMail.AddCookie(adminCookie)
	recMail := httptest.NewRecorder()
	handler.ServeHTTP(recMail, reqMail)

	if recMail.Code != http.StatusOK {
		t.Fatalf("expected 200 on test-mail, got %d: %s", recMail.Code, recMail.Body.String())
	}

	// 5. POST /admin/settings/clear-cache
	reqCache := httptest.NewRequest(http.MethodPost, "/admin/settings/clear-cache", nil)
	reqCache.AddCookie(adminCookie)
	recCache := httptest.NewRecorder()
	handler.ServeHTTP(recCache, reqCache)

	if recCache.Code != http.StatusOK {
		t.Fatalf("expected 200 on clear-cache, got %d: %s", recCache.Code, recCache.Body.String())
	}

	// 6. POST /admin/settings/reset
	reqReset := httptest.NewRequest(http.MethodPost, "/admin/settings/reset", nil)
	reqReset.AddCookie(adminCookie)
	recReset := httptest.NewRecorder()
	handler.ServeHTTP(recReset, reqReset)

	if recReset.Code != http.StatusOK {
		t.Fatalf("expected 200 on reset, got %d: %s", recReset.Code, recReset.Body.String())
	}
}

func TestAPIGeneratorAndPublicRESTFlow(t *testing.T) {
	server := connect.NewServer()
	engine := cb.NewEngine("TGo Booster API Studio Test")

	// Register a test products controller with MemoryStore
	productStore := cb.NewMemoryStore(
		map[string]interface{}{"id": "1", "name": "ThinkPad P16", "price": 25000000},
		map[string]interface{}{"id": "2", "name": "MacBook Pro M3", "price": 32000000},
	)
	pCtrl := &cb.Controller{
		Title:        "Products",
		Table:        "products",
		DataProvider: productStore,
	}
	engine.Register(pCtrl)
	engine.Mount(server)
	handler := server.Handler()

	recAuth := httptest.NewRecorder()
	engine.Auth.SetSessionUser(recAuth, &cb.User{
		ID:          "admin-root",
		Email:       "root@internal.net",
		Name:        "Root Superadmin",
		PrivilegeID: "superadmin",
		RoleName:    "Super Administrator",
	})
	adminCookie := recAuth.Result().Cookies()[0]

	// 1. GET /admin/api_generator should render Studio
	reqStudio := httptest.NewRequest(http.MethodGet, "/admin/api_generator", nil)
	reqStudio.AddCookie(adminCookie)
	recStudio := httptest.NewRecorder()
	handler.ServeHTTP(recStudio, reqStudio)

	if recStudio.Code != http.StatusOK {
		t.Fatalf("expected 200 on /admin/api_generator, got %d", recStudio.Code)
	}
	bodyStudio := recStudio.Body.String()
	if !strings.Contains(bodyStudio, "API &amp; Tokens Studio") && !strings.Contains(bodyStudio, "API & Tokens Studio") {
		t.Fatalf("expected API & Tokens Studio title in response HTML")
	}
	if !strings.Contains(bodyStudio, "Access Tokens &amp; Keys") && !strings.Contains(bodyStudio, "Access Tokens & Keys") {
		t.Fatalf("expected tokens tab in response HTML")
	}

	// 2. GET /admin/api_generator/openapi.json
	reqOpenAPI := httptest.NewRequest(http.MethodGet, "/admin/api_generator/openapi.json", nil)
	reqOpenAPI.AddCookie(adminCookie)
	recOpenAPI := httptest.NewRecorder()
	handler.ServeHTTP(recOpenAPI, reqOpenAPI)

	if recOpenAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on openapi.json, got %d", recOpenAPI.Code)
	}
	var spec map[string]interface{}
	if err := json.NewDecoder(recOpenAPI.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to decode OpenAPI JSON: %v", err)
	}
	if spec["openapi"] != "3.0.0" {
		t.Fatalf("expected openapi 3.0.0, got %v", spec["openapi"])
	}

	// 3. Test Public Health Check Endpoint (No Auth Needed)
	reqHealth := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recHealth := httptest.NewRecorder()
	handler.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/health, got %d", recHealth.Code)
	}

	// 4. POST /admin/api_generator/save to create a new token with read:products scope
	createTokPayload := map[string]interface{}{
		"name":        "Mobile Storefront App",
		"environment": "production",
		"rate_limit":  120,
		"scopes":      []string{"read:products"},
	}
	createTokJSON, _ := json.Marshal(createTokPayload)
	reqCreateTok := httptest.NewRequest(http.MethodPost, "/admin/api_generator/save", bytes.NewReader(createTokJSON))
	reqCreateTok.Header.Set("Content-Type", "application/json")
	reqCreateTok.AddCookie(adminCookie)
	recCreateTok := httptest.NewRecorder()
	handler.ServeHTTP(recCreateTok, reqCreateTok)

	if recCreateTok.Code != http.StatusOK {
		t.Fatalf("expected 200 on save token, got %d: %s", recCreateTok.Code, recCreateTok.Body.String())
	}
	var tokResp map[string]interface{}
	_ = json.NewDecoder(recCreateTok.Body).Decode(&tokResp)
	createdTokID := tokResp["id"].(string)
	createdTokenSecret := tokResp["token"].(string)

	if !strings.HasPrefix(createdTokenSecret, "tgo_live_") {
		t.Fatalf("expected generated token to have 'tgo_live_' prefix, got %s", createdTokenSecret)
	}

	// 5. Test Live Public REST Query with the newly created token
	reqPublicAPI := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	reqPublicAPI.Header.Set("Authorization", "Bearer "+createdTokenSecret)
	recPublicAPI := httptest.NewRecorder()
	handler.ServeHTTP(recPublicAPI, reqPublicAPI)

	if recPublicAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/products with valid token, got %d: %s", recPublicAPI.Code, recPublicAPI.Body.String())
	}
	var apiData map[string]interface{}
	_ = json.NewDecoder(recPublicAPI.Body).Decode(&apiData)
	if apiData["success"] != true {
		t.Fatalf("expected success: true in API response")
	}

	// 6. Test Scope Enforcement (Attempt write with read-only token)
	postBody := bytes.NewReader([]byte(`{"name":"ROG Strix","price":40000000}`))
	reqForbidden := httptest.NewRequest(http.MethodPost, "/api/v1/products", postBody)
	reqForbidden.Header.Set("Authorization", "Bearer "+createdTokenSecret)
	reqForbidden.Header.Set("Content-Type", "application/json")
	recForbidden := httptest.NewRecorder()
	handler.ServeHTTP(recForbidden, reqForbidden)

	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden when writing with read-only token, got %d", recForbidden.Code)
	}

	// 7. Test In-Browser Test Request Proxy (/admin/api_generator/test-request)
	proxyPayload := map[string]interface{}{
		"method":  "GET",
		"path":    "/api/v1/products",
		"token":   createdTokenSecret,
		"payload": "",
	}
	proxyJSON, _ := json.Marshal(proxyPayload)
	reqProxy := httptest.NewRequest(http.MethodPost, "/admin/api_generator/test-request", bytes.NewReader(proxyJSON))
	reqProxy.Header.Set("Content-Type", "application/json")
	reqProxy.AddCookie(adminCookie)
	recProxy := httptest.NewRecorder()
	handler.ServeHTTP(recProxy, reqProxy)

	if recProxy.Code != http.StatusOK {
		t.Fatalf("expected 200 on test-request proxy, got %d", recProxy.Code)
	}

	// 8. Delete / Revoke Token
	delTokPayload := map[string]interface{}{"id": createdTokID}
	delTokJSON, _ := json.Marshal(delTokPayload)
	reqDelTok := httptest.NewRequest(http.MethodPost, "/admin/api_generator/delete", bytes.NewReader(delTokJSON))
	reqDelTok.Header.Set("Content-Type", "application/json")
	reqDelTok.AddCookie(adminCookie)
	recDelTok := httptest.NewRecorder()
	handler.ServeHTTP(recDelTok, reqDelTok)

	if recDelTok.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete token, got %d", recDelTok.Code)
	}

	// Subsequent request with deleted token must fail with 401
	reqAfterDel := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	reqAfterDel.Header.Set("Authorization", "Bearer "+createdTokenSecret)
	recAfterDel := httptest.NewRecorder()
	handler.ServeHTTP(recAfterDel, reqAfterDel)

	if recAfterDel.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after token revoked, got %d", recAfterDel.Code)
	}
}




