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
		"APITokensCount":        len(d.Engine.apiTokens),
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
