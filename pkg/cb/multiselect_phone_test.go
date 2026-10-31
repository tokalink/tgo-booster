package cb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

func TestMultiSelect_DynamicOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("MultiSelect Dynamic Test")
	srv := connect.NewServer()
	e.Mount(srv)

	// 1. Ambil field tenants dari user controller
	userCtrl := e.FindController("users")
	if userCtrl == nil {
		t.Fatal("user controller tidak ditemukan")
	}

	var tenantsField *Field
	for _, f := range userCtrl.Forms {
		if f.Name == "tenants" {
			tenantsField = &f
			break
		}
	}
	if tenantsField == nil {
		t.Fatal("field tenants tidak ditemukan di user controller")
	}
	if tenantsField.Type != InputMultiSelect {
		t.Fatalf("expected field type %q, got %q", InputMultiSelect, tenantsField.Type)
	}
	if tenantsField.OptionsFunc == nil {
		t.Fatal("OptionsFunc harus terdefinisi")
	}

	// 2. Tambah tenant baru secara dinamis
	newTenant := &Tenant{
		Name: "Alpha Toko",
		Slug: "alpha-toko",
	}
	if err := e.TenantManager.Save(newTenant); err != nil {
		t.Fatalf("Save tenant gagal: %v", err)
	}

	// OptionsFunc langsung memuat tenant baru tanpa restart
	opts := tenantsField.OptionsFunc()
	foundNew := false
	for _, opt := range opts {
		if opt.Value == "alpha-toko" {
			foundNew = true
			if !strings.Contains(opt.Label, "Alpha Toko") || !strings.Contains(opt.Label, "alpha-toko") {
				t.Errorf("label opsi tidak memuat nama dan slug: %q", opt.Label)
			}
		}
	}
	if !foundNew {
		t.Error("tenant baru 'alpha-toko' tidak ditemukan di OptionsFunc dinamis")
	}

	// Pastikan urut nama (alpha-toko harus sebelum demo-enterprise)
	var alphaIdx, demoIdx int = -1, -1
	for idx, opt := range opts {
		if opt.Value == "alpha-toko" {
			alphaIdx = idx
		}
		if opt.Value == "demo-enterprise" {
			demoIdx = idx
		}
	}
	if alphaIdx == -1 || demoIdx == -1 || alphaIdx > demoIdx {
		t.Errorf("opsi tidak urut nama: alphaIdx=%d, demoIdx=%d", alphaIdx, demoIdx)
	}

	// 3. Render untuk superadmin vs non-superadmin
	// Sesi Superadmin
	superUser := &User{ID: "1", RoleName: "Superadmin", PrivilegeID: "1"}
	reqSuper := httptest.NewRequest(http.MethodGet, "/admin/users/add", nil)
	recSuper := httptest.NewRecorder()
	e.Auth.SetSessionUser(recSuper, superUser)
	for _, c := range recSuper.Result().Cookies() {
		reqSuper.AddCookie(c)
	}

	resolvedSuper := userCtrl.resolveDynamicFormFields(reqSuper, userCtrl.Forms)
	var superTenantsField *Field
	for _, f := range resolvedSuper {
		if f.Name == "tenants" {
			superTenantsField = &f
			break
		}
	}
	hasStarSuper := false
	for _, opt := range superTenantsField.Options {
		if opt.Value == "*" {
			hasStarSuper = true
			break
		}
	}
	if !hasStarSuper {
		t.Error("opsi '*' harus muncul bila dibuka oleh superadmin")
	}

	// Sesi Non-Superadmin
	opsUser := &User{ID: "2", RoleName: "Operations Manager", PrivilegeID: "2"}
	reqOps := httptest.NewRequest(http.MethodGet, "/admin/users/add", nil)
	recOps := httptest.NewRecorder()
	e.Auth.SetSessionUser(recOps, opsUser)
	for _, c := range recOps.Result().Cookies() {
		reqOps.AddCookie(c)
	}

	resolvedOps := userCtrl.resolveDynamicFormFields(reqOps, userCtrl.Forms)
	var opsTenantsField *Field
	for _, f := range resolvedOps {
		if f.Name == "tenants" {
			opsTenantsField = &f
			break
		}
	}
	for _, opt := range opsTenantsField.Options {
		if opt.Value == "*" {
			t.Error("opsi '*' TIDAK boleh muncul bila dibuka oleh non-superadmin")
		}
	}
}

func TestMultiSelect_CheckedValues(t *testing.T) {
	t.Chdir(t.TempDir())

	// Uji template helper isOptionSelected
	if !isOptionSelected("toko-a, toko-b", "toko-a") {
		t.Error("expected 'toko-a' to be selected in string 'toko-a, toko-b'")
	}
	if !isOptionSelected("toko-a, toko-b", "toko-b") {
		t.Error("expected 'toko-b' to be selected in string 'toko-a, toko-b'")
	}
	if isOptionSelected("toko-a, toko-b", "toko-c") {
		t.Error("expected 'toko-c' NOT to be selected")
	}
	if !isOptionSelected([]string{"toko-a", "toko-b"}, "toko-a") {
		t.Error("expected 'toko-a' to be selected in []string")
	}
	if isOptionSelected([]string{"toko-a", "toko-b"}, "toko-c") {
		t.Error("expected 'toko-c' NOT to be selected in []string")
	}

	// Render form dengan string vs []string
	fieldStr := Field{
		Name: "tenants",
		Type: InputMultiSelect,
		Options: []Option{
			{Value: "default", Label: "Default"},
			{Value: "demo-enterprise", Label: "Demo"},
		},
		Value: "default",
	}

	htmlStr, err := RenderFormContent(map[string]interface{}{
		"Title":      "Test",
		"BasePath":   "/admin/users",
		"FormAction": "/admin/users/add",
		"Forms":      []Field{fieldStr},
	})
	if err != nil {
		t.Fatalf("render form string value gagal: %v", err)
	}
	outStr := string(htmlStr)
	if !strings.Contains(outStr, `value="default" checked`) {
		t.Error("checkbox default harus checked untuk nilai string 'default'")
	}
	if strings.Contains(outStr, `value="demo-enterprise" checked`) {
		t.Error("checkbox demo-enterprise tidak boleh checked")
	}

	// Render dengan []string
	fieldSlice := Field{
		Name: "tenants",
		Type: InputMultiSelect,
		Options: []Option{
			{Value: "default", Label: "Default"},
			{Value: "demo-enterprise", Label: "Demo"},
		},
		Value: []string{"default", "demo-enterprise"},
	}
	htmlSlice, err := RenderFormContent(map[string]interface{}{
		"Title":      "Test",
		"BasePath":   "/admin/users",
		"FormAction": "/admin/users/add",
		"Forms":      []Field{fieldSlice},
	})
	if err != nil {
		t.Fatalf("render form slice value gagal: %v", err)
	}
	outSlice := string(htmlSlice)
	if !strings.Contains(outSlice, `value="default" checked`) {
		t.Error("checkbox default harus checked untuk slice")
	}
	if !strings.Contains(outSlice, `value="demo-enterprise" checked`) {
		t.Error("checkbox demo-enterprise harus checked untuk slice")
	}
}

func TestMultiSelect_PostMultipleValues(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("MultiSelect POST Test")
	srv := connect.NewServer()
	e.Mount(srv)

	userCtrl := e.FindController("users")
	if userCtrl == nil {
		t.Fatal("user controller tidak ditemukan")
	}

	// Buat POST form dengan banyak nilai tenants
	formVals := url.Values{}
	formVals.Set("name", "Multi User")
	formVals.Set("email", "multi@tgo.io")
	formVals.Set("password", "SecretPass123!")
	formVals.Set("role", "Superadmin")
	formVals.Set("status", "Active")
	formVals.Add("tenants", "default")
	formVals.Add("tenants", "demo-enterprise")

	req := httptest.NewRequest(http.MethodPost, "/admin/users/add", strings.NewReader(formVals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	userCtrl.handleCreateSubmit(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("handleCreateSubmit code expected %d, got %d (body=%s)", http.StatusSeeOther, rec.Code, rec.Body.String())
	}

	u := e.UserManager.FindByEmail("multi@tgo.io")
	if u == nil {
		t.Fatal("user multi@tgo.io harus tersimpan")
	}
	if len(u.Tenants) != 2 || u.Tenants[0] != "default" || u.Tenants[1] != "demo-enterprise" {
		t.Fatalf("tenants tidak tersimpan dengan benar: %+v", u.Tenants)
	}
}

func TestValidation_TenantSlugNotExists(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Validation Slug Not Exists")
	udp := e.UserManager.NewDataProvider()

	// 1. Superadmin mencoba menambah user dengan slug tak dikenal -> harus galat
	ctxSuper := &Context{
		User: &User{ID: "1", RoleName: "Superadmin", PrivilegeID: "1"},
	}

	err := udp.Create(ctxSuper, map[string]interface{}{
		"name":     "Uji Slug Tak Ada",
		"email":    "badslug@tgo.io",
		"password": "Password123!",
		"role":     "Superadmin",
		"tenants":  []string{"slug-tidak-ada"},
	})
	if err == nil {
		t.Fatal("Create dengan slug tak ada harus galat")
	}
	expectedMsg := "Akun pelanggan slug-tidak-ada tidak ditemukan"
	if err.Error() != expectedMsg {
		t.Fatalf("expected error %q, got %q", expectedMsg, err.Error())
	}

	// 2. Update dengan slug tak dikenal juga harus galat
	acc, err := e.UserManager.CreateUser("Asli", "asli@tgo.io", "Pass123!", "Superadmin", "1", "", "Active")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	err = udp.Update(ctxSuper, acc.ID, map[string]interface{}{
		"tenants": []string{"slug-fiktif"},
	})
	if err == nil {
		t.Fatal("Update dengan slug tak ada harus galat")
	}
	expectedUpdateMsg := "Akun pelanggan slug-fiktif tidak ditemukan"
	if err.Error() != expectedUpdateMsg {
		t.Fatalf("expected error %q, got %q", expectedUpdateMsg, err.Error())
	}
}

func TestValidation_WildcardRoleRestriction(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Wildcard Role Restriction")
	udp := e.UserManager.NewDataProvider()
	ctxSuper := &Context{
		User: &User{ID: "1", RoleName: "Superadmin", PrivilegeID: "1"},
	}

	// 1. Peran non-superadmin dengan '*' -> galat
	err := udp.Create(ctxSuper, map[string]interface{}{
		"name":     "Staf Biasa",
		"email":    "staf@tgo.io",
		"password": "Password123!",
		"role":     "Operations Manager",
		"tenants":  []string{"*"},
	})
	if err == nil {
		t.Fatal("Create non-superadmin dengan '*' harus ditolak")
	}
	expectedMsg := "Akses semua akun (*) hanya untuk peran superadmin"
	if err.Error() != expectedMsg {
		t.Fatalf("expected error %q, got %q", expectedMsg, err.Error())
	}

	// 2. Peran superadmin dengan '*' -> boleh
	err = udp.Create(ctxSuper, map[string]interface{}{
		"name":     "Super Baru",
		"email":    "superbaru@tgo.io",
		"password": "Password123!",
		"role":     "Superadmin",
		"tenants":  []string{"*"},
	})
	if err != nil {
		t.Fatalf("Create superadmin dengan '*' harus diizinkan, dapat galat: %v", err)
	}
	saved := e.UserManager.FindByEmail("superbaru@tgo.io")
	if saved == nil || len(saved.Tenants) != 1 || saved.Tenants[0] != "*" {
		t.Fatalf("tenants tidak tersimpan dengan benar: %+v", saved)
	}

	// 3. Update non-superadmin dengan '*' -> galat
	acc, _ := e.UserManager.CreateUser("Budi", "budi@tgo.io", "Pass123!", "Read-Only Auditor", "3", "", "Active")
	err = udp.Update(ctxSuper, acc.ID, map[string]interface{}{
		"tenants": []string{"*"},
	})
	if err == nil {
		t.Fatal("Update non-superadmin dengan '*' harus ditolak")
	}
	if err.Error() != expectedMsg {
		t.Fatalf("expected error %q, got %q", expectedMsg, err.Error())
	}
}

func TestPhone_NormalizationAndErrors(t *testing.T) {
	// 1. Kosong -> kosong, tanpa galat (opsional)
	res, err := NormalizePhone("")
	if err != nil || res != "" {
		t.Errorf("NormalizePhone(\"\") = (%q, %v), want (\"\", nil)", res, err)
	}
	res, err = NormalizePhone("   ")
	if err != nil || res != "" {
		t.Errorf("NormalizePhone(\"   \") = (%q, %v), want (\"\", nil)", res, err)
	}

	// 2. Normalisasi pembersihan spasi, tanda hubung, titik, kurung, dan awalan '+'
	validCases := map[string]string{
		"+62 812-3456-7890":   "+6281234567890",
		"+62.812.3456.7890":   "+6281234567890",
		"0812-3456-7890":      "081234567890",
		"(021) 12345678":      "02112345678",
		"+62 (021) 1234-5678": "+6202112345678",
		"12345678":            "12345678",        // 8 digit
		"123456789012345":     "123456789012345", // 15 digit
		"+12345678":           "+12345678",
		"+123456789012345":    "+123456789012345",
	}
	for in, expected := range validCases {
		got, err := NormalizePhone(in)
		if err != nil {
			t.Errorf("NormalizePhone(%q) unexpected error: %v", in, err)
		} else if got != expected {
			t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, expected)
		}
	}

	// 3. Galat jika < 8 digit, > 15 digit, atau karakter selain digit
	invalidCases := []string{
		"1234567",           // 7 digit (< 8)
		"+1234567",          // 7 digit
		"1234567890123456",  // 16 digit (> 15)
		"+1234567890123456", // 16 digit (> 15)
		"0812abc3456",       // non-digit
		"+",                 // tanpa digit
		"++62812345678",     // tanda plus ganda
		"0812+345678",       // plus di tengah
	}
	for _, in := range invalidCases {
		got, err := NormalizePhone(in)
		if err == nil {
			t.Errorf("NormalizePhone(%q) expected error, got %q", in, got)
		} else if err.Error() != "Nomor WhatsApp tidak valid" {
			t.Errorf("NormalizePhone(%q) error = %q, want 'Nomor WhatsApp tidak valid'", in, err.Error())
		}
	}
}

func TestPhone_TenantSaveAndRead(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Phone Tenant Test")

	// 1. Setter SetPhone
	err := e.TenantManager.SetPhone("default", "+62 812-3456-7890")
	if err != nil {
		t.Fatalf("SetPhone gagal: %v", err)
	}
	tDefault := e.TenantManager.GetBySlug("default")
	if tDefault == nil || tDefault.Phone != "+6281234567890" {
		t.Fatalf("phone tenant tidak cocok: got %q, want '+6281234567890'", tDefault.Phone)
	}

	// Galat nomor tidak valid
	if err := e.TenantManager.SetPhone("default", "1234"); err == nil {
		t.Fatal("SetPhone dengan nomor tidak valid harus galat")
	}

	// 2. DataProvider Create & Update
	tdp := e.TenantManager.NewDataProvider()
	ctx := &Context{Ctx: context.Background()}

	err = tdp.Create(ctx, map[string]interface{}{
		"name":        "Organisasi Baru",
		"slug":        "org-baru",
		"owner_email": "owner@org.id",
		"phone":       "0812-9999-8888",
	})
	if err != nil {
		t.Fatalf("Create tenant via provider gagal: %v", err)
	}

	tOrg := e.TenantManager.GetBySlug("org-baru")
	if tOrg == nil || tOrg.Phone != "081299998888" {
		t.Fatalf("phone tenant org-baru tidak tersimpan: got %q", tOrg.Phone)
	}

	// FindByID & FindAll menyertakan phone
	row, err := tdp.FindByID(ctx, tOrg.ID)
	if err != nil {
		t.Fatalf("FindByID gagal: %v", err)
	}
	if row["phone"] != "081299998888" {
		t.Errorf("FindByID tidak mengembalikan phone: got %v", row["phone"])
	}

	allRows, err := tdp.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll gagal: %v", err)
	}
	foundPhone := false
	for _, r := range allRows {
		if r["slug"] == "org-baru" && r["phone"] == "081299998888" {
			foundPhone = true
		}
	}
	if !foundPhone {
		t.Error("FindAll tidak memuat phone untuk org-baru")
	}

	// Update phone
	err = tdp.Update(ctx, tOrg.ID, map[string]interface{}{
		"phone": "+62 811-2222-3333",
	})
	if err != nil {
		t.Fatalf("Update phone gagal: %v", err)
	}
	tOrgUpdated := e.TenantManager.GetBySlug("org-baru")
	if tOrgUpdated.Phone != "+6281122223333" {
		t.Errorf("phone update tidak cocok: got %q", tOrgUpdated.Phone)
	}
}

func TestPhone_UserSaveAndRead(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Phone User Test")

	// 1. Setter SetPhone pada akun bawaan
	if err := e.UserManager.SetPhone("admin@tgo.io", "+62 855-6666-7777"); err != nil {
		t.Fatalf("SetPhone: %v", err)
	}
	accReload := e.UserManager.FindByEmail("admin@tgo.io")
	if accReload == nil || accReload.Phone != "+6285566667777" {
		t.Fatalf("phone user tidak tersimpan: got %q", accReload.Phone)
	}

	// 2. DataProvider Create & Update
	udp := e.UserManager.NewDataProvider()
	ctx := &Context{
		User: &User{ID: "1", RoleName: "Superadmin", PrivilegeID: "1"},
	}

	err := udp.Create(ctx, map[string]interface{}{
		"name":     "Staf Phone",
		"email":    "stafphone@tgo.io",
		"password": "Password123!",
		"role":     "Superadmin",
		"phone":    "0899-1234-5678",
		"tenants":  []string{"default"},
	})
	if err != nil {
		t.Fatalf("Create user via provider gagal: %v", err)
	}

	staf := e.UserManager.FindByEmail("stafphone@tgo.io")
	if staf == nil || staf.Phone != "089912345678" {
		t.Fatalf("staf phone tidak tersimpan: got %q", staf.Phone)
	}

	// FindByID & FindAll
	rec, err := udp.FindByID(ctx, staf.ID)
	if err != nil {
		t.Fatalf("FindByID user gagal: %v", err)
	}
	if rec["phone"] != "089912345678" {
		t.Errorf("FindByID user tidak mengembalikan phone: got %v", rec["phone"])
	}

	allUsers, err := udp.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll user gagal: %v", err)
	}
	foundUserPhone := false
	for _, u := range allUsers {
		if u["email"] == "stafphone@tgo.io" && u["phone"] == "089912345678" {
			foundUserPhone = true
		}
	}
	if !foundUserPhone {
		t.Error("FindAll user tidak memuat phone")
	}

	// Update phone
	err = udp.Update(ctx, staf.ID, map[string]interface{}{
		"phone": "+62 877-8888-9999",
	})
	if err != nil {
		t.Fatalf("Update phone gagal: %v", err)
	}
	stafUpdated := e.UserManager.FindByEmail("stafphone@tgo.io")
	if stafUpdated.Phone != "+6287788889999" {
		t.Errorf("staf phone update tidak cocok: got %q", stafUpdated.Phone)
	}
}

func TestPhone_LegacyDocumentCompatibility(t *testing.T) {
	t.Chdir(t.TempDir())

	// Dokumen lama tanpa field phone
	legacyTenantsJSON := `[
		{
			"id": "1",
			"name": "Legacy Org",
			"slug": "legacy-org",
			"plan": "Enterprise",
			"status": "Active",
			"owner_email": "legacy@org.id",
			"max_users": 10,
			"max_storage_mb": 1024,
			"created_at": "2025-01-01 00:00:00",
			"updated_at": "2025-01-01 00:00:00"
		}
	]`
	_ = os.MkdirAll("data", 0755)
	if err := os.WriteFile(filepath.Join("data", "tenants.json"), []byte(legacyTenantsJSON), 0644); err != nil {
		t.Fatalf("write legacy tenants: %v", err)
	}

	legacyUsersJSON := `[
		{
			"id": "1",
			"avatar": "",
			"name": "Legacy Admin",
			"email": "legacy@tgo.io",
			"password_hash": "dummyhash",
			"salt": "dummysalt",
			"role_id": "1",
			"role": "Superadmin",
			"status": "Active",
			"tenants": ["legacy-org"],
			"default_tenant": "legacy-org",
			"created_at": "2025-01-01 00:00:00"
		}
	]`
	if err := os.WriteFile(filepath.Join("data", "users.json"), []byte(legacyUsersJSON), 0644); err != nil {
		t.Fatalf("write legacy users: %v", err)
	}

	e := NewEngine("Legacy Test")
	tOrg := e.TenantManager.GetBySlug("legacy-org")
	if tOrg == nil {
		t.Fatal("legacy-org harus termuat")
	}
	if tOrg.Phone != "" {
		t.Errorf("legacy tenant phone harus string kosong, got %q", tOrg.Phone)
	}

	u := e.UserManager.FindByEmail("legacy@tgo.io")
	if u == nil {
		t.Fatal("legacy user harus termuat")
	}
	if u.Phone != "" {
		t.Errorf("legacy user phone harus string kosong, got %q", u.Phone)
	}

	// Pastikan serialization omitempty tidak memuat phone jika kosong
	tData, err := json.Marshal(tOrg)
	if err != nil {
		t.Fatalf("marshal tenant: %v", err)
	}
	if strings.Contains(string(tData), `"phone"`) {
		t.Errorf("phone kosong seharusnya di-omitempty pada json: %s", string(tData))
	}
}
