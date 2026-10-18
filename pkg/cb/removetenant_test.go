package cb

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRemoveTenantRevokesAccessAndSessions(t *testing.T) {
	e := newAuthEngine(t)
	acc, err := e.UserManager.CreateUser("Budi", "budi@uji.id", "Sandi#12345", "Operations Manager", "2", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.UserManager.SetTenants(acc.ID, []string{"a", "b"}, "a"); err != nil {
		t.Fatal(err)
	}
	e.Auth.StrictSessions = true
	time.Sleep(1100 * time.Millisecond) // resolusi detik RFC3339: sesi lama harus lebih tua dari pencabutan
	u := &User{ID: acc.ID, Name: acc.Name, Email: acc.Email, PrivilegeID: "2", RoleName: "Operations Manager", Tenants: []string{"a", "b"}, ActiveTenant: "a"}
	req := sessionReq(e, u)
	if e.Auth.GetSessionUser(req) == nil {
		t.Fatal("sesi baru harus sah")
	}
	time.Sleep(1100 * time.Millisecond)

	got, err := e.UserManager.RemoveTenant(acc.ID, "A")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tenants) != 1 || got.Tenants[0] != "b" || got.DefaultTenant != "" {
		t.Fatalf("tenant/default tak sesuai: %+v", got)
	}
	if e.Auth.GetSessionUser(req) != nil {
		t.Error("sesi lama harus ditolak setelah RemoveTenant")
	}
	// idempoten
	if g2, err := e.UserManager.RemoveTenant(acc.ID, "a"); err != nil || len(g2.Tenants) != 1 {
		t.Errorf("idempoten gagal: %v %+v", err, g2)
	}
	// superadmin "*" tidak terpengaruh
	if err := e.UserManager.SetTenants(acc.ID, []string{"*"}, ""); err != nil {
		t.Fatal(err)
	}
	if g3, _ := e.UserManager.RemoveTenant(acc.ID, "*"); len(g3.Tenants) != 1 {
		t.Error("tenant '*' tidak boleh dicabut")
	}
}

func TestStrictRoleResolutionDeniesNonMembers(t *testing.T) {
	e := newAuthEngine(t)
	u := &User{ID: "x", Name: "X", Email: "x@uji.id", PrivilegeID: "2", RoleName: "Operations Manager"}
	r := httptest.NewRequest("GET", "/admin", nil)

	// Tanpa resolver: perilaku lama (peran global).
	if e.EffectiveRole(r, u) == nil {
		t.Fatal("tanpa resolver peran global harus berlaku")
	}
	e.SetRoleResolver(func(r *http.Request, u *User) string { return "" }) // bukan anggota
	if e.EffectiveRole(r, u) == nil {
		t.Fatal("StrictRoleResolution mati: peran global tetap cadangan (perilaku lama)")
	}
	e.StrictRoleResolution = true
	if e.EffectiveRole(r, u) != nil {
		t.Error("non-anggota tidak boleh mendapat peran global saat StrictRoleResolution")
	}
	// Superadmin tidak terpengaruh.
	sup := &User{ID: "s", Name: "S", Email: "s@uji.id", PrivilegeID: "1", RoleName: "Superadmin"}
	if e.EffectiveRole(r, sup) == nil {
		t.Error("superadmin tidak boleh terpengaruh")
	}
	// Anggota (resolver mengembalikan slug) tetap mendapat perannya.
	e.SetRoleResolver(func(r *http.Request, u *User) string { return "auditor" })
	if role := e.EffectiveRole(r, u); role == nil || role.Slug != "auditor" {
		t.Errorf("anggota harus mendapat peran dari resolver: %+v", role)
	}
}
