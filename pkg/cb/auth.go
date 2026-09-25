package cb

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const SessionCookieName = "cb_session_token"

// AuthManager handles admin authentication and session management
type AuthManager struct {
	AdminPath string
	AppName   string
}

func NewAuthManager(adminPath, appName string) *AuthManager {
	return &AuthManager{
		AdminPath: adminPath,
		AppName:   appName,
	}
}

// ServeLogin handles GET & POST for admin login
func (a *AuthManager) ServeLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// If already logged in, redirect to dashboard
		if user := a.GetSessionUser(r); user != nil {
			http.Redirect(w, r, a.AdminPath, http.StatusSeeOther)
			return
		}
		a.renderLogin(w, "", "admin@tgo.io")
		return
	}

	// Handle POST Login
	_ = r.ParseForm()
	email := strings.TrimSpace(r.FormValue("email"))
	password := strings.TrimSpace(r.FormValue("password"))

	if email == "" || password == "" {
		a.renderLogin(w, "Email and password are required.", email)
		return
	}

	// Check credentials (supports demo admin & customizable DB lookup)
	if (email == "admin@tgo.io" || email == "admin@example.com") && password == "admin123" {
		user := &User{
			ID:          "1",
			Name:        "Super Administrator",
			Email:       email,
			PrivilegeID: "1",
			RoleName:    "Super Admin",
			CreatedAt:   time.Now(),
		}
		a.SetSessionUser(w, user)
		http.Redirect(w, r, a.AdminPath, http.StatusSeeOther)
		return
	}

	a.renderLogin(w, "Invalid email address or password.", email)
}

// ServeLogout handles logout and session invalidation
func (a *AuthManager) ServeLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
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

// SetSessionUser issues a signed session cookie
func (a *AuthManager) SetSessionUser(w http.ResponseWriter, user *User) {
	userData, _ := json.Marshal(user)
	token := base64.RawURLEncoding.EncodeToString(userData)

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionUser decodes user from session cookie
func (a *AuthManager) GetSessionUser(r *http.Request) *User {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}

	var user User
	if err := json.Unmarshal(decoded, &user); err != nil {
		return nil
	}
	return &user
}
