package cb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateUsername(t *testing.T) {
	ok := map[string]string{"Budi.Santoso": "budi.santoso", "  kasir_01 ": "kasir_01", "a-b": "", "abc": "abc", "toko-maju": "toko-maju"}
	for in, want := range ok {
		got, err := ValidateUsername(in)
		if in == "a-b" { // 3 karakter: valid
			want = "a-b"
		}
		if err != nil || got != want {
			t.Errorf("%q → %q,%v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ab", "a@b.com", "-abc", ".abc", "abc.", "a..b", "ada spasi", "Admin", "ROOT", "kriva", "x" + strings.Repeat("y", 32), "büdi"} {
		if _, err := ValidateUsername(bad); err == nil {
			t.Errorf("%q harus ditolak", bad)
		}
	}
}

func TestUsername_UniqueAndLogin(t *testing.T) {
	e := newAuthEngine(t)
	um := e.UserManager
	a, _ := um.CreateUser("Ani", "ani@x.id", "Rahasia123", "Owner", "4", "", "Active")
	b, _ := um.CreateUser("Budi", "budi@x.id", "Rahasia123", "Owner", "4", "", "Active")
	if _, err := um.SetUsername(a.ID, "Ani.Toko"); err != nil {
		t.Fatal(err)
	}
	if _, err := um.SetUsername(b.ID, "ani.toko"); err == nil {
		t.Fatal("username sama (tanpa peduli huruf) harus ditolak")
	}
	if !um.UsernameTaken("ANI.TOKO", b.ID) || um.UsernameTaken("ani.toko", a.ID) {
		t.Fatal("UsernameTaken salah")
	}
	if _, err := um.SetUsername(b.ID, "admin"); err == nil {
		t.Fatal("nama cadangan harus ditolak")
	}
	if um.FindByLogin("ani.toko").Email != "ani@x.id" || um.FindByLogin("ANI@X.ID").Username != "ani.toko" || um.FindByLogin("tidak.ada") != nil {
		t.Fatal("FindByLogin")
	}
	// masuk dengan username maupun email; pesan galat seragam untuk akun tak ada dan sandi salah
	if _, err := um.Authenticate("ani.toko", "Rahasia123", "1.1.1.1", 5, 15); err != nil {
		t.Fatalf("login username: %v", err)
	}
	if _, err := um.Authenticate("ani@x.id", "Rahasia123", "1.1.1.1", 5, 15); err != nil {
		t.Fatalf("login email: %v", err)
	}
	_, e1 := um.Authenticate("ani.toko", "salah", "2.2.2.2", 5, 15)
	_, e2 := um.Authenticate("tidak.ada", "salah", "3.3.3.3", 5, 15)
	if e1 == nil || e2 == nil || e1.Error() != e2.Error() {
		t.Fatalf("pesan harus seragam: %v | %v", e1, e2)
	}
	// mengosongkan username
	if _, err := um.SetUsername(a.ID, ""); err != nil || um.FindByLogin("ani.toko") != nil {
		t.Fatalf("kosongkan username: %v", err)
	}
}

func TestLogin_UsernameAndPostRedirect(t *testing.T) {
	e := newAuthEngine(t)
	acc, _ := e.UserManager.CreateUser("Ani", "ani@x.id", "Rahasia123", "Owner", "4", "", "Active")
	_, _ = e.UserManager.SetUsername(acc.ID, "ani.toko")
	_ = e.UserManager.SetTenants(acc.ID, []string{"a", "b"}, "a")
	e.Auth.PostLoginRedirect = func(r *http.Request, u *User) string {
		if len(u.Tenants) > 1 {
			return "/workspaces"
		}
		return ""
	}
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		e.Auth.ServeLogin(rec, req)
		return rec
	}
	if rec := post("email=ani.toko&password=Rahasia123"); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/workspaces" {
		t.Fatalf("login username + pemilih tenant: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := post("login=ani@x.id&password=Rahasia123"); rec.Header().Get("Location") != "/workspaces" {
		t.Fatalf("login via field 'login': %s", rec.Header().Get("Location"))
	}
	_ = e.UserManager.SetTenants(acc.ID, []string{"a"}, "a")
	if rec := post("email=ani.toko&password=Rahasia123"); rec.Header().Get("Location") != e.AdminPath {
		t.Fatalf("satu tenant langsung ke beranda: %s", rec.Header().Get("Location"))
	}
	e.Auth.PostLoginRedirect = func(*http.Request, *User) string { return "//evil.example" }
	if rec := post("email=ani.toko&password=Rahasia123"); rec.Header().Get("Location") != e.AdminPath {
		t.Fatalf("tujuan eksternal harus diabaikan: %s", rec.Header().Get("Location"))
	}
	if rec := post("email=ani.toko&password=salah"); rec.Code == http.StatusSeeOther {
		t.Fatal("sandi salah tidak boleh masuk")
	}
	_ = time.Now
}

// Pergantian tenant: tujuan 'next' hanya path lokal; tenant di luar hak akses ditolak.
func TestTenantSwitch_NextParam(t *testing.T) {
	e := newAuthEngine(t)
	for _, s := range []string{"alpha", "beta", "gamma"} {
		if err := e.TenantManager.Save(&Tenant{Name: s, Slug: s, Plan: "Starter", Status: "Active"}); err != nil {
			t.Fatal(err)
		}
	}
	u := &User{ID: "9", Email: "x@x.id", Name: "X", RoleName: "Owner", PrivilegeID: "4", Tenants: []string{"alpha", "beta"}, CreatedAt: time.Now()}
	do := func(path string) *httptest.ResponseRecorder {
		req := sessionReq(e, u)
		req.URL.Path = path
		req.URL, _ = req.URL.Parse(path)
		req.Method = http.MethodGet
		rec := httptest.NewRecorder()
		e.handleTenantSwitch(rec, req)
		return rec
	}
	if rec := do("/admin/tenants/switch/alpha?next=/admin/invoices"); rec.Header().Get("Location") != "/admin/invoices" {
		t.Fatalf("next lokal: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := do("/admin/tenants/switch/alpha?next=//evil.example"); strings.Contains(rec.Header().Get("Location"), "evil") {
		t.Fatalf("next eksternal harus diabaikan: %s", rec.Header().Get("Location"))
	}
	if rec := do("/admin/tenants/switch/alpha?next=https://evil.example"); strings.Contains(rec.Header().Get("Location"), "evil") {
		t.Fatalf("next absolut harus diabaikan: %s", rec.Header().Get("Location"))
	}
	rec := do("/admin/tenants/switch/gamma?next=/admin")
	if strings.Contains(rec.Header().Get("Location"), "next") || !strings.Contains(rec.Header().Get("Location"), "Access+denied") {
		t.Fatalf("tenant di luar hak akses: %s", rec.Header().Get("Location"))
	}
}

// BeginLogin: akun ber-2FA tidak boleh mendapat sesi tanpa langkah kedua; akun tanpa 2FA langsung bersesi.
func TestBeginLogin_Enforces2FA(t *testing.T) {
	e := newAuthEngine(t)
	e.Auth.SecondFactor = fakeTF{code: "123456"}
	sessionCookieSet := func(rec *httptest.ResponseRecorder) bool {
		for _, c := range rec.Result().Cookies() {
			if c.Name == SessionCookieName && c.Value != "" {
				return true
			}
		}
		return false
	}
	rec := httptest.NewRecorder()
	if !e.Auth.BeginLogin(rec, &User{ID: "1", Email: "admin@tgo.io", Name: "A", CreatedAt: time.Now()}) || sessionCookieSet(rec) {
		t.Fatal("akun ber-2FA harus ditantang tanpa sesi")
	}
	rec = httptest.NewRecorder()
	if e.Auth.BeginLogin(rec, &User{ID: "2", Email: "tanpa2fa@x.id", Name: "B", CreatedAt: time.Now()}) || !sessionCookieSet(rec) {
		t.Fatal("akun tanpa 2FA harus langsung bersesi")
	}
}
