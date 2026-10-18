package cb

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func lovReq(e *Engine, u *User, q url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if u != nil {
		req = sessionReq(e, u)
	} else {
		req = httptest.NewRequest(http.MethodGet, "/admin/api/lov", nil)
	}
	req.URL.RawQuery = q.Encode()
	rec := httptest.NewRecorder()
	e.handleLOVAPI(rec, req)
	return rec
}

// LOV hanya melayani tabel milik controller terdaftar, untuk pengguna login yang berizin baca; SQL tak pernah disusun
// dari parameter.
func TestLOV_OnlyRegisteredControllersWithPermission(t *testing.T) {
	e := newAuthEngine(t)
	store := NewMemoryStore()
	_ = store.Create(nil, map[string]interface{}{"id": "1", "name": "Alice"})
	ctrl := &Controller{Title: "Staff", Table: "staff", PrimaryKey: "id", DataProvider: store}
	ctrl.AddCol("ID", "id", ColText, false, true).AddCol("Name", "name", ColText, true, true)
	e.Register(ctrl)

	// Peran tanpa izin baca "staff".
	e.roles = append(e.roles, &Role{ID: "9", Name: "Tamu", Slug: "tamu", Permissions: map[string]PermissionMatrix{}})
	tamu := &User{ID: "u9", Name: "Tamu", Email: "t@uji.id", PrivilegeID: "9", RoleName: "Tamu"}
	// Peran dengan izin baca.
	e.roles = append(e.roles, &Role{ID: "8", Name: "Baca", Slug: "baca", Permissions: map[string]PermissionMatrix{"staff": {CanRead: true, IsVisible: true}}})
	baca := &User{ID: "u8", Name: "Baca", Email: "b@uji.id", PrivilegeID: "8", RoleName: "Baca"}

	if rec := lovReq(e, nil, url.Values{"table": {"staff"}}); rec.Code != http.StatusUnauthorized {
		t.Errorf("tanpa sesi: %d, harus 401", rec.Code)
	}
	for _, tbl := range []string{"plat_documents", "pg_catalog.pg_user", "staff;drop table x", "users"} {
		if rec := lovReq(e, baca, url.Values{"table": {tbl}}); rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("tabel %q: %d, harus 400/404", tbl, rec.Code)
		}
	}
	for _, col := range []string{"name) FROM users --", "(select 1)", "a;b", "NAME", "1x"} {
		if rec := lovReq(e, baca, url.Values{"table": {"staff"}, "label": {col}}); rec.Code != http.StatusBadRequest {
			t.Errorf("kolom %q: %d, harus 400", col, rec.Code)
		}
	}
	if rec := lovReq(e, tamu, url.Values{"table": {"staff"}}); rec.Code != http.StatusForbidden {
		t.Errorf("tanpa izin baca: %d, harus 403", rec.Code)
	}
	rec := lovReq(e, baca, url.Values{"table": {"staff"}})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Alice") {
		t.Errorf("dengan izin: %d %s", rec.Code, rec.Body.String())
	}
}
