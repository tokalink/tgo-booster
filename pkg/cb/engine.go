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

	// 4. Built-in CRUDBooster v5.6 System Tools
	e.mountSystemTools(server)
}

func (e *Engine) mountSystemTools(server connect.Server) {
	// Module Generator
	server.Register(e.AdminPath+"/module_generator", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">⚡ Module Generator</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Generate full-featured Go CRUD controllers from your database schema in 1-click</p>
					</div>
					<button class="btn btn-primary btn-sm" onclick="alert('Module generator engine ready.')">+ Create New Module</button>
				</div>
				<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem;">
					<div class="cb-stat-card" style="border-left: 4px solid var(--accent);">
						<div style="font-weight: 800; font-size: 1.05rem; margin-bottom: 4px;">🛍️ Products Module</div>
						<div style="font-size: 0.8rem; color: var(--text-muted);">Table: <code>products</code> • 5 Columns • 4 Form Fields</div>
						<div style="margin-top: 1rem; display: flex; gap: 6px;">
							<a href="%s/products" class="btn btn-secondary btn-sm">Open Module</a>
							<span class="cb-badge cb-badge-success">Active</span>
						</div>
					</div>
					<div class="cb-stat-card" style="border-left: 4px solid #a855f7;">
						<div style="font-weight: 800; font-size: 1.05rem; margin-bottom: 4px;">👥 Customers Module</div>
						<div style="font-size: 0.8rem; color: var(--text-muted);">Table: <code>customers</code> • 4 Columns • 2 Form Fields</div>
						<div style="margin-top: 1rem; display: flex; gap: 6px;">
							<a href="%s/customers" class="btn btn-secondary btn-sm">Open Module</a>
							<span class="cb-badge cb-badge-success">Active</span>
						</div>
					</div>
					<div class="cb-stat-card" style="border-left: 4px solid #10b981;">
						<div style="font-weight: 800; font-size: 1.05rem; margin-bottom: 4px;">📦 Orders Module</div>
						<div style="font-size: 0.8rem; color: var(--text-muted);">Table: <code>orders</code> • 4 Columns • 3 Form Fields</div>
						<div style="margin-top: 1rem; display: flex; gap: 6px;">
							<a href="%s/orders" class="btn btn-secondary btn-sm">Open Module</a>
							<span class="cb-badge cb-badge-success">Active</span>
						</div>
					</div>
				</div>
			</div>
		`, e.AdminPath, e.AdminPath, e.AdminPath)
		e.RenderLayout(w, r, "Module Generator", template.HTML(html))
	}))

	// Menu Management
	server.Register(e.AdminPath+"/menus", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">📑 Menu Management</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Customize navigation order, icons, and privilege permissions</p>
					</div>
					<button class="btn btn-primary btn-sm" onclick="alert('Menu item added.')">+ Add Menu Item</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>Icon</th><th>Menu Title</th><th>Route Path</th><th>Privilege Access</th><th>Status</th></tr>
					</thead>
					<tbody>
						<tr><td>📊</td><td><strong>Dashboard</strong></td><td><code>/admin</code></td><td>All Roles</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
						<tr><td>🛍️</td><td><strong>Products</strong></td><td><code>/admin/products</code></td><td>Superadmin, Manager</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
						<tr><td>👥</td><td><strong>Customers</strong></td><td><code>/admin/customers</code></td><td>Superadmin, Sales</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
						<tr><td>📦</td><td><strong>Orders</strong></td><td><code>/admin/orders</code></td><td>Superadmin, Finance</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "Menu Management", template.HTML(html))
	}))

	// Privileges & Roles
	server.Register(e.AdminPath+"/privileges", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">🛡️ Privileges & Roles (CBPrivileges)</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Role-Based Access Control matrix (Create, Read, Update, Delete)</p>
					</div>
					<button class="btn btn-primary btn-sm">+ Add Role</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>Role Name</th><th>Is Superadmin</th><th>Users Count</th><th>Permissions Matrix</th><th>Actions</th></tr>
					</thead>
					<tbody>
						<tr><td><strong>Superadmin</strong></td><td><span class="cb-badge cb-badge-success">Yes</span></td><td>1 User</td><td><span class="cb-badge cb-badge-primary">Full Access (C,R,U,D)</span></td><td><button class="btn btn-secondary btn-sm">Edit</button></td></tr>
						<tr><td><strong>Operations Manager</strong></td><td><span class="cb-badge cb-badge-warning">No</span></td><td>4 Users</td><td><span class="cb-badge cb-badge-primary">Read, Write (C,R,U)</span></td><td><button class="btn btn-secondary btn-sm">Edit</button></td></tr>
						<tr><td><strong>Read-Only Auditor</strong></td><td><span class="cb-badge cb-badge-warning">No</span></td><td>2 Users</td><td><span class="cb-badge cb-badge-primary">Read Only (R)</span></td><td><button class="btn btn-secondary btn-sm">Edit</button></td></tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "Privileges & Roles", template.HTML(html))
	}))

	// Users Management
	server.Register(e.AdminPath+"/users", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">👥 User Management (CBUsers)</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Manage administrator accounts and privilege bindings</p>
					</div>
					<button class="btn btn-primary btn-sm">+ Add Admin User</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>User</th><th>Email Address</th><th>Privilege Role</th><th>Status</th><th>Actions</th></tr>
					</thead>
					<tbody>
						<tr>
							<td>
								<div style="display: flex; align-items: center; gap: 8px;">
									<div class="table-avatar" style="background: #38bdf8;">AD</div>
									<strong>Admin Super</strong>
								</div>
							</td>
							<td><code>admin@tgo.io</code></td>
							<td><span class="cb-badge cb-badge-primary">Superadmin</span></td>
							<td><span class="cb-badge cb-badge-success">Active</span></td>
							<td><button class="btn btn-secondary btn-sm">Edit</button></td>
						</tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "User Management", template.HTML(html))
	}))

	// API Generator
	server.Register(e.AdminPath+"/api_generator", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">🔑 API Generator & Secret Keys</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Generate ConnectRPC and RESTful endpoints with bearer authentication</p>
					</div>
					<button class="btn btn-primary btn-sm">+ Generate New API</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>Endpoint Name</th><th>Protocol</th><th>Path</th><th>Secret Key</th><th>Status</th></tr>
					</thead>
					<tbody>
						<tr><td><strong>Products List API</strong></td><td><span class="cb-badge cb-badge-primary">ConnectRPC / REST</span></td><td><code>/api/v1/products</code></td><td><code>tgo_sec_99a8b7c6</code></td><td><span class="cb-badge cb-badge-success">Enabled</span></td></tr>
						<tr><td><strong>Orders Webhook</strong></td><td><span class="cb-badge cb-badge-primary">HTTP POST</span></td><td><code>/api/v1/orders/webhook</code></td><td><code>tgo_sec_44e3f211</code></td><td><span class="cb-badge cb-badge-success">Enabled</span></td></tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "API Generator", template.HTML(html))
	}))

	// Email Templates
	server.Register(e.AdminPath+"/email_templates", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">✉️ Email & Notification Templates</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Automated transactional notifications for system events</p>
					</div>
					<button class="btn btn-primary btn-sm">+ Add Template</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>Template Name</th><th>Trigger Slug</th><th>Subject</th><th>Status</th></tr>
					</thead>
					<tbody>
						<tr><td><strong>Welcome New Admin</strong></td><td><code>auth.welcome_user</code></td><td>Welcome to TGo Enterprise Admin</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
						<tr><td><strong>Order Payment Confirmation</strong></td><td><code>order.paid</code></td><td>Your Order #{{order_id}} has been paid</td><td><span class="cb-badge cb-badge-success">Active</span></td></tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "Email Templates", template.HTML(html))
	}))

	// CBLogs Audit
	server.Register(e.AdminPath+"/logs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">📋 CBLogs (Live Audit Trail)</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">Immutable records of all database modifications and user logins</p>
					</div>
					<button class="btn btn-secondary btn-sm" onclick="exportCSV()">⬇️ Export Logs CSV</button>
				</div>
				<table class="cb-table">
					<thead>
						<tr><th>IP Address</th><th>User</th><th>Description</th><th>Timestamp</th></tr>
					</thead>
					<tbody>
						<tr><td><code>127.0.0.1</code></td><td><strong>Admin Super</strong></td><td>Created Product SKU: MacBook Pro M3 Max</td><td style="color: var(--text-muted);">Just now</td></tr>
						<tr><td><code>127.0.0.1</code></td><td><strong>Admin Super</strong></td><td>Updated Customer Profile: Budi Cahyono</td><td style="color: var(--text-muted);">6 mins ago</td></tr>
						<tr><td><code>127.0.0.1</code></td><td><strong>System</strong></td><td>Automated database backup completed successfully</td><td style="color: var(--text-muted);">1 hour ago</td></tr>
					</tbody>
				</table>
			</div>
		`
		e.RenderLayout(w, r, "CBLogs Audit", template.HTML(html))
	}))

	// Settings
	server.Register(e.AdminPath+"/settings", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := fmt.Sprintf(`
			<div class="cb-card" style="max-width: 750px; margin: 0 auto;">
				<div class="cb-card-header">
					<div>
						<h2 class="cb-card-title">⚙️ Application Settings</h2>
						<p style="font-size: 0.82rem; color: var(--text-muted); margin-top: 2px;">General configuration and branding properties</p>
					</div>
				</div>
				<form onsubmit="event.preventDefault(); alert('Settings saved successfully.');" style="display: flex; flex-direction: column; gap: 1.25rem;">
					<div class="cb-form-group">
						<label class="cb-form-label">Application Name</label>
						<input type="text" class="cb-input" value="%s">
					</div>
					<div class="cb-form-group">
						<label class="cb-form-label">Admin Path Prefix</label>
						<input type="text" class="cb-input" value="%s" readonly style="opacity: 0.7;">
					</div>
					<div class="cb-form-group">
						<label class="cb-form-label">Default Theme Skin</label>
						<select class="cb-input">
							<option value="obsidian" selected>Obsidian Glass Dark (Default)</option>
							<option value="midnight">Midnight Indigo Blue</option>
							<option value="emerald">Emerald Corporate</option>
						</select>
					</div>
					<div style="display: flex; justify-content: flex-end; margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--border);">
						<button type="submit" class="btn btn-primary">Save Settings</button>
					</div>
				</form>
			</div>
		`, e.AppName, e.AdminPath)
		e.RenderLayout(w, r, "Settings", template.HTML(html))
	}))
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

	htmlBytes, err := RenderMasterLayout(data)
	if err != nil {
		http.Error(w, "Failed to render layout: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(htmlBytes)
}
