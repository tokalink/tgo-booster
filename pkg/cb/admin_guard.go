package cb

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

// adminServerAdapter membungkus connect.Server agar semua rute di bawah AdminPath wajib memiliki sesi login.
type adminServerAdapter struct {
	connect.Server
	engine *Engine
}

func (a *adminServerAdapter) Register(path string, h http.Handler) {
	if a.engine != nil && (path == a.engine.AdminPath || strings.HasPrefix(path, a.engine.AdminPath+"/")) {
		h = a.engine.requireSession(h)
	}
	a.Server.Register(path, h)
}

func (e *Engine) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.Auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		p := r.URL.Path
		if p == e.AdminPath+"/login" || p == e.AdminPath+"/login/2fa" || p == e.AdminPath+"/logout" {
			next.ServeHTTP(w, r)
			return
		}
		if e.Auth.GetSessionUser(r) == nil {
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Silakan masuk terlebih dahulu",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (e *Engine) requireSuperadminJSON(w http.ResponseWriter, r *http.Request) bool {
	if e.Auth == nil {
		return true
	}
	user := e.Auth.GetSessionUser(r)
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Silakan masuk terlebih dahulu",
		})
		return false
	}
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Forbidden: Superadmin role required",
		})
		return false
	}
	return true
}
