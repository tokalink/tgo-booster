package cb

import (
	"log"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// SecondFactorProvider menyediakan langkah kedua login (mis. TOTP). Diisi aplikasi.
type SecondFactorProvider interface {
	// Required: apakah akun dengan email ini wajib verifikasi langkah kedua.
	Required(email string) bool
	// Verify memeriksa kode (TOTP atau kode cadangan sekali pakai).
	Verify(email, code string) bool
	// ChallengeHTML menghasilkan halaman input kode (dirender aplikasi agar sesuai merek).
	ChallengeHTML(adminPath, email, errMsg string) string
}

const pending2FACookie = "cb_2fa_pending"
const pending2FATTL = 5 * time.Minute

type pending2FA struct {
	User *User  `json:"u"`
	Exp  int64  `json:"e"`
	Kind string `json:"k"`
}

func (a *AuthManager) setPending2FA(w http.ResponseWriter, user *User) {
	b, _ := json.Marshal(pending2FA{User: user, Exp: time.Now().Add(pending2FATTL).Unix(), Kind: "2fa"})
	payload := base64.RawURLEncoding.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{Name: pending2FACookie, Value: payload + "." + signSessionPayload("2fa|"+payload), Path: "/",
		MaxAge: int(pending2FATTL.Seconds()), HttpOnly: true, Secure: a.SecureCookies, SameSite: http.SameSiteLaxMode})
}

// SecondFactorRequirer (opsional) melaporkan galat saat menentukan kewajiban langkah kedua. Bila provider
// mengimplementasikannya, galat membuat login ditolak (gagal-tertutup) alih-alih dianggap "tidak wajib".
type SecondFactorRequirer interface {
	RequiredErr(email string) (bool, error)
}

// secondFactorRequired: (wajib, galat). Tanpa SecondFactor -> (false, nil).
func (a *AuthManager) secondFactorRequired(email string) (bool, error) {
	if a.SecondFactor == nil {
		return false, nil
	}
	if re, ok := a.SecondFactor.(SecondFactorRequirer); ok {
		return re.RequiredErr(email)
	}
	return a.SecondFactor.Required(email), nil
}

// BeginLogin menyelesaikan login bagi pengguna yang kata sandinya SUDAH diverifikasi pemanggil (mis. penerimaan undangan).
// Bila akun mewajibkan langkah kedua, sesi TIDAK dibuat: cookie pending diterbitkan dan pemanggil harus mengalihkan ke
// AdminPath+"/login/2fa" (needs2FA=true). Selain itu sesi dibuat langsung. Semua jalur masuk berkata sandi wajib lewat sini.
func (a *AuthManager) BeginLogin(w http.ResponseWriter, user *User) (needs2FA bool) {
	req, err := a.secondFactorRequired(user.Email)
	if err != nil {
		// Gagal-tertutup: tanpa sesi dan tanpa pending; pengguna diarahkan ke /login/2fa yang akan menolak.
		log.Printf("[auth] gagal menentukan 2FA untuk %s: %v (login ditolak)", user.Email, err)
		return true
	}
	if req {
		a.setPending2FA(w, user)
		return true
	}
	a.SetSessionUser(w, user)
	return false
}

func (a *AuthManager) readPending2FA(r *http.Request) *User {
	c, err := r.Cookie(pending2FACookie)
	if err != nil {
		return nil
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 || !verifyPendingSig(parts[0], parts[1]) {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil
	}
	var p pending2FA
	if json.Unmarshal(raw, &p) != nil || p.Kind != "2fa" || p.User == nil || time.Now().Unix() > p.Exp {
		return nil
	}
	return p.User
}

func verifyPendingSig(payload, sig string) bool {
	return verifySessionSignatureRaw("2fa|"+payload, sig)
}

func (a *AuthManager) tfLocked(key string) bool {
	a.tfMu.Lock()
	defer a.tfMu.Unlock()
	cut := time.Now().Add(-15 * time.Minute)
	kept := a.tfFails[key][:0]
	for _, t := range a.tfFails[key] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if a.tfFails == nil {
		a.tfFails = map[string][]time.Time{}
	}
	a.tfFails[key] = kept
	return len(kept) >= 5
}

func (a *AuthManager) tfFail(key string) {
	a.tfMu.Lock()
	if len(a.tfFails) > 5000 { // buang kunci yang semua catatannya sudah kedaluwarsa
		cut := time.Now().Add(-15 * time.Minute)
		for k, ts := range a.tfFails {
			if len(ts) == 0 || ts[len(ts)-1].Before(cut) {
				delete(a.tfFails, k)
			}
		}
	}
	if a.tfFails == nil {
		a.tfFails = map[string][]time.Time{}
	}
	a.tfFails[key] = append(a.tfFails[key], time.Now())
	a.tfMu.Unlock()
}

// ServeLogin2FA: langkah kedua login. Sesi baru dibuat hanya setelah kode benar; 5 kegagalan/15 menit mengunci.
func (a *AuthManager) ServeLogin2FA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	user := a.readPending2FA(r)
	if user == nil || a.SecondFactor == nil {
		http.Redirect(w, r, a.AdminPath+"/login", http.StatusSeeOther)
		return
	}
	render := func(msg string, code int) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(a.SecondFactor.ChallengeHTML(a.AdminPath, user.Email, msg)))
	}
	if r.Method != http.MethodPost {
		render("", http.StatusOK)
		return
	}
	key := strings.ToLower(user.Email) + "@" + a.clientIP(r)
	if a.tfLocked(key) {
		render("Terlalu banyak percobaan. Coba lagi dalam 15 menit.", http.StatusTooManyRequests)
		return
	}
	_ = r.ParseForm()
	if !a.SecondFactor.Verify(user.Email, strings.TrimSpace(r.FormValue("code"))) {
		a.tfFail(key)
		render("Kode tidak valid.", http.StatusUnprocessableEntity)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: pending2FACookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.SecureCookies})
	user.CreatedAt = time.Now()
	a.SetSessionUser(w, user)
	if a.Engine != nil {
		a.Engine.LogAudit(r, "LOGIN", "Auth", "Login dengan 2FA: "+user.Email)
	}
	http.Redirect(w, r, a.afterLogin(r, user), http.StatusSeeOther)
}
