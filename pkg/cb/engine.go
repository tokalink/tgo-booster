package cb

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

// Engine is the central CRUDBooster management system
type Engine struct {
	AppName     string
	AdminPath   string
	Auth        *AuthManager
	Dashboard   *DashboardHandler
	controllers []*Controller
	menus       []MenuItem
}

// NewEngine creates a new CRUDBooster Engine
func NewEngine(appName ...string) *Engine {
	name := "TGo Booster"
	if len(appName) > 0 && appName[0] != "" {
		name = appName[0]
	}

	e := &Engine{
		AppName:   name,
		AdminPath: "/admin",
		menus:     make([]MenuItem, 0),
	}
	e.Auth = NewAuthManager(e.AdminPath, e.AppName)
	e.Dashboard = &DashboardHandler{Engine: e}
	return e
}

// Register adds a CRUD module controller
func (e *Engine) Register(c *Controller) *Engine {
	c.Engine = e
	c.BasePath = fmt.Sprintf("%s/%s", e.AdminPath, c.Table)
	c.initDefaults()
	e.controllers = append(e.controllers, c)

	// Automatically add to sidebar menus
	e.menus = append(e.menus, MenuItem{
		ID:    c.Table,
		Title: c.Title,
		Icon:  c.Icon,
		Path:  c.BasePath,
	})
	return e
}

// AddMenu adds a custom sidebar menu item
func (e *Engine) AddMenu(title, icon, path string, badge ...string) *Engine {
	b := ""
	if len(badge) > 0 {
		b = badge[0]
	}
	e.menus = append(e.menus, MenuItem{
		ID:         strings.ToLower(strings.ReplaceAll(title, " ", "_")),
		Title:      title,
		Icon:       icon,
		Path:       path,
		Badge:      b,
		BadgeColor: "#38bdf8",
	})
	return e
}

// Mount registers all booster routes (Auth, Dashboard, CRUD modules) to the TGo server
func (e *Engine) Mount(server connect.Server, prefix ...string) {
	if len(prefix) > 0 && prefix[0] != "" {
		e.AdminPath = strings.TrimSuffix(prefix[0], "/")
		e.Auth.AdminPath = e.AdminPath
	}

	// 1. Auth routes
	server.Register(e.AdminPath+"/login", http.HandlerFunc(e.Auth.ServeLogin))
	server.Register(e.AdminPath+"/logout", http.HandlerFunc(e.Auth.ServeLogout))

	// 2. Dashboard route
	server.Register(e.AdminPath, e.Dashboard)
	server.Register(e.AdminPath+"/", e.Dashboard)

	// 3. Register all CRUD module controllers
	for _, ctrl := range e.controllers {
		ctrl.BasePath = fmt.Sprintf("%s/%s", e.AdminPath, ctrl.Table)
		server.Register(ctrl.BasePath, ctrl)
		server.Register(ctrl.BasePath+"/", ctrl)
	}
}

// RenderLayout renders a page inside the master CRUDBooster layout
func (e *Engine) RenderLayout(w http.ResponseWriter, r *http.Request, pageTitle string, content template.HTML) {
	user := e.Auth.GetSessionUser(r)
	if user == nil {
		user = &User{Name: "Guest Admin", RoleName: "Administrator"}
	}

	// Calculate initials
	initials := "AD"
	parts := strings.Split(user.Name, " ")
	if len(parts) >= 2 {
		initials = strings.ToUpper(string(parts[0][0]) + string(parts[1][0]))
	} else if len(parts) == 1 && len(parts[0]) > 0 {
		initials = strings.ToUpper(string(parts[0][0]))
	}

	// Mark active menu item
	currentPath := r.URL.Path
	activeMenus := make([]MenuItem, len(e.menus))
	copy(activeMenus, e.menus)
	for i := range activeMenus {
		if strings.HasPrefix(currentPath, activeMenus[i].Path) {
			activeMenus[i].IsActive = true
		}
	}

	tmpl, _ := template.New("master").Parse(MasterHTML)
	data := map[string]interface{}{
		"AppName":      e.AppName,
		"AdminPath":    e.AdminPath,
		"PageTitle":    pageTitle,
		"ActivePath":   currentPath,
		"User":         user,
		"UserInitials": initials,
		"MenuItems":    activeMenus,
		"Content":      content,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}
