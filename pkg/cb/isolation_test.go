package cb

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// isoEnv: engine mode tenant ketat dengan dua tenant dan satu anggota tenant A (bukan superadmin).
type isoEnv struct {
	e      *Engine
	member *UserAccount
	super  *UserAccount
}

func newIsoEnv(t *testing.T) *isoEnv {
	t.Helper()
	t.Chdir(t.TempDir()) // jangan menulis data/*.json milik repositori
	e := NewEngine("Uji")
	e.EnableMultiTenancy(true)
	e.StrictTenancy = true
	for _, s := range []string{"iso-a", "iso-b"} {
		if e.TenantManager.GetBySlug(s) == nil {
			if err := e.TenantManager.Save(&Tenant{Name: s, Slug: s, Status: "Active"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	n := time.Now().UnixNano()
	member, err := e.UserManager.CreateUser("Ani", fmt.Sprintf("ani-%d@iso.test", n), "rahasia-uji-123", "Operations Manager", "2", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.UserManager.SetTenants(member.ID, []string{"iso-a"}, "iso-a"); err != nil {
		t.Fatal(err)
	}
	super, err := e.UserManager.CreateUser("Root", fmt.Sprintf("root-%d@iso.test", n), "rahasia-uji-123", "Superadmin", "1", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = e.UserManager.DeleteUser(member.ID)
		_ = e.UserManager.DeleteUser(super.ID)
	})
	return &isoEnv{e: e, member: member, super: super}
}

// session menerbitkan cookie sesi; isi cookie sengaja dipalsukan (tenant & tenant aktif lain) untuk memastikan hak
// akses dibaca dari akun, bukan dari cookie.
func (env *isoEnv) session(acc *UserAccount, forgedTenants []string, forgedActive string) *http.Cookie {
	rec := httptest.NewRecorder()
	env.e.Auth.SetSessionUser(rec, &User{ID: acc.ID, Name: acc.Name, Email: acc.Email, RoleName: acc.Role,
		PrivilegeID: acc.RoleID, Tenants: forgedTenants, ActiveTenant: forgedActive})
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			return c
		}
	}
	return nil
}

func (env *isoEnv) resolve(cookie *http.Cookie, mutate func(r *http.Request)) string {
	r := httptest.NewRequest(http.MethodGet, "/admin/billing", nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if mutate != nil {
		mutate(r)
	}
	return env.e.ResolveTenant(r, env.e.Auth.GetSessionUser(r))
}

func TestStrictTenancyBlocksCrossTenantSelection(t *testing.T) {
	env := newIsoEnv(t)
	c := env.session(env.member, []string{"iso-b", "*"}, "iso-b")
	if c == nil {
		t.Fatal("cookie sesi tidak terbit")
	}
	cases := map[string]func(r *http.Request){
		"tanpa pilihan":            nil,
		"query ?tenant=iso-b":      func(r *http.Request) { r.URL.RawQuery = "tenant=iso-b" },
		"cookie tenant aktif":      func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "cb_active_tenant", Value: "iso-b"}) },
		"header X-Tenant-Slug":     func(r *http.Request) { r.Header.Set("X-Tenant-Slug", "iso-b") },
		"query tenant tak dikenal": func(r *http.Request) { r.URL.RawQuery = "tenant=default" },
	}
	for name, mut := range cases {
		if got := env.resolve(c, mut); got != "iso-a" {
			t.Errorf("%s: tenant = %q, ingin iso-a (isi cookie palsu tidak boleh berlaku)", name, got)
		}
	}
	u := env.e.Auth.GetSessionUser(httptest.NewRequest(http.MethodGet, "/", nil))
	_ = u
	if env.e.TenantManager.UserCanAccess(&User{ID: env.member.ID, RoleName: "Operations Manager", Tenants: []string{"*"}}, env.e.TenantManager.GetBySlug("iso-b")) {
		t.Error("tanda * tidak boleh memberi akses non-superadmin di mode ketat")
	}
	if env.e.TenantManager.UserCanAccess(&User{ID: "x", RoleName: "Operations Manager"}, env.e.TenantManager.GetBySlug("default")) {
		t.Error("pengguna tanpa tenant tidak boleh jatuh ke tenant default")
	}
}

func TestStrictTenancyRevocationIsImmediate(t *testing.T) {
	env := newIsoEnv(t)
	c := env.session(env.member, []string{"iso-a"}, "iso-a")
	if got := env.resolve(c, nil); got != "iso-a" {
		t.Fatalf("sebelum dicabut: %q", got)
	}
	if err := env.e.UserManager.SetTenants(env.member.ID, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := env.resolve(c, nil); got != "" {
		t.Fatalf("setelah dicabut sesi lama masih memakai tenant %q", got)
	}
}

func TestStrictTenancySuperadminMaySwitch(t *testing.T) {
	env := newIsoEnv(t)
	c := env.session(env.super, []string{"*"}, "")
	if got := env.resolve(c, func(r *http.Request) { r.URL.RawQuery = "tenant=iso-b" }); got != "iso-b" {
		t.Fatalf("superadmin harus bisa memilih tenant: %q", got)
	}
}

func TestLOVRequiresAuthAndBlocksSystemTables(t *testing.T) {
	env := newIsoEnv(t)
	call := func(cookie *http.Cookie, q string) int {
		r := httptest.NewRequest(http.MethodGet, "/admin/api/lov?"+q, nil)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		env.e.handleLOVAPI(w, r)
		return w.Code
	}
	member := env.session(env.member, nil, "")
	super := env.session(env.super, nil, "")
	if code := call(nil, "table=users&label=email"); code != http.StatusUnauthorized {
		t.Errorf("anonim: %d, ingin 401", code)
	}
	// Modul sistem terdaftar saat Mount; tanpa Mount jawabannya 404. Keduanya menolak — yang penting bukan 200.
	if code := call(member, "table=users&label=email"); code != http.StatusForbidden && code != http.StatusNotFound {
		t.Errorf("anggota membaca modul pengguna: %d, ingin ditolak (403/404)", code)
	}
	if code := call(member, "table=axera.plat_documents&label=body"); code != http.StatusNotFound {
		t.Errorf("anggota membaca tabel sembarang: %d, ingin 404", code)
	}
	if code := call(super, "table=users;drop&label=email"); code != http.StatusBadRequest {
		t.Errorf("nama tabel berbahaya: %d, ingin 400", code)
	}
	if code := call(super, "table=users&label=email)%20from%20x--"); code != http.StatusBadRequest {
		t.Errorf("nama kolom berbahaya: %d, ingin 400", code)
	}
	// Tabel tanpa controller terdaftar tidak pernah dikueri langsung, bahkan untuk superadmin.
	if code := call(super, "table=users&label=email"); code != http.StatusNotFound {
		t.Errorf("superadmin, tabel tanpa controller: %d, ingin 404", code)
	}
}
