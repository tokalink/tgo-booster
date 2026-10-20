package cb

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tokalink/tgo/pkg/transport/connect"
)

func TestWave1_GuestAdminRoutesAndStudioProtection(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Guest Test")
	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()

	// GET /admin/login -> 200
	reqLogin := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	recLogin := httptest.NewRecorder()
	h.ServeHTTP(recLogin, reqLogin)
	if recLogin.Code != http.StatusOK {
		t.Fatalf("GET /admin/login: got %d, want 200", recLogin.Code)
	}

	// Tamu GET halaman admin -> 303 ke /admin/login
	for _, path := range []string{"/admin", "/admin/users", "/admin/settings"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/admin/login") {
			t.Errorf("GET %s tanpa sesi: got %d (Location=%q), want 303 ke /admin/login", path, rec.Code, rec.Header().Get("Location"))
		}
	}

	// Tamu POST endpoint studio -> 401
	postPaths := []string{
		"/admin/api_generator/save",
		"/admin/settings/save",
		"/admin/settings/test-ai",
		"/admin/settings/models",
		"/admin/settings/reset",
		"/admin/settings/clear-cache",
		"/admin/settings/test-mail",
		"/admin/menus/save",
		"/admin/pages/save",
		"/admin/api_generator/test-request",
	}
	for _, path := range postPaths {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("POST %s tanpa sesi: got %d, want 401", path, rec.Code)
		}
	}

	// Tamu GET endpoint JSON studio -> bukan 200
	for _, path := range []string{"/admin/settings/api/system-info", "/admin/api_generator/openapi.json"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("GET %s tanpa sesi tidak boleh 200", path)
		}
	}
}

func TestWave1_NonSuperadminStudioEndpointsForbidden(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 NonSuper Test")
	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()

	recAuth := httptest.NewRecorder()
	e.Auth.SetSessionUser(recAuth, &User{
		ID:          "2",
		Name:        "Budi Operations",
		Email:       "manager@tgo.io",
		PrivilegeID: "2",
		RoleName:    "Operations Manager",
	})
	cookie := recAuth.Result().Cookies()[0]

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admin/settings/test-mail"},
		{http.MethodPost, "/admin/settings/test-ai"},
		{http.MethodPost, "/admin/settings/models"},
		{http.MethodPost, "/admin/settings/clear-cache"},
		{http.MethodPost, "/admin/settings/reset"},
		{http.MethodGet, "/admin/settings/api/system-info"},
		{http.MethodGet, "/admin/api_generator/openapi.json"},
		{http.MethodPost, "/admin/api_generator/test-request"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s sebagai non-superadmin: got %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
}

func TestWave1_SuperadminAIHostAndTestRequestValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Superadmin Validation")
	st := e.GetSettings()
	st.OpenAIHost = "https://ai.sumopod.com"
	st.OpenAIAPIKey = "sk-stored-secret-key"
	_ = e.SaveSettings(&st)
	_ = e.SaveAPIToken(&APIToken{
		Name:        "Test Token",
		Token:       "tgo_live_validtoken123456",
		Environment: "production",
		Scopes:      []string{"*"},
		Status:      "active",
	})

	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()

	recAuth := httptest.NewRecorder()
	e.Auth.SetSessionUser(recAuth, &User{
		ID:          "1",
		Name:        "Super Administrator",
		Email:       "admin@tgo.io",
		PrivilegeID: "1",
		RoleName:    "Superadmin",
	})
	adminCookie := recAuth.Result().Cookies()[0]

	postJSON := func(path string, body map[string]interface{}) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(adminCookie)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	// test-ai & models dengan host lain tanpa api_key -> 400 (tidak memakai kunci tersimpan)
	if rec := postJSON("/admin/settings/test-ai", map[string]interface{}{
		"host":    "https://attacker.example.com",
		"api_key": "",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("test-ai host lain tanpa api_key: got %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
	if rec := postJSON("/admin/settings/models", map[string]interface{}{
		"host":    "https://attacker.example.com",
		"api_key": "",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("models host lain tanpa api_key: got %d, want 400 (%s)", rec.Code, rec.Body.String())
	}

	// test-request tanpa token -> 400
	if rec := postJSON("/admin/api_generator/test-request", map[string]interface{}{
		"method": "GET",
		"path":   "/api/v1/health",
		"token":  "",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("test-request tanpa token: got %d, want 400", rec.Code)
	}

	// test-request dengan path /admin/... -> 400
	if rec := postJSON("/admin/api_generator/test-request", map[string]interface{}{
		"method": "GET",
		"path":   "/admin/users",
		"token":  "tgo_live_validtoken123456",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("test-request path /admin/users: got %d, want 400", rec.Code)
	}

	// test-request dengan method di luar GET/POST/PUT/PATCH/DELETE -> 400
	if rec := postJSON("/admin/api_generator/test-request", map[string]interface{}{
		"method": "TRACE",
		"path":   "/api/v1/health",
		"token":  "tgo_live_validtoken123456",
	}); rec.Code != http.StatusBadRequest {
		t.Errorf("test-request method TRACE: got %d, want 400", rec.Code)
	}
}

func TestWave1_NilIsNotSuperadmin(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Nil Check")
	if e.IsUserSuperadmin(nil) {
		t.Fatal("IsUserSuperadmin(nil) harus false")
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	if e.CheckPermission(req, "users", "read") {
		t.Fatal("CheckPermission tanpa sesi (ketika Auth != nil) harus false")
	}
	e.Auth = nil
	if !e.CheckPermission(req, "users", "read") {
		t.Fatal("CheckPermission ketika Auth == nil harus tetap true")
	}
}

func TestWave1_UsersModuleEscalationPrevention(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Users")
	um := e.UserManager

	// CreateUser dengan role kosong -> error
	if _, err := um.CreateUser("NoRole", "norole@uji.id", "Sandi#12345", "", "", "", "Active"); err == nil {
		t.Fatal("CreateUser dengan role kosong harus mengembalikan error")
	}

	udp := um.NewDataProvider()
	opsActor := &User{
		ID:          "2",
		Name:        "Budi Ops",
		Email:       "manager@tgo.io",
		PrivilegeID: "2",
		RoleName:    "Operations Manager",
		Tenants:     []string{"toko-a"},
	}
	ctx := &Context{User: opsActor}

	// 1. Non-superadmin tidak bisa membuat akun Superadmin atau role tak dikenal
	for _, forbiddenRole := range []string{"Superadmin", "Super Administrator", "1", "RoleHantu", ""} {
		err := udp.Create(ctx, map[string]interface{}{
			"name":     "Target",
			"email":    "target-" + forbiddenRole + "@uji.id",
			"password": "Sandi#12345",
			"role":     forbiddenRole,
			"tenants":  "toko-a",
		})
		if err == nil {
			t.Errorf("non-superadmin tidak boleh membuat akun dengan role %q", forbiddenRole)
		}
	}

	// 2. Non-superadmin tidak bisa memberi tenant di luar miliknya atau "*"
	for _, badTenants := range []string{"toko-b", "toko-a, toko-b", "*"} {
		err := udp.Create(ctx, map[string]interface{}{
			"name":     "Target Tenant",
			"email":    "tenant-bad@uji.id",
			"password": "Sandi#12345",
			"role":     "Read-Only Auditor",
			"tenants":  badTenants,
		})
		if err == nil {
			t.Errorf("non-superadmin tidak boleh memberi tenants %q", badTenants)
		}
	}

	// Bila ctx.User.Tenants berisi "*" tetapi bukan superadmin, tetap tidak boleh memberi "*"
	opsWildcard := &User{
		ID:          "2",
		Name:        "Budi Ops",
		Email:       "manager@tgo.io",
		PrivilegeID: "2",
		RoleName:    "Operations Manager",
		Tenants:     []string{"*", "toko-a"},
	}
	if err := udp.Create(&Context{User: opsWildcard}, map[string]interface{}{
		"name":     "Wildcard Target",
		"email":    "wildcard@uji.id",
		"password": "Sandi#12345",
		"role":     "Read-Only Auditor",
		"tenants":  "*",
	}); err == nil {
		t.Error("non-superadmin dengan '*' di Tenants tetap tidak boleh memberi '*'")
	}

	// 3. Pembuatan yang sah oleh non-superadmin
	if err := udp.Create(ctx, map[string]interface{}{
		"name":     "Staf Toko A",
		"email":    "staf-a@uji.id",
		"password": "Sandi#12345",
		"role":     "Read-Only Auditor",
		"tenants":  "toko-a",
	}); err != nil {
		t.Fatalf("pembuatan staf sah gagal: %v", err)
	}
	staf := um.FindByEmail("staf-a@uji.id")
	if staf == nil {
		t.Fatal("staf-a@uji.id harus ditemukan")
	}

	// 4. Non-superadmin tidak bisa mengubah akun Superadmin (ID "1")
	if err := udp.Update(ctx, "1", map[string]interface{}{
		"name": "Diambil Alih",
		"role": "Read-Only Auditor",
	}); err == nil {
		t.Error("non-superadmin tidak boleh mengubah akun superadmin")
	}

	// 5. Non-superadmin tidak bisa menaikkan akun biasa menjadi Superadmin atau memberi tenant lain
	if err := udp.Update(ctx, staf.ID, map[string]interface{}{
		"role": "Super Administrator",
	}); err == nil {
		t.Error("non-superadmin tidak boleh mengubah role akun biasa menjadi Superadmin")
	}
	if err := udp.Update(ctx, staf.ID, map[string]interface{}{
		"tenants": "toko-b",
	}); err == nil {
		t.Error("non-superadmin tidak boleh memberi tenant di luar miliknya saat Update")
	}
	if err := udp.Update(ctx, staf.ID, map[string]interface{}{
		"tenants": "*",
	}); err == nil {
		t.Error("non-superadmin tidak boleh memberi tenant '*' saat Update")
	}
}

func TestWave1_SessionTakesRoleFromAccountAndRejectsSuspended(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Session")
	acc, err := e.UserManager.CreateUser("Staf Biasa", "biasa@uji.id", "Sandi#12345", "Read-Only Auditor", "3", "", "Active")
	if err != nil {
		t.Fatal(err)
	}

	// Buat cookie bertanda tangan sah namun klaim RoleName:"Superadmin" dan PrivilegeID:"1"
	req := sessionReq(e, &User{
		ID:          acc.ID,
		Name:        "Fake Super",
		Email:       "fake@uji.id",
		PrivilegeID: "1",
		RoleName:    "Superadmin",
	})
	got := e.Auth.GetSessionUser(req)
	if got == nil {
		t.Fatal("sesi akun aktif harus diterima")
	}
	if got.RoleName != "Read-Only Auditor" || got.PrivilegeID != "3" || got.Email != "biasa@uji.id" || got.Name != "Staf Biasa" {
		t.Fatalf("identitas/role harus diambil dari akun, dapat: %+v", got)
	}
	if e.IsUserSuperadmin(got) {
		t.Fatal("akun ber-role biasa tidak boleh dianggap superadmin meskipun cookie mengklaim Superadmin")
	}

	// Tangguhkan akun -> selalu ditolak meski StrictSessions false
	e.Auth.StrictSessions = false
	if err := e.UserManager.UpdateUser(acc.ID, "", "", "", "", "", "", "Suspended"); err != nil {
		t.Fatal(err)
	}
	if got := e.Auth.GetSessionUser(req); got != nil {
		t.Fatalf("akun Suspended harus selalu ditolak, dapat: %+v", got)
	}
}

func TestWave1_SQLStoreCreateForcesSessionTenant(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewSQLStore(db, "items", "id")
	store.SetTenantScoped(true)
	if err := store.AutoMigrate(Field{Name: "title", Type: InputText}); err != nil {
		t.Fatal(err)
	}

	ctxSesi := &Context{TenantSlug: "tenant-sesi"}
	payload := map[string]interface{}{
		"title":     "Item Rahasia",
		"tenant_id": "tenant-lain",
	}
	if err := store.Create(ctxSesi, payload); err != nil {
		t.Fatal(err)
	}

	rowsSesi, err := store.FindAll(ctxSesi)
	if err != nil || len(rowsSesi) != 1 {
		t.Fatalf("expected 1 row in tenant-sesi, got %d (err: %v)", len(rowsSesi), err)
	}
	if rowsSesi[0]["tenant_id"] != "tenant-sesi" {
		t.Fatalf("expected tenant_id 'tenant-sesi', got %v", rowsSesi[0]["tenant_id"])
	}

	rowsLain, err := store.FindAll(&Context{TenantSlug: "tenant-lain"})
	if err != nil || len(rowsLain) != 0 {
		t.Fatalf("expected 0 rows in tenant-lain, got %d", len(rowsLain))
	}
}

func TestWave1_MountSessionSecretAndEmptyTokens(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Cleanup(func() {
		sessionSecretMu.Lock()
		sessionSecretCustom = false
		hmacSessionSecret = []byte("tgo-booster-dev-only-session-secret-change-me")
		sessionSecretMu.Unlock()
	})

	// 1. Pemuatan token dari berkas kosong / [] tidak error
	e := NewEngine("Wave1 Secret")
	if toks := e.LoadDynamicTokens(); len(toks) != 0 {
		t.Fatalf("expected 0 tokens on fresh engine, got %d", len(toks))
	}

	// 2. APP_ENV=production dengan secret bawaan: Mount tidak panic, tetapi sesi tidak dibuat/diterima
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_SESSION_SECRET", "")
	e.Mount(connect.NewServer())
	if req := sessionReq(e, &User{ID: "1"}); len(req.Cookies()) != 0 {
		t.Fatal("di production dengan secret bawaan, cookie sesi tidak boleh dibuat")
	}
	if verifySessionSignatureRaw("x", signSessionPayload("x")) {
		t.Fatal("di production dengan secret bawaan, tanda tangan sesi tidak boleh diterima")
	}
	// Aplikasi memasang rahasia sesudah Mount -> sesi berfungsi
	if err := SetSessionSecret([]byte(strings.Repeat("k", 40))); err != nil {
		t.Fatal(err)
	}
	if got := e.Auth.GetSessionUser(sessionReq(e, &User{ID: "1"})); got == nil {
		t.Fatal("setelah SetSessionSecret sesudah Mount, sesi harus berfungsi")
	}
	sessionSecretMu.Lock()
	sessionSecretCustom = false
	sessionSecretMu.Unlock()

	// 3. APP_SESSION_SECRET >= 32 byte -> dipasang otomatis saat Mount
	t.Setenv("APP_SESSION_SECRET", strings.Repeat("s", 36))
	e.Mount(connect.NewServer())
	if SessionSecretIsDefault() {
		t.Fatal("APP_SESSION_SECRET >= 32 byte harus mengganti secret bawaan saat Mount")
	}
}

// Pengguna non-superadmin hanya boleh mengelola akun yang seluruh tenant-nya miliknya; role kosong saat Update = tetap.
func TestWave1_UsersCrossTenantManagementDenied(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Users Lintas Tenant")
	um := e.UserManager
	udp := um.NewDataProvider()

	lain, err := um.CreateUser("Staf B", "staf-b@uji.id", "Sandi#12345", "Read-Only Auditor", "3", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if err := um.SetTenants(lain.ID, []string{"toko-a", "toko-b"}, "toko-a"); err != nil {
		t.Fatal(err)
	}
	sendiri, err := um.CreateUser("Staf A", "staf-a2@uji.id", "Sandi#12345", "Read-Only Auditor", "3", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if err := um.SetTenants(sendiri.ID, []string{"toko-a"}, "toko-a"); err != nil {
		t.Fatal(err)
	}
	ctx := &Context{User: &User{ID: "2", PrivilegeID: "2", RoleName: "Operations Manager", Tenants: []string{"toko-a"}}}

	if err := udp.Update(ctx, lain.ID, map[string]interface{}{"password": "Diambil#Alih1"}); err == nil {
		t.Error("tidak boleh mengubah akun yang juga punya akses ke tenant lain")
	}
	if err := udp.Delete(ctx, lain.ID); err == nil {
		t.Error("tidak boleh menghapus akun yang juga punya akses ke tenant lain")
	}
	if err := udp.Delete(ctx, "1"); err == nil {
		t.Error("tidak boleh menghapus akun superadmin")
	}
	if err := udp.Update(ctx, sendiri.ID, map[string]interface{}{"name": "Staf A Baru", "role": ""}); err != nil {
		t.Fatalf("ubah nama akun se-tenant (role kosong = tetap) harus boleh: %v", err)
	}
	if got := um.FindByID(sendiri.ID); got == nil || got.Role != "Read-Only Auditor" || got.Name != "Staf A Baru" {
		t.Fatalf("role harus tetap & nama berubah, dapat %+v", got)
	}
}

// Pratinjau peran (Privileges › switch) hanya berlaku untuk akun superadmin asli; klaim cookie akun biasa diabaikan.
func TestWave1_SwitchedRoleOnlyForRealSuperadmin(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wave1 Switch")
	e.Auth.StrictSessions = true

	super := e.UserManager.FindByID("1")
	if super == nil {
		t.Fatal("akun superadmin bawaan tidak ada")
	}
	req := sessionReq(e, &User{ID: super.ID, SwitchedRole: "3"})
	got := e.Auth.GetSessionUser(req)
	if got == nil || got.PrivilegeID != "3" || e.IsUserSuperadmin(got) {
		t.Fatalf("superadmin yang beralih ke peran 3 harus berperan 3 (sesi tetap sah), dapat %+v", got)
	}

	biasa, err := e.UserManager.CreateUser("Biasa", "biasa-sw@uji.id", "Sandi#12345", "Read-Only Auditor", "3", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	got = e.Auth.GetSessionUser(sessionReq(e, &User{ID: biasa.ID, SwitchedRole: "1"}))
	if got == nil || e.IsUserSuperadmin(got) || got.SwitchedRole != "" {
		t.Fatalf("akun biasa tidak boleh memakai SwitchedRole, dapat %+v", got)
	}
}
