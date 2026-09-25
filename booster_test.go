package booster

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
