package cb

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func newAuthEngine(t *testing.T) *Engine {
	t.Helper()
	tmp := t.TempDir()
	old, _ := os.Getwd()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(old) })
	return NewEngine("Hardening")
}

func sessionReq(e *Engine, u *User) *http.Request {
	rec := httptest.NewRecorder()
	e.Auth.SetSessionUser(rec, u)
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

// Cookie sesi tanpa tanda tangan (format lama) tidak boleh diterima; pemalsuan dan kedaluwarsa ditolak.
func TestSession_RejectsForgedAndExpired(t *testing.T) {
	e := newAuthEngine(t)
	forged, _ := json.Marshal(User{ID: "1", Email: "x@y.z", RoleName: "Superadmin", PrivilegeID: "1", CreatedAt: time.Now()})
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: base64.RawURLEncoding.EncodeToString(forged)})
	if e.Auth.GetSessionUser(req) != nil {
		t.Fatal("cookie tanpa tanda tangan diterima sebagai sesi (pintu belakang)")
	}
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: base64.RawURLEncoding.EncodeToString(forged) + ".deadbeef"})
	if e.Auth.GetSessionUser(req) != nil {
		t.Fatal("tanda tangan palsu diterima")
	}
	if e.Auth.GetSessionUser(sessionReq(e, &User{ID: "1", Email: "a@b.c", RoleName: "Superadmin"})) == nil {
		t.Fatal("sesi sah harus diterima")
	}
	old := &User{ID: "1", Email: "a@b.c", RoleName: "Superadmin", CreatedAt: time.Now().Add(-25 * time.Hour)}
	if e.Auth.GetSessionUser(sessionReq(e, old)) != nil {
		t.Fatal("sesi lewat 24 jam harus ditolak di sisi server")
	}
}

func TestSession_SecretCanBeChanged(t *testing.T) {
	e := newAuthEngine(t)
	req := sessionReq(e, &User{ID: "1", Email: "a@b.c", RoleName: "Superadmin"})
	if e.Auth.GetSessionUser(req) == nil {
		t.Fatal("sesi harus sah sebelum ganti rahasia")
	}
	if err := SetSessionSecret([]byte("pendek")); err == nil {
		t.Fatal("rahasia pendek harus ditolak")
	}
	if err := SetSessionSecret([]byte(strings.Repeat("k", 40))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sessionSecretMu.Lock()
		sessionSecretCustom = false
		hmacSessionSecret = []byte("tgo-booster-dev-only-session-secret-change-me")
		sessionSecretMu.Unlock()
	})
	if e.Auth.GetSessionUser(req) != nil {
		t.Fatal("sesi bertanda tangan rahasia lama harus ditolak setelah rahasia diganti")
	}
	if SessionSecretIsDefault() {
		t.Fatal("rahasia kustom tidak boleh dianggap bawaan")
	}
}

// Tidak ada kata sandi bawaan yang tetap berlaku setelah diganti (pintu belakang admin123 dihapus); hash lama ditingkatkan ke bcrypt.
func TestAuthenticate_NoBackdoorAndUpgrade(t *testing.T) {
	e := newAuthEngine(t)
	um := e.UserManager
	acc, err := um.CreateUser("Uji", "uji@kriva.id", "Rahasia123", "Owner", "4", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if acc.PasswordAlgo != "bcrypt" {
		t.Fatalf("akun baru harus bcrypt: %q", acc.PasswordAlgo)
	}
	if err := um.UpdatePassword(um.FindByEmail("admin@tgo.io").ID, "BaruSekali99"); err != nil {
		t.Fatal(err)
	}
	if _, err := um.Authenticate("admin@tgo.io", "admin123", "1.1.1.1", 5, 15); err == nil {
		t.Fatal("admin123 tidak boleh masuk setelah kata sandi diganti")
	}
	if _, err := um.Authenticate("admin@tgo.io", "BaruSekali99", "1.1.1.1", 5, 15); err != nil {
		t.Fatalf("kata sandi baru harus berlaku: %v", err)
	}
	um.mu.Lock()
	for _, u := range um.users {
		if u.Email == "uji@kriva.id" {
			salt := GenerateRandomSalt()
			u.Salt, u.PasswordHash, u.PasswordAlgo = salt, HashUserPassword("LamaSekali1", salt), ""
		}
	}
	um.mu.Unlock()
	if _, err := um.Authenticate("uji@kriva.id", "LamaSekali1", "1.1.1.1", 5, 15); err != nil {
		t.Fatalf("hash lama harus tetap valid: %v", err)
	}
	if um.FindByEmail("uji@kriva.id").PasswordAlgo != "bcrypt" {
		t.Fatal("hash lama harus ditingkatkan ke bcrypt setelah login")
	}
}

// Ganti sandi/cabut sesi menolak sesi lama; akun ditangguhkan atau dihapus ditolak pada mode ketat.
func TestSession_RevocationAndStrictMode(t *testing.T) {
	e := newAuthEngine(t)
	um := e.UserManager
	acc, _ := um.CreateUser("Uji", "rev@kriva.id", "Rahasia123", "Finance", "5", "", "Active")
	mk := func() *User {
		return &User{ID: acc.ID, Email: acc.Email, RoleName: "Finance", CreatedAt: time.Now().Add(-time.Minute)}
	}
	if e.Auth.GetSessionUser(sessionReq(e, mk())) == nil {
		t.Fatal("sesi awal harus sah")
	}
	if err := um.UpdatePassword(acc.ID, "Rahasia456"); err != nil {
		t.Fatal(err)
	}
	if e.Auth.GetSessionUser(sessionReq(e, mk())) != nil {
		t.Fatal("sesi sebelum ganti sandi harus ditolak")
	}
	fresh := &User{ID: acc.ID, Email: acc.Email, RoleName: "Finance", CreatedAt: time.Now().Add(2 * time.Second)}
	if e.Auth.GetSessionUser(sessionReq(e, fresh)) == nil {
		t.Fatal("sesi baru setelah ganti sandi harus sah")
	}
	e.Auth.StrictSessions = true
	_ = um.UpdateUser(acc.ID, "", "", "", "", "", "", "Suspended")
	if e.Auth.GetSessionUser(sessionReq(e, fresh)) != nil {
		t.Fatal("akun ditangguhkan harus ditolak (mode ketat)")
	}
	_ = um.DeleteUser(acc.ID)
	if e.Auth.GetSessionUser(sessionReq(e, fresh)) != nil {
		t.Fatal("akun terhapus harus ditolak (mode ketat)")
	}
}

type fakeTF struct{ code string }

func (f fakeTF) Required(email string) bool          { return email == "admin@tgo.io" }
func (f fakeTF) Verify(email, code string) bool      { return code == f.code }
func (f fakeTF) ChallengeHTML(p, e, m string) string { return "<form>2FA " + m + "</form>" }

// Login dengan 2FA: sandi benar saja belum membuat sesi; kode salah dikunci setelah 5 percobaan.
func TestLogin_SecondFactor(t *testing.T) {
	e := newAuthEngine(t)
	e.Auth.SecondFactor = fakeTF{code: "123456"}
	if err := e.UserManager.UpdatePassword(e.UserManager.FindByEmail("admin@tgo.io").ID, "KataSandi123"); err != nil {
		t.Fatal(err)
	}
	do := func(path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		if strings.HasSuffix(path, "/2fa") {
			e.Auth.ServeLogin2FA(rec, req)
		} else {
			e.Auth.ServeLogin(rec, req)
		}
		return rec
	}
	rec := do("/admin/login", "email=admin@tgo.io&password=KataSandi123", nil)
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/login/2fa") {
		t.Fatalf("harus diarahkan ke 2FA: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	var pending []*http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			t.Fatal("sesi tidak boleh dibuat sebelum 2FA lulus")
		}
		pending = append(pending, c)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: pending[0].Value})
	if e.Auth.GetSessionUser(req) != nil {
		t.Fatal("cookie pending 2FA tidak boleh dipakai sebagai sesi")
	}
	if rec := do("/admin/login/2fa", "code=000000", pending); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("kode salah: %d", rec.Code)
	}
	for i := 0; i < 5; i++ {
		do("/admin/login/2fa", "code=111111", pending)
	}
	if rec := do("/admin/login/2fa", "code=123456", pending); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("setelah 5 gagal harus terkunci: %d", rec.Code)
	}
	e.Auth.tfFails = nil
	rec = do("/admin/login/2fa", "code=123456", pending)
	ok := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName && c.Value != "" {
			ok = true
		}
	}
	if rec.Code != http.StatusSeeOther || !ok {
		t.Fatalf("kode benar harus membuat sesi: %d", rec.Code)
	}
}
