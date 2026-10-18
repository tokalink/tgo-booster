package cb

import (
	"net/http"
	"strings"
)

// DashboardHandler renders the modern Executive Admin Dashboard
type DashboardHandler struct {
	Engine *Engine
}

func (d *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := d.Engine.Auth.GetSessionUser(r)
	if user == nil {
		http.Redirect(w, r, d.Engine.AdminPath+"/login", http.StatusSeeOther)
		return
	}

	// Calculate initials
	initials := "AD"
	parts := strings.Split(user.Name, " ")
	if len(parts) >= 2 {
		initials = strings.ToUpper(string(parts[0][0]) + string(parts[1][0]))
	} else if len(parts) == 1 && len(parts[0]) > 0 {
		initials = strings.ToUpper(string(parts[0][0]))
	}

	// Calculate live counts
	usersCount := 0
	if d.Engine.UserManager != nil {
		usersCount = len(d.Engine.UserManager.GetAllUsers())
	}

	var recentLogs []AuditLogEntry
	if d.Engine.AuditLogger != nil {
		allLogs := d.Engine.AuditLogger.GetAllLogs()
		if len(allLogs) > 6 {
			recentLogs = allLogs[:6]
		} else {
			recentLogs = allLogs
		}
	}

	// Non-superadmin: jangan tampilkan data global lintas tenant (log audit platform, jumlah semua akun, token API).
	apiTokens := len(d.Engine.apiTokens)
	if !d.Engine.IsUserSuperadmin(user) {
		active := ""
		if d.Engine != nil {
			active = d.Engine.ResolveTenant(r, user)
		}
		// Log audit belum menyimpan tenant (docs/14 temuan #8): non-superadmin tidak melihat log platform.
		recentLogs = nil
		apiTokens = 0
		usersCount = 0
		if d.Engine.UserManager != nil {
			for _, u := range d.Engine.UserManager.GetAllUsers() {
				for _, t := range u.Tenants {
					if active != "" && strings.EqualFold(t, active) {
						usersCount++
						break
					}
				}
			}
		}
	}

	data := map[string]interface{}{
		"AppName":               d.Engine.AppName,
		"AdminPath":             d.Engine.AdminPath,
		"User":                  user,
		"UserInitials":          initials,
		"Controllers":           d.Engine.controllers,
		"ActiveModulesCount":    len(d.Engine.controllers),
		"TotalControllersCount": len(d.Engine.controllers) + len(d.Engine.systemCtrls),
		"UsersCount":            usersCount,
		"PagesCount":            len(d.Engine.pages),
		"APITokensCount":        apiTokens,
		"Diagnostics":           d.Engine.collectSystemDiagnostics(),
		"RecentLogs":            recentLogs,
	}

	contentHTML, err := RenderDashboardContent(data)
	if err != nil {
		http.Error(w, "Failed to render dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}

	d.Engine.RenderLayout(w, r, "Executive Dashboard", contentHTML)
}
