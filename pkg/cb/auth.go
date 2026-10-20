package cb

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const SessionCookieName = "cb_session_token"

// hmacSessionSecret: rahasia penandatangan sesi. WAJIB diganti lewat SetSessionSecret di aplikasi nyata;
// nilai bawaan hanya untuk pengujian/pengembangan lokal dan memicu peringatan di log.
var (
	hmacSessionSecret     = []byte("tgo-booster-dev-only-session-secret-change-me")
	sessionSecretCustom   bool
	sessionSecretMu       sync.RWMutex
	sessionSecretWarnOnce sync.Once
)

// SetSessionSecret mengganti rahasia penandatangan sesi (minimal 32 byte).
func SetSessionSecret(b []byte) error {
	if len(b) < 32 {
		return errors.New("session secret minimal 32 byte")
	}
	sessionSecretMu.Lock()
	hmacSessionSecret = append([]byte(nil), b...)
	sessionSecretCustom = true
	sessionSecretMu.Unlock()
	return nil
}

// SessionSecretIsDefault: true bila rahasia sesi masih nilai bawaan (tidak aman untuk produksi).
func SessionSecretIsDefault() bool {
	sessionSecretMu.RLock()
	defer sessionSecretMu.RUnlock()
	return !sessionSecretCustom
}

// AuthManager handles admin authentication, rate limiting, and session security
type AuthManager struct {
	Engine    *Engine
	AdminPath string
	AppName   string

	// SecureCookies menandai cookie sesi Secure (aktifkan di belakang HTTPS).
	SecureCookies bool
	// SessionMaxAge: umur maksimum sesi (default 24 jam), ditegakkan di sisi server lewat waktu terbit.
	SessionMaxAge time.Duration
	// StrictSessions: sesi hanya sah bila akunnya masih ada, aktif, dan tidak dicabut (SessionsValidFrom).
	StrictSessions bool
	// TrustProxy: percayai X-Forwarded-For/X-Real-IP untuk alamat klien (hanya di belakang proxy tepercaya).
	TrustProxy bool
	// TrustedProxyHops: jumlah proxy tepercaya di depan aplikasi (bawaan 1). Alamat klien = entri X-Forwarded-For ke-N
	// dari KANAN; entri di kirinya dapat dipalsukan klien dan diabaikan.
	TrustedProxyHops int
	// PostLoginRedirect: tujuan setelah login berhasil (mis. pemilih tenant). Kosong = beranda admin.
	PostLoginRedirect func(r *http.Request, u *User) string
	// SecondFactor: bila diisi, akun yang memerlukannya harus lolos verifikasi langkah kedua sebelum sesi dibuat.
	SecondFactor SecondFactorProvider

	tfMu    sync.Mutex
	tfFails map[string][]time.Time
}

func NewAuthManager(adminPath, appName string) *AuthManager {
	return &AuthManager{
		AdminPath: adminPath,
		AppName:   appName,
	}
}

func signSessionPayload(payload string) string {
	sessionSecretMu.RLock()
	key := hmacSessionSecret
	sessionSecretMu.RUnlock()
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifySessionSignature(payload, signature string) bool {
	return verifySessionSignatureRaw(payload, signature)
}

// insecureSessionSecret: true bila APP_ENV=production tetapi rahasia sesi masih bawaan. Dalam keadaan ini sesi tidak
// dibuat maupun diterima (cookie bisa dipalsukan siapa pun yang tahu nilai bawaan). Aplikasi boleh memasang rahasia
// sesudah Mount; pemeriksaan ini terjadi saat sesi dipakai.
func insecureSessionSecret() bool {
	return SessionSecretIsDefault() && strings.EqualFold(os.Getenv("APP_ENV"), "production")
}

func verifySessionSignatureRaw(payload, signature string) bool {
	if insecureSessionSecret() {
		return false
	}
	expectedSig := signSessionPayload(payload)
	return hmac.Equal([]byte(expectedSig), []byte(signature))
}

// ServeLogin handles GET & POST for admin login with brute-force prevention and real user auth
func (a *AuthManager) ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// If already logged in, redirect to dashboard
		if user := a.GetSessionUser(r); user != nil {
			http.Redirect(w, r, a.AdminPath, http.StatusSeeOther)
			return
		}
		a.renderLogin(w, "", "")
		return
	}

	// Handle POST Login
	_ = r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	if email == "" {
		email = strings.TrimSpace(r.FormValue("login"))
	}
	password := strings.TrimSpace(r.FormValue("password"))

	if email == "" || password == "" {
		a.renderLogin(w, "Username/email and password are required.", email)
		return
	}

	clientIP := a.clientIP(r)

	maxAttempts := 5
	lockoutMin := 15
	if a.Engine != nil {
		st := a.Engine.GetSettings()
		if st.MaxLoginAttempts > 0 {
			maxAttempts = st.MaxLoginAttempts
		}
		if st.LockoutDurationMin > 0 {
			lockoutMin = st.LockoutDurationMin
		}
	}

	// Authenticate against UserManager if available
	if a.Engine != nil && a.Engine.UserManager != nil {
		acc, err := a.Engine.UserManager.Authenticate(email, password, clientIP, maxAttempts, lockoutMin)
		if err != nil {
			// Catat login gagal/lockout (tanpa kata sandi) agar upaya brute force terlihat di audit.
			action := "LOGIN_FAILED"
			if strings.HasPrefix(err.Error(), "Account locked") {
				action = "LOGIN_LOCKED"
			}
			a.Engine.LogAudit(r, action, "Auth", fmt.Sprintf("Login gagal untuk '%s' dari %s", strings.TrimSpace(email), clientIP))
			a.renderLogin(w, err.Error(), email)
			return
		}

		// Enforce Maintenance Mode: only Superadmin allowed if maintenance active
		st := a.Engine.GetSettings()
		if st.MaintenanceMode {
			isSuper := strings.EqualFold(acc.Role, "superadmin") || acc.RoleID == "1" || strings.EqualFold(acc.Role, "super administrator")
			if !isSuper {
				msg := "Platform is currently under scheduled maintenance."
				if st.MaintenanceMessage != "" {
					msg = st.MaintenanceMessage
				}
				a.renderLogin(w, msg, email)
				return
			}
		}

		user := &User{
			ID:           acc.ID,
			Name:         acc.Name,
			Email:        acc.Email,
			Photo:        acc.Avatar,
			PrivilegeID:  acc.RoleID,
			RoleName:     acc.Role,
			Tenants:      acc.Tenants,
			ActiveTenant: acc.DefaultTenant,
			CreatedAt:    time.Now(),
		}
		if user.ActiveTenant == "" && len(user.Tenants) > 0 && user.Tenants[0] != "*" {
			user.ActiveTenant = user.Tenants[0]
		}
		req2fa, tfErr := a.secondFactorRequired(user.Email)
		if tfErr != nil {
			log.Printf("[auth] gagal menentukan 2FA untuk %s: %v (login ditolak)", user.Email, tfErr)
			a.renderLogin(w, "Tidak dapat memverifikasi pengaturan keamanan akun. Coba lagi sebentar lagi.", email)
			return
		}
		if req2fa {
			a.setPending2FA(w, user)
			http.Redirect(w, r, a.AdminPath+"/login/2fa", http.StatusSeeOther)
			return
		}
		a.SetSessionUser(w, user)

		if a.Engine != nil {
			a.Engine.LogAudit(r, "LOGIN", "Auth", fmt.Sprintf("Admin user '%s' (%s) authenticated from %s", user.Name, user.Email, clientIP))
		}

		http.Redirect(w, r, a.afterLogin(r, user), http.StatusSeeOther)
		return
	}

	// Fallback check for standalone tests without full engine
	if a.Engine == nil && os.Getenv("APP_ENV") != "production" && (email == "admin@tgo.io" || email == "admin@example.com") && password == "admin123" {
		user := &User{
			ID:          "1",
			Name:        "Super Administrator",
			Email:       email,
			PrivilegeID: "1",
			RoleName:    "Superadmin",
			CreatedAt:   time.Now(),
		}
		a.SetSessionUser(w, user)
		http.Redirect(w, r, a.AdminPath, http.StatusSeeOther)
		return
	}

	a.renderLogin(w, "Invalid username/email or password.", email)
}

// ServeLogout handles logout, session invalidation, and audit logging
func (a *AuthManager) ServeLogout(w http.ResponseWriter, r *http.Request) {
	if user := a.GetSessionUser(r); user != nil && a.Engine != nil {
		a.Engine.LogAudit(r, "LOGOUT", "Auth", fmt.Sprintf("Admin user '%s' (%s) logged out", user.Name, user.Email))
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.SecureCookies,
	})
	http.Redirect(w, r, a.AdminPath+"/login", http.StatusSeeOther)
}

func (a *AuthManager) renderLogin(w http.ResponseWriter, errMsg, defaultEmail string) {
	data := map[string]interface{}{
		"AppName":      a.AppName,
		"AdminPath":    a.AdminPath,
		"LoginAction":  a.AdminPath + "/login",
		"Error":        errMsg,
		"DefaultEmail": defaultEmail,
	}
	htmlBytes, err := RenderLoginTemplate(data)
	if err != nil {
		http.Error(w, "Failed to render login: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(htmlBytes)
}

// SetSessionUser issues an HMAC-SHA256 signed session cookie
func (a *AuthManager) SetSessionUser(w http.ResponseWriter, user *User) {
	if insecureSessionSecret() {
		log.Printf("[SECURITY] sesi tidak dibuat: APP_ENV=production tetapi rahasia sesi masih bawaan (atur APP_SESSION_SECRET / SetSessionSecret)")
		return
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	userData, _ := json.Marshal(user)
	payload := base64.RawURLEncoding.EncodeToString(userData)
	sig := signSessionPayload(payload)
	token := payload + "." + sig

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(a.maxAge()),
		HttpOnly: true,
		Secure:   a.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionUser decodes and verifies user from signed session cookie.
// Cookie tanpa tanda tangan ditolak; umur sesi, status akun, dan pencabutan sesi ditegakkan di server.
func (a *AuthManager) GetSessionUser(r *http.Request) *User {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return nil
	}
	if !verifySessionSignature(parts[0], parts[1]) {
		return nil // Tampered / unsigned session token
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil
	}
	var user User
	if err := json.Unmarshal(decoded, &user); err != nil {
		return nil
	}
	if user.CreatedAt.IsZero() || time.Since(user.CreatedAt) > a.maxAge() || user.CreatedAt.After(time.Now().Add(5*time.Minute)) {
		return nil
	}
	if a.Engine != nil && a.Engine.UserManager != nil {
		acc := a.Engine.UserManager.FindByID(user.ID)
		switch {
		case acc == nil:
			if a.StrictSessions {
				return nil
			}
		case strings.EqualFold(acc.Status, "Suspended"):
			return nil
		case acc.SessionsValidFrom != "":
			if from, err := time.Parse(time.RFC3339, acc.SessionsValidFrom); err == nil && user.CreatedAt.Before(from) {
				return nil
			}
		}
		// Identitas, peran, dan daftar tenant selalu diambil dari akun terbaru, bukan salinan di cookie:
		// perubahan peran/pencabutan workspace berlaku seketika tanpa menunggu sesi kedaluwarsa.
		if acc != nil {
			user.PrivilegeID = acc.RoleID
			user.RoleName = acc.Role
			user.Name = acc.Name
			user.Email = acc.Email
			user.Tenants = append([]string(nil), acc.Tenants...)
			// Pratinjau peran hanya untuk akun yang benar-benar superadmin.
			if user.SwitchedRole != "" {
				if a.Engine.IsUserSuperadmin(&user) {
					if role := a.Engine.FindRole(user.SwitchedRole); role != nil {
						user.PrivilegeID = role.ID
						user.RoleName = role.Name
					}
				} else {
					user.SwitchedRole = ""
				}
			}
		}
	}
	return &user
}

func (a *AuthManager) maxAge() time.Duration {
	if a.SessionMaxAge > 0 {
		return a.SessionMaxAge
	}
	return 24 * time.Hour
}

func (a *AuthManager) clientIP(r *http.Request) string {
	return ClientIP(r, a.TrustProxy, a.TrustedProxyHops)
}

// ClientIP menentukan alamat klien. Tanpa trust: host dari RemoteAddr (header diabaikan). Dengan trust: entri
// X-Forwarded-For ke-`hops` dari kanan (bawaan 1), harus IP valid; bila tidak ada/tidak valid, X-Real-IP yang valid,
// lalu RemoteAddr. Entri kiri XFF tidak pernah dipercaya karena bisa disuntik klien.
func ClientIP(r *http.Request, trust bool, hops int) string {
	if trust {
		if hops < 1 {
			hops = 1
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if idx := len(parts) - hops; idx >= 0 {
				if ip := net.ParseIP(strings.TrimSpace(parts[idx])); ip != nil {
					return ip.String()
				}
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

// afterLogin menentukan tujuan setelah sesi dibuat (PostLoginRedirect bila ada; hanya path lokal yang diterima).
func (a *AuthManager) afterLogin(r *http.Request, u *User) string {
	if a.PostLoginRedirect != nil {
		if p := a.PostLoginRedirect(r, u); strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") {
			return p
		}
	}
	return a.AdminPath
}
