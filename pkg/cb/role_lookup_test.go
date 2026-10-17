package cb

import (
	"net/http"
	"testing"
	"time"
)

// Peran kustom (per tenant) didahulukan atas peran global; slug tak dikenal tetap ditolak; superadmin tak pernah diganti.
func TestRoleLookup_CustomRole(t *testing.T) {
	e := newAuthEngine(t)
	custom := &Role{Name: "Kasir", Slug: "custom-7", Permissions: map[string]PermissionMatrix{
		"invoices": {IsVisible: true, CanRead: true, CanCreate: true},
	}}
	e.SetRoleResolver(func(r *http.Request, u *User) string {
		switch u.Email {
		case "kasir@x.id":
			return "custom-7"
		case "hantu@x.id":
			return "custom-999"
		}
		return ""
	})
	e.SetRoleLookup(func(r *http.Request, slug string) *Role {
		if slug == "custom-7" {
			return custom
		}
		return nil
	})
	user := func(email string) *User {
		return &User{ID: email, Email: email, Name: email, RoleName: "Auditor", PrivilegeID: "3", CreatedAt: time.Now()}
	}

	req := sessionReq(e, user("kasir@x.id"))
	if r := e.EffectiveRole(req, user("kasir@x.id")); r == nil || r.Slug != "custom-7" {
		t.Fatalf("peran kustom tidak dipakai: %+v", r)
	}
	if !e.CheckPermission(sessionReq(e, user("kasir@x.id")), "invoices", "create") {
		t.Error("kasir harus boleh membuat invoice")
	}
	if e.CheckPermission(sessionReq(e, user("kasir@x.id")), "invoices", "delete") {
		t.Error("kasir tidak boleh menghapus invoice")
	}
	if e.CheckPermission(sessionReq(e, user("kasir@x.id")), "accounts", "read") {
		t.Error("kasir tidak boleh membaca akun")
	}
	if e.CheckPermission(sessionReq(e, user("hantu@x.id")), "invoices", "read") {
		t.Error("slug tidak dikenal harus ditolak, bukan jatuh ke peran akun")
	}
	// tanpa resolver → peran bawaan akun (Auditor boleh baca)
	if r := e.EffectiveRole(sessionReq(e, user("lain@x.id")), user("lain@x.id")); r == nil || r.Slug != "auditor" {
		t.Errorf("peran bawaan: %+v", r)
	}
}
