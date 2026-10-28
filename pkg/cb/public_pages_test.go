package cb

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServePublicPageCanBeDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	e.pages = []*Page{{ID: "1", Title: "Beranda", Slug: "home", RouteURL: "/", Status: "published", Template: "default", Content: "<p>hai</p>"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if rec := httptest.NewRecorder(); !e.ServePublicPage(rec, req) || rec.Code != http.StatusOK {
		t.Fatalf("halaman publik harus dilayani secara bawaan (code %d)", rec.Code)
	}

	e.DisablePublicPages = true
	if rec := httptest.NewRecorder(); e.ServePublicPage(rec, req) {
		t.Fatal("DisablePublicPages=true: tidak boleh melayani halaman publik")
	}

	e.DisablePublicPages = false
	s := e.GetSettings()
	s.DisablePublicPages = true
	if err := e.SaveSettings(&s); err != nil {
		t.Fatal(err)
	}
	if rec := httptest.NewRecorder(); e.ServePublicPage(rec, req) {
		t.Fatal("pengaturan disable_public_pages: tidak boleh melayani halaman publik")
	}
}

func TestPagesStudioMenuHiddenWhenPublicPagesDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	e.initDefaultMenus()
	has := func() bool {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		for _, m := range e.buildActiveMenus(req, "/admin", &User{RoleName: "superadmin"}) {
			if m.ID == "pages_studio" {
				return true
			}
			for _, c := range m.Children {
				if c.ID == "pages_studio" {
					return true
				}
			}
		}
		return false
	}
	if !has() {
		t.Skip("menu pages_studio tidak ada pada konfigurasi bawaan uji")
	}
	e.DisablePublicPages = true
	if has() {
		t.Fatal("menu Pages & SEO harus tersembunyi saat halaman publik dimatikan")
	}
}
