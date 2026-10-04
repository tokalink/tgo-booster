package cb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

// Engine is the central Booster management system
type Engine struct {
	AppName     string
	AdminPath   string
	Auth        *AuthManager
	Dashboard   *DashboardHandler
	UserManager *UserManager
	AuditLogger *AuditLogger
	EmailTemplates *EmailTemplateManager
	controllers []*Controller
	systemCtrls []*Controller
	menus       []*MenuItem
	roles       []*Role
	pages       []*Page
	settings    *AppSettings
	apiTokens   []*APIToken
	customAdminPages []*CustomAdminPage
	db          *sql.DB
	server      connect.Server
}

// NewEngine creates a new Booster Engine
func NewEngine(appName ...string) *Engine {
	name := "TGo Booster"
	if len(appName) > 0 && appName[0] != "" {
		name = appName[0]
	}

	e := &Engine{
		AppName:   name,
		AdminPath: "/admin",
		menus:     make([]*MenuItem, 0),
		roles:     make([]*Role, 0),
		pages:     make([]*Page, 0),
		apiTokens: make([]*APIToken, 0),
		customAdminPages: make([]*CustomAdminPage, 0),
	}
	e.Auth = NewAuthManager(e.AdminPath, e.AppName)
	e.Auth.Engine = e
	e.Dashboard = &DashboardHandler{Engine: e}
	e.UserManager = NewUserManager(e)
	e.AuditLogger = NewAuditLogger(e)
	e.EmailTemplates = NewEmailTemplateManager(e)
	e.LoadDynamicRoles()
	e.LoadDynamicMenus()
	e.LoadDynamicPages()
	e.LoadDynamicSettings()
	e.LoadDynamicTokens()
	return e
}

// Register adds a CRUD module controller
func (e *Engine) Register(c *Controller) *Engine {
	c.Engine = e
	c.BasePath = fmt.Sprintf("%s/%s", e.AdminPath, c.Table)
	c.initDefaults()
	e.controllers = append(e.controllers, c)

	// Automatically ensure presence in sidebar menus
	e.syncControllerMenu(c)
	return e
}

// AddMenu adds a custom sidebar menu item
func (e *Engine) AddMenu(title, icon, path string, badge ...string) *Engine {
	b := ""
	if len(badge) > 0 {
		b = badge[0]
	}
	item := &MenuItem{
		ID:         strings.ToLower(strings.ReplaceAll(title, " ", "_")),
		Title:      title,
		Type:       "url",
		Icon:       icon,
		Path:       path,
		Badge:      b,
		BadgeColor: "primary",
		Order:      len(e.menus) + 1,
	}
	e.menus = append(e.menus, item)
	return e
}

// CustomAdminPage represents an arbitrary standalone custom page inside the admin console
type CustomAdminPage struct {
	Path        string
	Title       string
	Icon        string
	Badge       string
	RequireRole string
	Handler     func(w http.ResponseWriter, r *http.Request) template.HTML
}

// AddCustomPage registers an arbitrary admin page with full layout, session authentication, and optional sidebar menu
func (e *Engine) AddCustomPage(path, title, icon string, handler func(w http.ResponseWriter, r *http.Request) template.HTML, addToMenu ...bool) *Engine {
	normPath := "/" + strings.Trim(path, "/")
	fullPath := e.AdminPath + normPath

	page := &CustomAdminPage{
		Path:    fullPath,
		Title:   title,
		Icon:    icon,
		Handler: handler,
	}
	e.customAdminPages = append(e.customAdminPages, page)

	// Add to sidebar menu by default unless explicitly disabled
	shouldAddMenu := true
	if len(addToMenu) > 0 {
		shouldAddMenu = addToMenu[0]
	}
	if shouldAddMenu {
		e.AddMenu(title, icon, fullPath)
	}

	// Register on server if server is already running/mounted
	if e.server != nil {
		e.registerCustomAdminPage(e.server, page)
	}
	return e
}

func (e *Engine) registerCustomAdminPage(server connect.Server, page *CustomAdminPage) {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e.Auth != nil {
			user := e.Auth.GetSessionUser(r)
			if user == nil {
				http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
				return
			}
			if page.RequireRole != "" && !strings.EqualFold(user.RoleName, page.RequireRole) && !e.IsUserSuperadmin(user) {
				e.RenderForbidden(w, r, page.Title, page.RequireRole)
				return
			}
		}

		contentHTML := page.Handler(w, r)
		e.RenderLayout(w, r, page.Title, contentHTML)
	})

	server.Register(page.Path, handlerFunc)
	server.Register(page.Path+"/", handlerFunc)
}

// FindController looks up a registered controller by table name (business modules & system tools)
func (e *Engine) FindController(table string) *Controller {
	for _, c := range e.controllers {
		if c.Table == table {
			return c
		}
	}
	for _, c := range e.systemCtrls {
		if c.Table == table {
			return c
		}
	}
	return nil
}

// SetDB sets the shared database connection pool for relational lookups
func (e *Engine) SetDB(db *sql.DB) *Engine {
	e.db = db
	return e
}

// FindDB returns the configured database, or looks for one in registered controllers
func (e *Engine) FindDB() *sql.DB {
	if e.db != nil {
		return e.db
	}
	for _, c := range e.controllers {
		if s, ok := c.DataProvider.(*SQLStore); ok && s.DB() != nil {
			return s.DB()
		}
	}
	return nil
}

// handleLOVAPI provides JSON records for the LOV modal and dynamic relations
func (e *Engine) handleLOVAPI(w http.ResponseWriter, r *http.Request) {
	table := strings.TrimSpace(r.URL.Query().Get("table"))
	if table == "" {
		http.Error(w, `{"error":"Table parameter is required"}`, http.StatusBadRequest)
		return
	}
	labelCol := strings.TrimSpace(r.URL.Query().Get("label"))
	if labelCol == "" {
		labelCol = "name"
	}
	keyCol := strings.TrimSpace(r.URL.Query().Get("key"))
	if keyCol == "" {
		keyCol = "id"
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))

	var results []map[string]interface{}

	// 1. Check registered controller first
	if ctrl := e.FindController(table); ctrl != nil {
		rows := ctrl.fetchRows(r)
		for _, row := range rows {
			keyVal := fmt.Sprint(row[keyCol])
			labelVal := fmt.Sprint(row[labelCol])
			if labelVal == "" || labelVal == "<nil>" {
				labelVal = keyVal
			}
			if q == "" || strings.Contains(strings.ToLower(keyVal), q) || strings.Contains(strings.ToLower(labelVal), q) {
				results = append(results, map[string]interface{}{
					keyCol:   keyVal,
					labelCol: labelVal,
					"id":     keyVal,
					"name":   labelVal,
				})
			}
		}
	} else if db := e.FindDB(); db != nil {
		// 2. Query target table directly from SQL database
		query := fmt.Sprintf("SELECT %s, %s FROM %s", keyCol, labelCol, table)
		var args []interface{}
		if q != "" {
			query += fmt.Sprintf(" WHERE LOWER(%s) LIKE ? OR LOWER(%s) LIKE ?", labelCol, keyCol)
			args = append(args, "%"+q+"%", "%"+q+"%")
		}
		query += fmt.Sprintf(" ORDER BY %s ASC LIMIT 100", labelCol)
		rows, err := db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, l interface{}
				if err := rows.Scan(&k, &l); err == nil {
					keyVal := fmt.Sprint(k)
					labelVal := fmt.Sprint(l)
					results = append(results, map[string]interface{}{
						keyCol:   keyVal,
						labelCol: labelVal,
						"id":     keyVal,
						"name":   labelVal,
					})
				}
			}
		}
	}

	if results == nil {
		results = make([]map[string]interface{}, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

// Mount registers all booster routes (Auth, Dashboard, CRUD modules) to the TGo server
func (e *Engine) Mount(server connect.Server, prefix ...string) {
	e.server = server
	if len(prefix) > 0 && prefix[0] != "" {
		e.AdminPath = strings.TrimSuffix(prefix[0], "/")
		e.Auth.AdminPath = e.AdminPath
	}

	// Restore previously generated dynamic modules, roles, menus, and pages from persistent storage
	e.LoadDynamicModules()
	e.LoadDynamicRoles()
	e.LoadDynamicMenus()
	e.LoadDynamicPages()
	e.LoadDynamicTokens()

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

	// 4. Built-in Booster Studio Tools
	e.mountSystemTools(server)

	// 5. Public REST & ConnectRPC API Endpoints (/api/v1/...)
	e.registerPublicAPIRoutes(server)

	// 6. LOV Modal & Dynamic Relation API
	server.Register(e.AdminPath+"/api/lov", http.HandlerFunc(e.handleLOVAPI))

	// 7. Register Public Custom Page Routes
	e.RegisterPageRoutes(server)

	// 8. Register Standalone Custom Admin Pages
	for _, cp := range e.customAdminPages {
		e.registerCustomAdminPage(server, cp)
	}
}

func (e *Engine) registerSystemController(server connect.Server, ctrl *Controller) {
	ctrl.Engine = e
	ctrl.BasePath = fmt.Sprintf("%s/%s", e.AdminPath, ctrl.Table)
	ctrl.initDefaults()
	e.systemCtrls = append(e.systemCtrls, ctrl)
	server.Register(ctrl.BasePath, ctrl)
	server.Register(ctrl.BasePath+"/", ctrl)
}

func (e *Engine) mountSystemTools(server connect.Server) {
	hasModule := func(tbl string) bool {
		for _, c := range e.controllers {
			if c.Table == tbl {
				return true
			}
		}
		return false
	}

	// 1. User Accounts (/admin/users)
	if !hasModule("users") {
		userCtrl := &Controller{
			Title:        "User Accounts",
			Table:        "users",
			Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/></svg>`,
			DataProvider: e.UserManager.NewDataProvider(),
		}
		userCtrl.
			AddCol("ID", "id", ColText, false, true).
			AddCol("Avatar", "avatar", ColImage, false, false).
			AddCol("User Name", "name", ColText, true, true).
			AddCol("Email Address", "email", ColEmail, true, true).
			AddCol("Privilege Role", "role", ColBadge, false, true).
			AddCol("Status", "status", ColBadge, false, false).
			AddCol("Registered At", "created_at", ColText, false, true)

		roleOpts := make([]Option, 0, len(e.roles))
		for _, r := range e.roles {
			roleOpts = append(roleOpts, Option{Label: r.Name, Value: r.Name})
		}
		if len(roleOpts) == 0 {
			roleOpts = []Option{
				{Label: "Superadmin", Value: "Superadmin"},
				{Label: "Operations Manager", Value: "Operations Manager"},
				{Label: "Read-Only Auditor", Value: "Read-Only Auditor"},
			}
		}

		userCtrl.
			AddForm("Full Name", "name", InputText, true, "e.g. John Doe").
			AddForm("Email Address", "email", InputEmail, true, "admin@enterprise.com").
			AddForm("Profile Photo", "avatar", InputUpload, false, "").
			AddForm("Password", "password", InputPassword, false, "Leave blank to keep current password").
			AddSelect("Privilege Role", "role", true, "Select role...", roleOpts...).
			AddSelect("Account Status", "status", true, "Select status...",
				Option{Label: "Active", Value: "Active"},
				Option{Label: "Suspended", Value: "Suspended"},
			)
		e.registerSystemController(server, userCtrl)
	}

	// 2. Privileges & Roles (/admin/privileges)
	server.Register(e.AdminPath+"/privileges", http.HandlerFunc(e.handlePrivileges))
	server.Register(e.AdminPath+"/privileges/", http.HandlerFunc(e.handlePrivileges))

	// 3. API Generator & Access Keys (/admin/api_generator)
	e.registerAPIGeneratorRoutes(server)

	// 4. Email Templates (/admin/email_templates)
	if !hasModule("email_templates") {
		emailCtrl := &Controller{
			Title:        "Email Templates",
			Table:        "email_templates",
			Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/></svg>`,
			DataProvider: e.EmailTemplates.NewDataProvider(),
		}
		emailCtrl.
			AddCol("ID", "id", ColText, false, true).
			AddCol("Template Name", "title", ColText, true, true).
			AddCol("Trigger Slug", "slug", ColText, true, true).
			AddCol("Email Subject", "subject", ColText, true, true).
			AddCol("Status", "status", ColBadge, false, false)

		emailCtrl.
			AddForm("Template Name", "title", InputText, true, "e.g. Order Invoice").
			AddForm("Trigger Event Slug", "slug", InputText, true, "e.g. order.invoice").
			AddForm("Email Subject", "subject", InputText, true, "Subject line...").
			AddForm("Email Body", "content", InputWYSIWYG, false, "HTML / Markdown email body...").
			AddSelect("Status", "status", true, "Status...",
				Option{Label: "Active", Value: "Active"},
				Option{Label: "Draft", Value: "Draft"},
			)
		e.registerSystemController(server, emailCtrl)
	}

	// 5. Security & Audit Trail (/admin/logs)
	if !hasModule("logs") {
		logCtrl := &Controller{
			Title:        "Security & Audit Trail",
			Table:        "logs",
			Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`,
			DataProvider: e.AuditLogger.NewDataProvider(),
		}
		logCtrl.
			AddCol("ID", "id", ColText, false, true).
			AddCol("IP Address", "ip", ColText, false, false).
			AddCol("User Account", "user", ColText, true, true).
			AddCol("Action Type", "action", ColBadge, false, true).
			AddCol("Module", "module", ColBadge, false, true).
			AddCol("Description", "description", ColText, true, false).
			AddCol("Timestamp", "created_at", ColText, false, true)
		e.registerSystemController(server, logCtrl)
	}

	// 6. Module Generator Studio (/admin/module_generator)
	server.Register(e.AdminPath+"/module_generator", http.HandlerFunc(e.handleModuleGenerator))
	server.Register(e.AdminPath+"/module_generator/", http.HandlerFunc(e.handleModuleGenerator))

	// 7. Menu Management (/admin/menus)
	server.Register(e.AdminPath+"/menus", http.HandlerFunc(e.handleMenus))
	server.Register(e.AdminPath+"/menus/", http.HandlerFunc(e.handleMenus))
	server.Register(e.AdminPath+"/menus/api/menu", http.HandlerFunc(e.handleMenuAPI))
	server.Register(e.AdminPath+"/menus/save", http.HandlerFunc(e.handleMenuSave))
	server.Register(e.AdminPath+"/menus/delete", http.HandlerFunc(e.handleMenuDelete))
	server.Register(e.AdminPath+"/menus/reorder", http.HandlerFunc(e.handleMenuReorder))
	server.Register(e.AdminPath+"/menus/reorder_tree", http.HandlerFunc(e.handleMenuReorderTree))
	server.Register(e.AdminPath+"/menus/reset", http.HandlerFunc(e.handleMenuReset))

	// 8. Custom Pages & SEO Studio (/admin/pages)
	server.Register(e.AdminPath+"/pages", http.HandlerFunc(e.handlePages))
	server.Register(e.AdminPath+"/pages/", http.HandlerFunc(e.handlePages))
	server.Register(e.AdminPath+"/pages/api/page", http.HandlerFunc(e.handlePageAPI))
	server.Register(e.AdminPath+"/pages/save", http.HandlerFunc(e.handlePageSave))
	server.Register(e.AdminPath+"/pages/delete", http.HandlerFunc(e.handlePageDelete))
	server.Register(e.AdminPath+"/pages/preview", http.HandlerFunc(e.handlePagePreview))
	server.Register(e.AdminPath+"/pages/generate-ai", http.HandlerFunc(e.handlePageGenerateAI))

	// 9. Application Settings Studio (/admin/settings)
	e.registerSettingsRoutes(server)
}

// RenderLayout renders a page inside the master Booster layout
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

	// Filter and mark active menu items based on role visibility permissions
	currentPath := r.URL.Path
	activeMenus := e.buildActiveMenus(r, currentPath, user)

	data := map[string]interface{}{
		"AppName":      e.AppName,
		"AdminPath":    e.AdminPath,
		"PageTitle":    pageTitle,
		"ActivePath":   currentPath,
		"User":         user,
		"UserInitials": initials,
		"MenuItems":    activeMenus,
		"IsSuperadmin": e.IsUserSuperadmin(user),
		"Content":      content,
		"Settings":     e.GetSettings(),
	}

	htmlBytes, err := RenderMasterLayout(data)
	if err != nil {
		http.Error(w, "Failed to render layout: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(htmlBytes)
}

// RegisterDynamic dynamically registers a controller at runtime and mounts its routes if server is active
func (e *Engine) RegisterDynamic(c *Controller) *Engine {
	c.Engine = e
	c.BasePath = fmt.Sprintf("%s/%s", e.AdminPath, c.Table)
	c.initDefaults()

	found := false
	for i, existing := range e.controllers {
		if existing.Table == c.Table {
			e.controllers[i] = c
			found = true
			break
		}
	}
	if !found {
		e.controllers = append(e.controllers, c)
		e.syncControllerMenu(c)
		_ = e.SaveMenus()
	} else {
		if item, _ := e.findMenuItem(c.Table); item != nil {
			item.Title = c.Title
			item.Icon = c.Icon
			item.Path = c.BasePath
			_ = e.SaveMenus()
		}
	}

	if e.server != nil {
		e.server.Register(c.BasePath, c)
		e.server.Register(c.BasePath+"/", c)
	}
	return e
}

// Unregister removes a dynamic controller and its sidebar menu from the active engine
func (e *Engine) Unregister(table string) bool {
	removed := false
	for i, c := range e.controllers {
		if c.Table == table {
			e.controllers = append(e.controllers[:i], e.controllers[i+1:]...)
			removed = true
			break
		}
	}
	if e.deleteMenuItem(table) {
		removed = true
	}
	return removed
}

func (e *Engine) getPersistencePath() string {
	candidates := []string{
		"data/modules.json",
		"starter/data/modules.json",
		"../starter/data/modules.json",
	}
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return c
		}
	}
	return "data/modules.json"
}

// LoadDynamicModules restores any previously generated CRUD modules from modules.json
func (e *Engine) LoadDynamicModules() {
	p := e.getPersistencePath()
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var defs []ModuleDefinition
	if err := json.Unmarshal(data, &defs); err != nil {
		return
	}

	for _, def := range defs {
		if e.FindController(def.Table) != nil {
			continue // Already registered in Go code
		}
		ctrl := &Controller{
			Title:      def.Title,
			Table:      def.Table,
			Icon:       def.Icon,
			PrimaryKey: def.PrimaryKey,
			OrderBy:    def.OrderBy,
			Columns:    def.Columns,
			Forms:      def.Forms,
			Widgets:    def.Widgets,
		}
		if def.ProviderType == "sql" && e.FindDB() != nil {
			ctrl.DataProvider = NewSQLStore(e.FindDB(), def.Table, def.PrimaryKey)
		} else {
			sampleRows := generateSampleRows(def)
			ctrl.DataProvider = NewMemoryStore(sampleRows...)
		}
		e.Register(ctrl)
	}
}

// SaveModuleDefinition persists a module definition to disk
func (e *Engine) SaveModuleDefinition(def ModuleDefinition) error {
	p := e.getPersistencePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)

	var defs []ModuleDefinition
	if data, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(data, &defs)
	}

	found := false
	for i, d := range defs {
		if d.Table == def.Table {
			defs[i] = def
			found = true
			break
		}
	}
	if !found {
		defs = append(defs, def)
	}

	data, err := json.MarshalIndent(defs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// AutoMigrateAll executes AutoMigrate on all registered controllers backed by SQLStore.
func (e *Engine) AutoMigrateAll() error {
	for _, ctrl := range e.controllers {
		if err := ctrl.AutoMigrate(); err != nil {
			return err
		}
	}
	return nil
}


// DeleteModuleDefinition removes a module definition from persistent disk storage
func (e *Engine) DeleteModuleDefinition(table string) error {
	p := e.getPersistencePath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var defs []ModuleDefinition
	if err := json.Unmarshal(data, &defs); err != nil {
		return err
	}

	filtered := make([]ModuleDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Table != table {
			filtered = append(filtered, d)
		}
	}

	out, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, out, 0644)
}

// LogAudit records an operational or security event into the persistent audit trail
func (e *Engine) LogAudit(r *http.Request, action, module, description string) {
	if e.AuditLogger != nil {
		e.AuditLogger.Log(r, action, module, description)
	}
}

// handleModuleGenerator handles Module Generator Studio UI and all sub-endpoints
func (e *Engine) handleModuleGenerator(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		user := e.Auth.GetSessionUser(r)
		if user == nil {
			http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
			return
		}
		if !e.IsUserSuperadmin(user) {
			e.RenderForbidden(w, r, "Module Generator", "Superadmin")
			return
		}
	}

	subpath := strings.TrimPrefix(r.URL.Path, e.AdminPath+"/module_generator")
	subpath = strings.TrimPrefix(subpath, "/")

	switch {
	case subpath == "api/tables":
		e.handleAPITables(w, r)
	case subpath == "api/columns":
		e.handleAPIColumns(w, r)
	case subpath == "api/module":
		e.handleAPIModule(w, r)
	case subpath == "save" && r.Method == http.MethodPost:
		e.handleSaveModule(w, r)
	case subpath == "delete" && r.Method == http.MethodPost:
		e.handleDeleteModule(w, r)
	default:
		e.handleModuleStudioView(w, r)
	}
}

func (e *Engine) handleAPITables(w http.ResponseWriter, r *http.Request) {
	tableSet := make(map[string]bool)
	var tables []string

	// 1. From active controllers
	for _, c := range e.controllers {
		if !tableSet[c.Table] {
			tableSet[c.Table] = true
			tables = append(tables, c.Table)
		}
	}

	// 2. From database introspection if db connected
	if db := e.FindDB(); db != nil {
		rows, err := db.Query("SHOW TABLES")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var tbl string
				if err := rows.Scan(&tbl); err == nil && tbl != "" && !tableSet[tbl] {
					tableSet[tbl] = true
					tables = append(tables, tbl)
				}
			}
		} else {
			// Try SQLite
			sRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'cb_%'")
			if err == nil {
				defer sRows.Close()
				for sRows.Next() {
					var tbl string
					if err := sRows.Scan(&tbl); err == nil && tbl != "" && !tableSet[tbl] {
						tableSet[tbl] = true
						tables = append(tables, tbl)
					}
				}
			} else {
				// Try PostgreSQL
				pRows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'")
				if err == nil {
					defer pRows.Close()
					for pRows.Next() {
						var tbl string
						if err := pRows.Scan(&tbl); err == nil && tbl != "" && !tableSet[tbl] {
							tableSet[tbl] = true
							tables = append(tables, tbl)
						}
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tables)
}

func (e *Engine) handleAPIColumns(w http.ResponseWriter, r *http.Request) {
	tbl := strings.TrimSpace(r.URL.Query().Get("table"))
	if tbl == "" {
		http.Error(w, `{"error":"Table parameter is required"}`, http.StatusBadRequest)
		return
	}

	var cols []Column

	// 1. If database connected, inspect schema using database/sql ColumnTypes
	if db := e.FindDB(); db != nil {
		query := fmt.Sprintf("SELECT * FROM `%s` WHERE 1=0", tbl)
		rows, err := db.Query(query)
		if err != nil {
			query = fmt.Sprintf("SELECT * FROM %s WHERE 1=0", tbl)
			rows, err = db.Query(query)
		}

		if err == nil {
			defer rows.Close()
			if types, err := rows.ColumnTypes(); err == nil && len(types) > 0 {
				for _, ct := range types {
					name := ct.Name()
					dbType := strings.ToLower(ct.DatabaseTypeName())

					label := formatFriendlyLabel(name)
					colType := inferColumnType(name, dbType)

					searchable := (colType == ColText || colType == ColEmail)
					sortable := (colType != ColImage)

					width := ""
					if name == "id" {
						width = "80px"
					}

					cols = append(cols, Column{
						Label:      label,
						Name:       name,
						Type:       colType,
						Searchable: searchable,
						Sortable:   sortable,
						Width:      width,
					})
				}
			}
		}
	}

	// 2. If no DB columns found, check registered controller
	if len(cols) == 0 {
		if ctrl := e.FindController(tbl); ctrl != nil && len(ctrl.Columns) > 0 {
			cols = ctrl.Columns
		}
	}

	// 3. Fallback defaults
	if len(cols) == 0 {
		cols = []Column{
			{Label: "ID", Name: "id", Type: ColText, Searchable: false, Sortable: true, Width: "80px"},
			{Label: "Name", Name: "name", Type: ColText, Searchable: true, Sortable: true},
			{Label: "Status", Name: "status", Type: ColBadge, Searchable: false, Sortable: false},
			{Label: "Created At", Name: "created_at", Type: ColDateTime, Searchable: false, Sortable: true},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cols)
}

func (e *Engine) handleAPIModule(w http.ResponseWriter, r *http.Request) {
	tbl := strings.TrimSpace(r.URL.Query().Get("table"))
	if tbl == "" {
		http.Error(w, `{"error":"Table parameter is required"}`, http.StatusBadRequest)
		return
	}

	ctrl := e.FindController(tbl)
	if ctrl == nil {
		http.Error(w, `{"error":"Module not found"}`, http.StatusNotFound)
		return
	}

	providerType := "memory"
	if _, ok := ctrl.DataProvider.(*SQLStore); ok {
		providerType = "sql"
	}

	def := ModuleDefinition{
		Title:        ctrl.Title,
		Table:        ctrl.Table,
		Icon:         ctrl.Icon,
		PrimaryKey:   ctrl.PrimaryKey,
		OrderBy:      ctrl.OrderBy,
		Columns:      ctrl.Columns,
		Forms:        ctrl.Forms,
		Widgets:      ctrl.Widgets,
		ProviderType: providerType,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(def)
}

func (e *Engine) handleSaveModule(w http.ResponseWriter, r *http.Request) {
	var def ModuleDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	def.Table = strings.TrimSpace(def.Table)
	def.Title = strings.TrimSpace(def.Title)
	if def.Table == "" || def.Title == "" {
		http.Error(w, `{"error":"Table name and title are required"}`, http.StatusBadRequest)
		return
	}

	if def.PrimaryKey == "" {
		def.PrimaryKey = "id"
	}
	if def.OrderBy == "" {
		def.OrderBy = def.PrimaryKey + " DESC"
	}
	if def.Icon == "" {
		def.Icon = `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>`
	}

	// 1. Build or update Controller
	ctrl := &Controller{
		Title:      def.Title,
		Table:      def.Table,
		Icon:       def.Icon,
		PrimaryKey: def.PrimaryKey,
		OrderBy:    def.OrderBy,
		Columns:    def.Columns,
		Forms:      def.Forms,
		Widgets:    def.Widgets,
	}

	// 2. Assign DataProvider
	if def.ProviderType == "sql" && e.FindDB() != nil {
		ctrl.DataProvider = NewSQLStore(e.FindDB(), def.Table, def.PrimaryKey)
	} else {
		sampleRows := generateSampleRows(def)
		ctrl.DataProvider = NewMemoryStore(sampleRows...)
	}

	// 3. Register dynamically into the running server and menus
	e.RegisterDynamic(ctrl)

	// 4. Generate Go Controller code
	code := e.GenerateControllerCode(def)

	// 5. Write Go file to disk if controllers directory exists
	targetDirs := []string{
		"starter/app/controllers",
		"app/controllers",
		"controllers",
		"../starter/app/controllers",
	}
	for _, dir := range targetDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			filename := filepath.Join(dir, fmt.Sprintf("admin_%s_controller.go", def.Table))
			_ = os.WriteFile(filename, []byte(code), 0644)
			break
		}
	}

	// 6. Save persistent JSON definition
	_ = e.SaveModuleDefinition(def)

	// Return success JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Module '%s' deployed successfully!", def.Title),
		"path":    ctrl.BasePath,
	})
}

func (e *Engine) handleDeleteModule(w http.ResponseWriter, r *http.Request) {
	tbl := strings.TrimSpace(r.URL.Query().Get("table"))
	if tbl == "" {
		http.Error(w, `{"error":"Table parameter is required"}`, http.StatusBadRequest)
		return
	}

	e.Unregister(tbl)
	_ = e.DeleteModuleDefinition(tbl)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Module '%s' removed successfully", tbl),
	})
}

func (e *Engine) handleModuleStudioView(w http.ResponseWriter, r *http.Request) {
	var dbTables []string
	if db := e.FindDB(); db != nil {
		rows, err := db.Query("SHOW TABLES")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var t string
				if err := rows.Scan(&t); err == nil && t != "" {
					dbTables = append(dbTables, t)
				}
			}
		} else {
			sRows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'cb_%'")
			if err == nil {
				defer sRows.Close()
				for sRows.Next() {
					var t string
					if err := sRows.Scan(&t); err == nil && t != "" {
						dbTables = append(dbTables, t)
					}
				}
			}
		}
	}

	data := map[string]interface{}{
		"AdminPath":   e.AdminPath,
		"Controllers": e.controllers,
		"HasDB":       e.FindDB() != nil,
		"DBTables":    dbTables,
	}

	contentHTML, err := RenderModuleGeneratorContent(data)
	if err != nil {
		http.Error(w, "Failed to render Module Studio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	e.RenderLayout(w, r, "Module Studio Generator", contentHTML)
}

// GenerateControllerCode outputs production-ready Go code for a CRUD controller ala CRUDBooster
func (e *Engine) GenerateControllerCode(def ModuleDefinition) string {
	pascal := strings.ReplaceAll(def.Title, " ", "")
	pascal = strings.ReplaceAll(pascal, "&", "")
	pascal = strings.ReplaceAll(pascal, "-", "")
	pascal = strings.ReplaceAll(pascal, "_", "")
	if pascal == "" {
		pascal = "Module"
	}

	pk := def.PrimaryKey
	if pk == "" {
		pk = "id"
	}

	orderBy := def.OrderBy
	if orderBy == "" {
		orderBy = pk + " DESC"
	}

	var sb strings.Builder
	sb.WriteString("package controllers\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\tbooster \"github.com/tokalink/tgo-booster\"\n")
	if def.ProviderType == "sql" {
		sb.WriteString("\t\"github.com/tokalink/tgo-booster/starter/app/models\"\n")
	}
	sb.WriteString(")\n\n")

	sb.WriteString(fmt.Sprintf("// NewAdmin%sController builds the %s resource controller\n", pascal, def.Title))
	sb.WriteString(fmt.Sprintf("func NewAdmin%sController() *booster.Controller {\n", pascal))
	sb.WriteString("\tctrl := &booster.Controller{\n")
	sb.WriteString(fmt.Sprintf("\t\tTitle:      \"%s\",\n", def.Title))
	sb.WriteString(fmt.Sprintf("\t\tTable:      \"%s\",\n", def.Table))
	sb.WriteString(fmt.Sprintf("\t\tIcon:       `%s`,\n", def.Icon))
	sb.WriteString(fmt.Sprintf("\t\tPrimaryKey: \"%s\",\n", pk))
	sb.WriteString(fmt.Sprintf("\t\tOrderBy:    \"%s\",\n", orderBy))
	if def.ProviderType == "sql" {
		sb.WriteString(fmt.Sprintf("\t\tDataProvider: booster.NewSQLStore(models.DB, \"%s\", \"%s\"),\n", def.Table, pk))
	} else {
		sb.WriteString("\t\tDataProvider: booster.NewMemoryStore(),\n")
	}
	sb.WriteString("\t}\n\n")

	// Columns
	sb.WriteString("\t// 1. Data Grid Columns (CRUDBooster Table Display)\n")
	sb.WriteString("\tctrl.\n")
	for i, col := range def.Columns {
		colConst := "booster.ColText"
		switch col.Type {
		case ColImage:
			colConst = "booster.ColImage"
		case ColNumber:
			colConst = "booster.ColNumber"
		case ColMoney:
			colConst = "booster.ColMoney"
		case ColBadge:
			colConst = "booster.ColBadge"
		case ColDateTime:
			colConst = "booster.ColDateTime"
		case ColDate:
			colConst = "booster.ColDate"
		case ColEmail:
			colConst = "booster.ColEmail"
		case ColURL:
			colConst = "booster.ColURL"
		}
		term := "."
		if i == len(def.Columns)-1 {
			term = ""
		}
		sb.WriteString(fmt.Sprintf("\t\tAddCol(\"%s\", \"%s\", %s, %t, %t)%s\n", col.Label, col.Name, colConst, col.Searchable, col.Sortable, term))
	}
	sb.WriteString("\n")

	// Forms
	sb.WriteString("\t// 2. Form Input Fields (CRUDBooster Form Display)\n")
	sb.WriteString("\tctrl.\n")
	for i, f := range def.Forms {
		term := "."
		if i == len(def.Forms)-1 {
			term = ""
		}

		switch f.Type {
		case InputSelect, InputRadio:
			if f.DataTable != "" {
				sb.WriteString(fmt.Sprintf("\t\tAddSelectTable(\"%s\", \"%s\", \"%s\", %t, \"%s\")%s\n", f.Label, f.Name, f.DataTable, f.Required, f.Placeholder, term))
			} else if len(f.Options) > 0 {
				sb.WriteString(fmt.Sprintf("\t\tAddSelect(\"%s\", \"%s\", %t, \"%s\",\n", f.Label, f.Name, f.Required, f.Placeholder))
				for _, opt := range f.Options {
					sb.WriteString(fmt.Sprintf("\t\t\tbooster.Option{Value: \"%s\", Label: \"%s\"},\n", opt.Value, opt.Label))
				}
				sb.WriteString(fmt.Sprintf("\t\t)%s\n", term))
			} else {
				sb.WriteString(fmt.Sprintf("\t\tAddForm(\"%s\", \"%s\", booster.InputSelect, %t, \"%s\")%s\n", f.Label, f.Name, f.Required, f.Placeholder, term))
			}
		case InputLOV:
			if f.Format != "" {
				sb.WriteString(fmt.Sprintf("\t\tAddLOVFormatted(\"%s\", \"%s\", \"%s\", \"%s\", %t, \"%s\")%s\n", f.Label, f.Name, f.DataTable, f.Format, f.Required, f.Placeholder, term))
			} else {
				sb.WriteString(fmt.Sprintf("\t\tAddLOV(\"%s\", \"%s\", \"%s\", %t, \"%s\")%s\n", f.Label, f.Name, f.DataTable, f.Required, f.Placeholder, term))
			}
		default:
			inputConst := "booster.InputText"
			switch f.Type {
			case InputNumber:
				inputConst = "booster.InputNumber"
			case InputMoney:
				inputConst = "booster.InputMoney"
			case InputEmail:
				inputConst = "booster.InputEmail"
			case InputPassword:
				inputConst = "booster.InputPassword"
			case InputTextarea:
				inputConst = "booster.InputTextarea"
			case InputWYSIWYG:
				inputConst = "booster.InputWYSIWYG"
			case InputUpload:
				inputConst = "booster.InputUpload"
			case InputDate:
				inputConst = "booster.InputDate"
			case InputDateTime:
				inputConst = "booster.InputDateTime"
			case InputCheckbox:
				inputConst = "booster.InputCheckbox"
			case InputHidden:
				inputConst = "booster.InputHidden"
			}
			sb.WriteString(fmt.Sprintf("\t\tAddForm(\"%s\", \"%s\", %s, %t, \"%s\")%s\n", f.Label, f.Name, inputConst, f.Required, f.Placeholder, term))
		}
	}
	sb.WriteString("\n")

	// Widgets
	if len(def.Widgets) > 0 {
		sb.WriteString("\t// 3. Executive Widgets (CRUDBooster Widget Card equivalent)\n")
		for _, w := range def.Widgets {
			sb.WriteString("\tctrl.AddWidget(booster.WidgetCard{\n")
			sb.WriteString(fmt.Sprintf("\t\tTitle:     \"%s\",\n", w.Title))
			sb.WriteString(fmt.Sprintf("\t\tValue:     \"%s\",\n", w.Value))
			if w.Subtext != "" {
				sb.WriteString(fmt.Sprintf("\t\tSubtext:   \"%s\",\n", w.Subtext))
			}
			if w.Trend != "" {
				sb.WriteString(fmt.Sprintf("\t\tTrend:     \"%s\",\n", w.Trend))
				sb.WriteString(fmt.Sprintf("\t\tTrendType: \"%s\",\n", w.TrendType))
			}
			if w.Color != "" {
				sb.WriteString(fmt.Sprintf("\t\tColor:     \"%s\",\n", w.Color))
			}
			if w.ColSpan > 0 {
				sb.WriteString(fmt.Sprintf("\t\tColSpan:   %d,\n", w.ColSpan))
			}
			sb.WriteString("\t})\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\treturn ctrl\n")
	sb.WriteString("}\n")

	return sb.String()
}

func formatFriendlyLabel(name string) string {
	if name == "id" {
		return "ID"
	}
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if strings.ToLower(p) == "id" {
			parts[i] = "ID"
		} else if strings.ToLower(p) == "url" {
			parts[i] = "URL"
		} else if len(p) > 0 {
			parts[i] = strings.ToUpper(string(p[0])) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

func inferColumnType(name, dbType string) ColumnType {
	nameLower := strings.ToLower(name)
	switch {
	case nameLower == "id":
		return ColText
	case strings.Contains(nameLower, "price") || strings.Contains(nameLower, "amount") || strings.Contains(nameLower, "cost") || strings.Contains(nameLower, "total") || strings.Contains(nameLower, "salary"):
		return ColMoney
	case strings.Contains(nameLower, "email"):
		return ColEmail
	case strings.Contains(nameLower, "image") || strings.Contains(nameLower, "photo") || strings.Contains(nameLower, "avatar") || strings.Contains(nameLower, "thumbnail") || strings.Contains(nameLower, "logo") || strings.Contains(nameLower, "file"):
		return ColImage
	case strings.Contains(nameLower, "status") || strings.Contains(nameLower, "state") || strings.Contains(nameLower, "type") || strings.Contains(dbType, "enum"):
		return ColBadge
	case strings.Contains(nameLower, "date") && !strings.Contains(nameLower, "time"):
		return ColDate
	case strings.Contains(nameLower, "time") || strings.Contains(nameLower, "created_at") || strings.Contains(nameLower, "updated_at") || strings.Contains(dbType, "timestamp") || strings.Contains(dbType, "datetime"):
		return ColDateTime
	case strings.Contains(nameLower, "url") || strings.Contains(nameLower, "link") || strings.Contains(nameLower, "website"):
		return ColURL
	case strings.Contains(nameLower, "stock") || strings.Contains(nameLower, "qty") || strings.Contains(nameLower, "quantity") || strings.Contains(nameLower, "count") || strings.Contains(dbType, "int") || strings.Contains(dbType, "numeric"):
		return ColNumber
	default:
		return ColText
	}
}

func generateSampleRows(def ModuleDefinition) []map[string]interface{} {
	rows := make([]map[string]interface{}, 3)
	for idx := 0; idx < 3; idx++ {
		row := make(map[string]interface{})
		row[def.PrimaryKey] = fmt.Sprint(idx + 1)
		for _, col := range def.Columns {
			if col.Name == def.PrimaryKey {
				continue
			}
			switch col.Type {
			case ColMoney:
				row[col.Name] = fmt.Sprintf("Rp %d.000", (idx+1)*125)
			case ColNumber:
				row[col.Name] = (idx + 1) * 15
			case ColBadge:
				if idx == 0 {
					row[col.Name] = "Active"
				} else if idx == 1 {
					row[col.Name] = "Pending"
				} else {
					row[col.Name] = "Completed"
				}
			case ColEmail:
				row[col.Name] = fmt.Sprintf("contact%d@%s.com", idx+1, def.Table)
			case ColImage:
				row[col.Name] = fmt.Sprintf("https://images.unsplash.com/photo-152%d?w=100", idx)
			case ColDate:
				row[col.Name] = fmt.Sprintf("2026-0%d-15", idx+1)
			case ColDateTime:
				row[col.Name] = fmt.Sprintf("2026-0%d-15 10:30:00", idx+1)
			default:
				row[col.Name] = fmt.Sprintf("%s Sample #%d", def.Title, idx+1)
			}
		}
		rows[idx] = row
	}
	return rows
}

func (e *Engine) getPrivilegesPersistencePath() string {
	candidates := []string{
		"data/privileges.json",
		"starter/data/privileges.json",
		"../starter/data/privileges.json",
	}
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return c
		}
	}
	return "data/privileges.json"
}

// LoadDynamicRoles restores role definitions and permissions matrix from privileges.json
func (e *Engine) LoadDynamicRoles() {
	p := e.getPrivilegesPersistencePath()
	data, err := os.ReadFile(p)
	if err == nil && len(data) > 0 {
		var roles []*Role
		if err := json.Unmarshal(data, &roles); err == nil && len(roles) > 0 {
			e.roles = roles
			return
		}
	}
	if len(e.roles) == 0 {
		e.initDefaultRoles()
	}
}

func (e *Engine) initDefaultRoles() {
	super := &Role{
		ID:           "1",
		Name:         "Super Administrator",
		Slug:         "superadmin",
		IsSuperadmin: true,
		Description:  "Unrestricted administrative privileges across all modules, matrix configurations, and settings.",
		UsersCount:   1,
		Permissions:  make(map[string]PermissionMatrix),
		CreatedAt:    time.Now(),
	}

	opsPerms := make(map[string]PermissionMatrix)
	for _, c := range e.controllers {
		opsPerms[c.Table] = PermissionMatrix{
			IsVisible: true,
			CanCreate: true,
			CanRead:   true,
			CanUpdate: true,
			CanDelete: false,
		}
	}
	opsPerms["users"] = PermissionMatrix{IsVisible: true, CanCreate: false, CanRead: true, CanUpdate: false, CanDelete: false}
	opsPerms["logs"] = PermissionMatrix{IsVisible: true, CanCreate: false, CanRead: true, CanUpdate: false, CanDelete: false}

	ops := &Role{
		ID:           "2",
		Name:         "Operations Manager",
		Slug:         "ops_manager",
		IsSuperadmin: false,
		Description:  "Day-to-day operations and catalog management. Full create, read, and update permissions.",
		UsersCount:   4,
		Permissions:  opsPerms,
		CreatedAt:    time.Now(),
	}

	auditorPerms := make(map[string]PermissionMatrix)
	for _, c := range e.controllers {
		auditorPerms[c.Table] = PermissionMatrix{
			IsVisible: true,
			CanCreate: false,
			CanRead:   true,
			CanUpdate: false,
			CanDelete: false,
		}
	}
	auditorPerms["logs"] = PermissionMatrix{IsVisible: true, CanCreate: false, CanRead: true, CanUpdate: false, CanDelete: false}

	auditor := &Role{
		ID:           "3",
		Name:         "Read-Only Auditor",
		Slug:         "auditor",
		IsSuperadmin: false,
		Description:  "Compliance and auditing read-only access. Cannot create, edit, or delete any data.",
		UsersCount:   2,
		Permissions:  auditorPerms,
		CreatedAt:    time.Now(),
	}

	e.roles = []*Role{super, ops, auditor}
	_ = e.SaveRoles()
}

// SaveRoles persists all configured roles and their RBAC matrix to disk
func (e *Engine) SaveRoles() error {
	p := e.getPrivilegesPersistencePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(e.roles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// FindRole retrieves a role by ID, Slug, or Name
func (e *Engine) FindRole(key string) *Role {
	if key == "" {
		return nil
	}
	for _, r := range e.roles {
		if r.ID == key || strings.EqualFold(r.Slug, key) || strings.EqualFold(r.Name, key) {
			return r
		}
	}
	return nil
}

// IsUserSuperadmin checks if the current user possesses unrestricted administrator status
func (e *Engine) IsUserSuperadmin(user *User) bool {
	if user == nil {
		return true
	}
	if role := e.FindRole(user.PrivilegeID); role != nil {
		return role.IsSuperadmin
	}
	if role := e.FindRole(user.RoleName); role != nil {
		return role.IsSuperadmin
	}
	if user.PrivilegeID == "1" ||
		strings.EqualFold(user.RoleName, "Super Admin") ||
		strings.EqualFold(user.RoleName, "Super Administrator") ||
		strings.EqualFold(user.RoleName, "Superadmin") ||
		strings.EqualFold(user.RoleName, "Admin") ||
		strings.EqualFold(user.RoleName, "Administrator") {
		return true
	}
	return false
}

// CheckPermission evaluates whether the active request session is authorized for an action on a module
func (e *Engine) CheckPermission(r *http.Request, moduleTable, action string) bool {
	if e.Auth == nil {
		return true
	}
	user := e.Auth.GetSessionUser(r)
	if user == nil {
		return true
	}
	if e.IsUserSuperadmin(user) {
		return true
	}

	// Protected builder engine tools are superadmin only
	if moduleTable == "privileges" || moduleTable == "module_generator" || moduleTable == "settings" {
		return false
	}

	role := e.FindRole(user.PrivilegeID)
	if role == nil {
		role = e.FindRole(user.RoleName)
	}
	if role == nil {
		return false
	}
	return role.CanAccess(moduleTable, action)
}

// RenderForbidden presents an executive CRUDBooster 403 Forbidden screen
func (e *Engine) RenderForbidden(w http.ResponseWriter, r *http.Request, moduleTitle, action string) {
	w.WriteHeader(http.StatusForbidden)
	user := e.Auth.GetSessionUser(r)
	roleName := "Unknown Role"
	if user != nil && user.RoleName != "" {
		roleName = user.RoleName
	}

	forbiddenHTML := fmt.Sprintf(`
		<div style="max-width: 680px; margin: 3rem auto; text-align: center; padding: 2.5rem; background: var(--bg-card); border-radius: var(--radius-lg); border: 1px solid var(--border); box-shadow: var(--shadow-xl);">
			<div style="display: inline-flex; align-items: center; justify-content: center; width: 64px; height: 64px; border-radius: 50%%; background: rgba(239, 68, 68, 0.15); color: var(--danger); margin-bottom: 1.5rem; border: 1px solid rgba(239, 68, 68, 0.3);">
				<svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
					<line x1="12" y1="8" x2="12" y2="12"/>
					<line x1="12" y1="16" x2="12.01" y2="16"/>
				</svg>
			</div>
			<div class="cb-badge cb-badge-danger" style="margin-bottom: 1rem; font-size: 0.8rem; letter-spacing: 0.05em;">HTTP 403 FORBIDDEN</div>
			<h2 style="font-size: 1.5rem; font-weight: 800; color: var(--text-main); margin-bottom: 0.5rem;">Access Denied</h2>
			<p style="color: var(--text-muted); font-size: 0.95rem; line-height: 1.6; margin-bottom: 1.5rem;">
				Your current active role <strong style="color: var(--accent);">[%s]</strong> does not have permission to 
				<strong style="color: var(--text-main); text-transform: uppercase;">%s</strong> on the <strong>%s</strong> module.
			</p>
			<div style="background: var(--bg-subtle); padding: 1rem; border-radius: var(--radius-md); border: 1px solid var(--border); margin-bottom: 1.75rem; text-align: left; font-size: 0.85rem; color: var(--text-dim);">
				<div style="font-weight: 600; color: var(--text-muted); margin-bottom: 4px;">CRUDBooster RBAC Matrix Check:</div>
				<div>• Required Action: <code>%s</code></div>
				<div>• Module Resource: <code>%s</code></div>
				<div>• Session Role: <code>%s</code></div>
			</div>
			<div style="display: flex; gap: 10px; justify-content: center;">
				<a href="%s" class="btn btn-primary" style="padding: 0.6rem 1.25rem;">
					<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m3 9 9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>
					<span>Return to Dashboard</span>
				</a>
				<a href="%s/privileges" class="btn btn-secondary" style="padding: 0.6rem 1.25rem;">
					<span>Privileges Studio</span>
				</a>
			</div>
		</div>
	`, roleName, action, moduleTitle, action, moduleTitle, roleName, e.AdminPath, e.AdminPath)

	e.RenderLayout(w, r, "403 Forbidden - Access Denied", template.HTML(forbiddenHTML))
}

// handlePrivileges manages the CRUDBooster Privileges Studio and RBAC API
func (e *Engine) handlePrivileges(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		user := e.Auth.GetSessionUser(r)
		if user == nil {
			http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
			return
		}
		if !e.IsUserSuperadmin(user) {
			e.RenderForbidden(w, r, "Privileges & Access Control", "Superadmin")
			return
		}
	}

	sub := strings.TrimPrefix(r.URL.Path, e.AdminPath+"/privileges")
	sub = strings.TrimPrefix(sub, "/")
	sub = strings.TrimSuffix(sub, "/")

	switch {
	case sub == "api/role":
		id := r.URL.Query().Get("id")
		role := e.FindRole(id)
		w.Header().Set("Content-Type", "application/json")
		if role == nil {
			http.Error(w, `{"error":"Role not found"}`, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(role)
		return

	case sub == "save" && r.Method == http.MethodPost:
		var req struct {
			ID           string                      `json:"id"`
			Name         string                      `json:"name"`
			Slug         string                      `json:"slug"`
			IsSuperadmin bool                        `json:"is_superadmin"`
			Description  string                      `json:"description"`
			Permissions  map[string]PermissionMatrix `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"Invalid request JSON: `+err.Error()+`"}`, http.StatusBadRequest)
			return
		}
		if req.Name == "" || req.Slug == "" {
			http.Error(w, `{"error":"Role title and slug are required"}`, http.StatusBadRequest)
			return
		}

		var target *Role
		if req.ID != "" {
			target = e.FindRole(req.ID)
		}
		if target == nil && req.Slug != "" {
			target = e.FindRole(req.Slug)
		}
		if target == nil {
			target = &Role{
				ID:        fmt.Sprintf("%d", len(e.roles)+1),
				CreatedAt: time.Now(),
			}
			e.roles = append(e.roles, target)
		}

		target.Name = req.Name
		target.Slug = req.Slug
		target.IsSuperadmin = req.IsSuperadmin
		target.Description = req.Description
		if req.Permissions != nil {
			target.Permissions = req.Permissions
		} else if target.Permissions == nil {
			target.Permissions = make(map[string]PermissionMatrix)
		}

		if err := e.SaveRoles(); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Failed to save role: %s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		e.LogAudit(r, "ROLE_UPDATE", "Privileges", fmt.Sprintf("Saved RBAC role '%s' (Slug: %s, Superadmin: %v)", target.Name, target.Slug, target.IsSuperadmin))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"role":    target,
		})
		return

	case sub == "delete" && r.Method == http.MethodPost:
		id := r.URL.Query().Get("id")
		target := e.FindRole(id)
		if target == nil {
			http.Error(w, `{"error":"Role not found"}`, http.StatusNotFound)
			return
		}
		if target.IsSuperadmin || strings.EqualFold(target.Slug, "superadmin") {
			http.Error(w, `{"error":"Cannot delete superadmin role"}`, http.StatusBadRequest)
			return
		}

		for i, r := range e.roles {
			if r.ID == target.ID {
				e.roles = append(e.roles[:i], e.roles[i+1:]...)
				break
			}
		}
		_ = e.SaveRoles()
		e.LogAudit(r, "ROLE_DELETE", "Privileges", fmt.Sprintf("Deleted RBAC role '%s' (ID: %s)", target.Name, target.ID))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
		return

	case sub == "switch" && r.Method == http.MethodPost:
		slug := r.URL.Query().Get("role")
		target := e.FindRole(slug)
		if target == nil {
			http.Error(w, `{"error":"Target role not found"}`, http.StatusNotFound)
			return
		}

		user := e.Auth.GetSessionUser(r)
		if user == nil {
			user = &User{ID: "1", Name: "Admin User", Email: "admin@tgo.io"}
		}
		user.PrivilegeID = target.ID
		user.RoleName = target.Name
		e.Auth.SetSessionUser(w, user)
		e.LogAudit(r, "ROLE_SWITCH", "Privileges", fmt.Sprintf("User switched active session role to '%s'", target.Name))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"role_name": target.Name,
			"role_slug": target.Slug,
		})
		return

	case sub == "":
		user := e.Auth.GetSessionUser(r)
		currentRole := "Super Administrator"
		if user != nil && user.RoleName != "" {
			currentRole = user.RoleName
		}

		type ModuleItem struct {
			Table    string
			Title    string
			Icon     string
			BasePath string
		}
		var modules []ModuleItem
		for _, c := range e.controllers {
			modules = append(modules, ModuleItem{
				Table:    c.Table,
				Title:    c.Title,
				Icon:     c.Icon,
				BasePath: c.BasePath,
			})
		}
		for _, s := range e.systemCtrls {
			modules = append(modules, ModuleItem{
				Table:    s.Table,
				Title:    s.Title,
				Icon:     s.Icon,
				BasePath: s.BasePath,
			})
		}

		data := map[string]interface{}{
			"AdminPath":   e.AdminPath,
			"Roles":       e.roles,
			"Modules":     modules,
			"CurrentRole": currentRole,
		}

		contentHTML, err := RenderPrivilegesContent(data)
		if err != nil {
			http.Error(w, "Failed to render privileges studio: "+err.Error(), http.StatusInternalServerError)
			return
		}

		e.RenderLayout(w, r, "Privileges & Roles Management", contentHTML)
		return

	default:
		http.NotFound(w, r)
	}
}

// ============================================================================
// MENU MANAGEMENT STUDIO & HIERARCHICAL NAVIGATION ENGINE
// ============================================================================

func (e *Engine) getMenusPersistencePath() string {
	candidates := []string{
		"data/menus.json",
		"starter/data/menus.json",
		"../starter/data/menus.json",
	}
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return c
		}
	}
	return "data/menus.json"
}

// SaveMenus persists the active hierarchical menus tree to disk
func (e *Engine) SaveMenus() error {
	p := e.getMenusPersistencePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(e.menus, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// LoadDynamicMenus restores the navigation hierarchy from menus.json
func (e *Engine) LoadDynamicMenus() {
	p := e.getMenusPersistencePath()
	data, err := os.ReadFile(p)
	if err == nil && len(data) > 0 {
		var items []*MenuItem
		if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
			e.menus = items
			// Ensure any controller registered in code is present in menus
			for _, c := range e.controllers {
				if item, _ := e.findMenuItem(c.Table); item == nil {
					e.syncControllerMenu(c)
				}
			}
			return
		}
	}

	// Initialize default menu tree if missing or empty
	if len(e.menus) == 0 {
		e.initDefaultMenus()
		_ = e.SaveMenus()
	}
}

// initDefaultMenus generates a rich initial menu tree with headers, modules, and sub-menus
func (e *Engine) initDefaultMenus() {
	menus := make([]*MenuItem, 0)
	order := 1

	// 1. Group: Platform
	platformGroup := &MenuItem{
		ID:    "group_platform",
		Title: "Platform",
		Type:  "header",
		Order: order,
		Children: []*MenuItem{
			{
				ID:       "dashboard",
				Title:    "Executive Dashboard",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect width="7" height="9" x="3" y="3" rx="1"/><rect width="7" height="5" x="14" y="3" rx="1"/><rect width="7" height="9" x="14" y="12" rx="1"/><rect width="7" height="5" x="3" y="16" rx="1"/></svg>`,
				Path:     e.AdminPath,
				ParentID: "group_platform",
				Order:    1,
			},
		},
	}
	menus = append(menus, platformGroup)
	order++

	// 2. Group: Business Modules
	businessChildren := make([]*MenuItem, 0)
	bOrder := 1
	for _, c := range e.controllers {
		businessChildren = append(businessChildren, &MenuItem{
			ID:         c.Table,
			Title:      c.Title,
			Type:       "module",
			Icon:       c.Icon,
			Path:       c.BasePath,
			ParentID:   "group_business",
			BadgeColor: "primary",
			Order:      bOrder,
		})
		bOrder++
	}

	businessGroup := &MenuItem{
		ID:       "group_business",
		Title:    "Business Modules",
		Type:     "header",
		Order:    order,
		Children: businessChildren,
	}
	menus = append(menus, businessGroup)
	order++

	// 3. Group: Engine Tools
	toolsGroup := &MenuItem{
		ID:    "group_tools",
		Title: "Engine Tools",
		Type:  "header",
		Order: order,
		Children: []*MenuItem{
			{
				ID:         "module_generator",
				Title:      "Module Generator",
				Type:       "url",
				Icon:       `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M13 2 3 14h9l-1 8 10-12h-9l1-8z"/></svg>`,
				Path:       e.AdminPath + "/module_generator",
				ParentID:   "group_tools",
				Badge:      "PRO",
				BadgeColor: "purple",
				Roles:      []string{"superadmin"},
				Order:      1,
			},
			{
				ID:       "menu_management",
				Title:    "Menu Management",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="20" y1="6" y2="6"/><line x1="4" x2="20" y1="12" y2="12"/><line x1="4" x2="20" y1="18" y2="18"/></svg>`,
				Path:     e.AdminPath + "/menus",
				ParentID: "group_tools",
				Roles:    []string{"superadmin"},
				Order:      2,
			},
			{
				ID:       "privileges",
				Title:    "Privileges & Roles",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/></svg>`,
				Path:     e.AdminPath + "/privileges",
				ParentID: "group_tools",
				Roles:    []string{"superadmin"},
				Order:    3,
			},
			{
				ID:       "pages_studio",
				Title:    "Pages & SEO",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`,
				Path:     e.AdminPath + "/pages",
				ParentID: "group_tools",
				Roles:    []string{"superadmin"},
				Order:    4,
			},
		},
	}
	menus = append(menus, toolsGroup)
	order++

	// System Management Dropdown with Sub-menus!
	systemDropdown := &MenuItem{
		ID:    "system_dropdown",
		Title: "Administration",
		Type:  "dropdown",
		Icon:  `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><line x1="4" x2="4" y1="21" y2="14"/><line x1="4" x2="4" y1="10" y2="3"/><line x1="12" x2="12" y1="21" y2="12"/><line x1="12" x2="12" y1="8" y2="3"/><line x1="20" x2="20" y1="21" y2="16"/><line x1="20" x2="20" y1="12" y2="3"/><line x1="1" x2="7" y1="14" y2="14"/><line x1="9" x2="15" y1="8" y2="8"/><line x1="17" x2="23" y1="16" y2="16"/></svg>`,
		Order: order,
		Children: []*MenuItem{
			{
				ID:       "users",
				Title:    "User Accounts",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="5"/><path d="M20 21a8 8 0 0 0-16 0"/></svg>`,
				Path:     e.AdminPath + "/users",
				ParentID: "system_dropdown",
				Order:    1,
			},
			{
				ID:       "api_generator",
				Title:    "API & Tokens",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m15.5 7.5 2.3 2.3a1 1 0 0 0 1.4 0l2.1-2.1a1 1 0 0 0 0-1.4L19 4"/><path d="m21 2-9.6 9.6"/><circle cx="7.5" cy="15.5" r="5.5"/></svg>`,
				Path:     e.AdminPath + "/api_generator",
				ParentID: "system_dropdown",
				Order:    2,
			},
			{
				ID:       "email_templates",
				Title:    "Email Templates",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="16" x="2" y="4" rx="2"/><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7"/></svg>`,
				Path:     e.AdminPath + "/email_templates",
				ParentID: "system_dropdown",
				Order:    3,
			},
			{
				ID:       "logs",
				Title:    "Audit Trail",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" x2="8" y1="13" x2="13"/><line x1="16" x2="8" y1="17" y2="17"/></svg>`,
				Path:     e.AdminPath + "/logs",
				ParentID: "system_dropdown",
				Order:    4,
			},
			{
				ID:       "settings",
				Title:    "Settings",
				Type:     "url",
				Icon:     `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>`,
				Path:     e.AdminPath + "/settings",
				ParentID: "system_dropdown",
				Roles:    []string{"superadmin"},
				Order:    5,
			},
		},
	}
	menus = append(menus, systemDropdown)

	e.menus = menus
}

func (e *Engine) syncControllerMenu(c *Controller) {
	if len(e.menus) == 0 {
		e.LoadDynamicMenus()
	}
	// If already in menus, update metadata
	item, _ := e.findMenuItem(c.Table)
	if item != nil {
		item.Title = c.Title
		item.Icon = c.Icon
		item.Path = c.BasePath
		return
	}

	newItem := &MenuItem{
		ID:         c.Table,
		Title:      c.Title,
		Type:       "module",
		Icon:       c.Icon,
		Path:       c.BasePath,
		BadgeColor: "primary",
	}

	// Try to place under the "group_business" section header if present
	for _, m := range e.menus {
		if m.ID == "group_business" && m.Type == "header" {
			newItem.ParentID = "group_business"
			newItem.Order = len(m.Children) + 1
			m.Children = append(m.Children, newItem)
			_ = e.SaveMenus()
			return
		}
	}

	newItem.Order = len(e.menus) + 1
	e.menus = append(e.menus, newItem)
	_ = e.SaveMenus()
}

func (e *Engine) findMenuItem(id string) (*MenuItem, *MenuItem) {
	for _, m := range e.menus {
		if m.ID == id {
			return m, nil
		}
		for _, c := range m.Children {
			if c.ID == id {
				return c, m
			}
			for _, gc := range c.Children {
				if gc.ID == id {
					return gc, c
				}
			}
		}
	}
	return nil, nil
}

func (e *Engine) deleteMenuItem(id string) bool {
	for i, m := range e.menus {
		if m.ID == id {
			e.menus = append(e.menus[:i], e.menus[i+1:]...)
			_ = e.SaveMenus()
			return true
		}
		for j, c := range m.Children {
			if c.ID == id {
				m.Children = append(m.Children[:j], m.Children[j+1:]...)
				_ = e.SaveMenus()
				return true
			}
			for k, gc := range c.Children {
				if gc.ID == id {
					c.Children = append(c.Children[:k], c.Children[k+1:]...)
					_ = e.SaveMenus()
					return true
				}
			}
		}
	}
	return false
}

func (e *Engine) upsertMenuItem(item *MenuItem) error {
	if item.Title == "" {
		return fmt.Errorf("menu title is required")
	}
	if item.Type == "" {
		item.Type = "module"
	}
	if item.ID == "" {
		slug := strings.ToLower(strings.ReplaceAll(item.Title, " ", "_"))
		reg := regexp.MustCompile(`[^a-z0-9_]+`)
		slug = reg.ReplaceAllString(slug, "")
		if slug == "" {
			slug = fmt.Sprintf("menu_%d", time.Now().Unix())
		}
		if existing, _ := e.findMenuItem(slug); existing != nil {
			slug = fmt.Sprintf("%s_%d", slug, time.Now().Unix()%1000)
		}
		item.ID = slug
	}

	oldItem, oldParent := e.findMenuItem(item.ID)
	if oldItem != nil {
		oldParentID := ""
		if oldParent != nil {
			oldParentID = oldParent.ID
		}
		// If parent unchanged, update in-place without losing position or order
		if item.ParentID == oldParentID {
			oldItem.Title = item.Title
			oldItem.Type = item.Type
			oldItem.Path = item.Path
			oldItem.Target = item.Target
			oldItem.Icon = item.Icon
			oldItem.Badge = item.Badge
			oldItem.BadgeColor = item.BadgeColor
			oldItem.Roles = item.Roles
			oldItem.ParentID = item.ParentID
			return e.SaveMenus()
		}
		// Parent changed: remove from old location first
		e.deleteMenuItem(item.ID)
	}

	// If parent is set, attach as child
	if item.ParentID != "" {
		parent, _ := e.findMenuItem(item.ParentID)
		if parent != nil {
			item.ParentID = parent.ID
			item.Order = len(parent.Children) + 1
			parent.Children = append(parent.Children, item)
			return e.SaveMenus()
		}
	}

	// Otherwise, add to root
	item.ParentID = ""
	item.Order = len(e.menus) + 1
	e.menus = append(e.menus, item)
	return e.SaveMenus()
}

func (e *Engine) reorderMenuItem(id string, direction string) error {
	for i, m := range e.menus {
		if m.ID == id {
			if direction == "up" && i > 0 {
				e.menus[i], e.menus[i-1] = e.menus[i-1], e.menus[i]
				e.menus[i].Order, e.menus[i-1].Order = i+1, i
				return e.SaveMenus()
			} else if direction == "down" && i < len(e.menus)-1 {
				e.menus[i], e.menus[i+1] = e.menus[i+1], e.menus[i]
				e.menus[i].Order, e.menus[i+1].Order = i+1, i+2
				return e.SaveMenus()
			}
			return nil
		}
		for j, c := range m.Children {
			if c.ID == id {
				if direction == "up" && j > 0 {
					m.Children[j], m.Children[j-1] = m.Children[j-1], m.Children[j]
					m.Children[j].Order, m.Children[j-1].Order = j+1, j
					return e.SaveMenus()
				} else if direction == "down" && j < len(m.Children)-1 {
					m.Children[j], m.Children[j+1] = m.Children[j+1], m.Children[j]
					m.Children[j].Order, m.Children[j+1].Order = j+1, j+2
					return e.SaveMenus()
				}
				return nil
			}
		}
	}
	return fmt.Errorf("menu item not found: %s", id)
}

func (e *Engine) getMenuStats() map[string]int {
	total := 0
	groups := 0
	dropdowns := 0
	submenus := 0
	modules := 0

	for _, m := range e.menus {
		total++
		switch m.Type {
		case "header":
			groups++
		case "dropdown":
			dropdowns++
			for _, c := range m.Children {
				total++
				submenus++
				if c.Type == "module" {
					modules++
				}
			}
		case "module":
			modules++
		}
	}

	return map[string]int{
		"TotalMenus":     total,
		"TotalGroups":    groups,
		"TotalDropdowns": dropdowns,
		"TotalSubmenus":  submenus,
		"TotalModules":   modules,
	}
}

func (e *Engine) buildActiveMenus(r *http.Request, currentPath string, user *User) []*MenuItem {
	isSuper := e.IsUserSuperadmin(user)
	roleSlug := ""
	if user != nil {
		roleSlug = user.RoleName
		if rObj := e.FindRole(user.PrivilegeID); rObj != nil {
			roleSlug = rObj.Slug
		}
	}

	isItemPermitted := func(m *MenuItem) bool {
		if isSuper {
			return true
		}
		if len(m.Roles) > 0 {
			permitted := false
			for _, rName := range m.Roles {
				if rName == "*" || strings.EqualFold(rName, roleSlug) || (user != nil && strings.EqualFold(rName, user.RoleName)) {
					permitted = true
					break
				}
			}
			if !permitted {
				return false
			}
		}
		if m.Type == "module" {
			return e.CheckPermission(r, m.ID, "visible")
		}
		return true
	}

	result := make([]*MenuItem, 0, len(e.menus))

	for _, m := range e.menus {
		if !isItemPermitted(m) {
			continue
		}

		clone := *m
		clone.Children = nil

		if m.Type == "dropdown" {
			filteredChildren := make([]*MenuItem, 0, len(m.Children))
			childActive := false
			for _, child := range m.Children {
				if isItemPermitted(child) {
					childClone := *child
					if childClone.Path != "" && (currentPath == childClone.Path || (childClone.Path != e.AdminPath && strings.HasPrefix(currentPath, childClone.Path+"/"))) {
						childClone.IsActive = true
						childActive = true
					}
					filteredChildren = append(filteredChildren, &childClone)
				}
			}
			if len(filteredChildren) == 0 && !isSuper {
				continue
			}
			clone.Children = filteredChildren
			if childActive {
				clone.IsOpen = true
				clone.IsActive = true
			}
		} else if m.Type == "header" {
			filteredChildren := make([]*MenuItem, 0, len(m.Children))
			for _, child := range m.Children {
				if isItemPermitted(child) {
					childClone := *child
					if childClone.Type == "dropdown" {
						subChildren := make([]*MenuItem, 0, len(childClone.Children))
						subActive := false
						for _, sc := range childClone.Children {
							if isItemPermitted(sc) {
								scClone := *sc
								if scClone.Path != "" && (currentPath == scClone.Path || (scClone.Path != e.AdminPath && strings.HasPrefix(currentPath, scClone.Path+"/"))) {
									scClone.IsActive = true
									subActive = true
								}
								subChildren = append(subChildren, &scClone)
							}
						}
						childClone.Children = subChildren
						if subActive {
							childClone.IsOpen = true
							childClone.IsActive = true
						}
					} else if childClone.Path != "" && (currentPath == childClone.Path || (childClone.Path != e.AdminPath && strings.HasPrefix(currentPath, childClone.Path+"/"))) {
						childClone.IsActive = true
					}
					filteredChildren = append(filteredChildren, &childClone)
				}
			}
			clone.Children = filteredChildren
		} else {
			if m.Path != "" && (currentPath == m.Path || (m.Path != e.AdminPath && strings.HasPrefix(currentPath, m.Path+"/"))) {
				clone.IsActive = true
			}
		}

		result = append(result, &clone)
	}

	return result
}

func (e *Engine) handleMenus(w http.ResponseWriter, r *http.Request) {
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		e.RenderForbidden(w, r, "Menu Management", "Superadmin")
		return
	}

	var containers []*MenuItem
	var collectContainers func(items []*MenuItem, prefix string)
	collectContainers = func(items []*MenuItem, prefix string) {
		for _, it := range items {
			if it.Type == "header" || it.Type == "dropdown" {
				c := *it
				if prefix != "" {
					c.Title = prefix + " > " + it.Title
				}
				containers = append(containers, &c)
				if len(it.Children) > 0 {
					collectContainers(it.Children, c.Title)
				}
			}
		}
	}
	collectContainers(e.menus, "")

	data := map[string]interface{}{
		"AdminPath":   e.AdminPath,
		"Menus":       e.menus,
		"Containers":  containers,
		"Controllers": e.controllers,
		"Roles":       e.roles,
		"Stats":       e.getMenuStats(),
	}

	contentHTML, err := RenderMenusContent(data)
	if err != nil {
		http.Error(w, "Failed to render menu studio: "+err.Error(), http.StatusInternalServerError)
		return
	}

	e.RenderLayout(w, r, "Menu Management Studio", contentHTML)
}

func (e *Engine) handleMenuAPI(w http.ResponseWriter, r *http.Request) {
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := r.URL.Query().Get("id")
	item, parent := e.findMenuItem(id)
	if item == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "menu not found"})
		return
	}

	respItem := *item
	if respItem.ParentID == "" && parent != nil {
		respItem.ParentID = parent.ID
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&respItem)
}

func (e *Engine) handleMenuSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Superadmin privilege required"})
		return
	}

	var item MenuItem
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON: " + err.Error()})
			return
		}
	} else {
		_ = r.ParseForm()
		order := 0
		fmt.Sscanf(r.FormValue("order"), "%d", &order)
		item = MenuItem{
			ID:         r.FormValue("id"),
			Title:      r.FormValue("title"),
			Type:       r.FormValue("type"),
			Path:       r.FormValue("path"),
			Target:     r.FormValue("target"),
			ParentID:   r.FormValue("parent_id"),
			Icon:       r.FormValue("icon"),
			Badge:      r.FormValue("badge"),
			BadgeColor: r.FormValue("badge_color"),
			Order:      order,
			Roles:      r.Form["roles"],
		}
	}

	if err := e.upsertMenuItem(&item); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	e.LogAudit(r, "MENU_UPDATE", "Menus", fmt.Sprintf("Saved sidebar menu item '%s' (Type: %s, ID: %s)", item.Title, item.Type, item.ID))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Menu item saved successfully",
		"item":    item,
	})
}

func (e *Engine) handleMenuDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Superadmin privilege required"})
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		id = r.FormValue("id")
	}
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Menu ID is required"})
		return
	}

	if !e.deleteMenuItem(id) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Menu item not found"})
		return
	}

	e.LogAudit(r, "MENU_DELETE", "Menus", fmt.Sprintf("Deleted sidebar menu item ID: %s", id))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Menu item deleted successfully"})
}

func (e *Engine) handleMenuReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Superadmin privilege required"})
		return
	}

	id := r.URL.Query().Get("id")
	direction := r.URL.Query().Get("direction")
	if id == "" {
		id = r.FormValue("id")
		direction = r.FormValue("direction")
	}

	if err := e.reorderMenuItem(id, direction); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": err.Error()})
		return
	}

	e.LogAudit(r, "MENU_REORDER", "Menus", fmt.Sprintf("Reordered sidebar menu item '%s' (Direction: %s)", id, direction))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Menu reordered successfully"})
}

type MenuReorderTreeItem struct {
	ID       string                `json:"id"`
	ParentID string                `json:"parent_id"`
	Order    int                   `json:"order"`
	Children []MenuReorderTreeItem `json:"children"`
}

type MenuReorderTreeRequest struct {
	Tree []MenuReorderTreeItem `json:"tree"`
}

func (e *Engine) handleMenuReorderTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Superadmin privilege required"})
		return
	}

	var req MenuReorderTreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid JSON: " + err.Error()})
		return
	}

	// Flatten all existing items to a lookup map
	itemsMap := make(map[string]*MenuItem)
	var collectItems func(list []*MenuItem)
	collectItems = func(list []*MenuItem) {
		for _, m := range list {
			itemsMap[m.ID] = m
			if len(m.Children) > 0 {
				collectItems(m.Children)
			}
		}
	}
	collectItems(e.menus)

	// Rebuild tree from request
	newMenus := make([]*MenuItem, 0, len(req.Tree))
	usedIDs := make(map[string]bool)

	for i, rootNode := range req.Tree {
		item, exists := itemsMap[rootNode.ID]
		if !exists {
			continue
		}
		usedIDs[item.ID] = true
		item.Order = i + 1
		item.ParentID = ""

		// Rebuild children
		childItems := make([]*MenuItem, 0, len(rootNode.Children))
		for j, childNode := range rootNode.Children {
			child, cExists := itemsMap[childNode.ID]
			if !cExists {
				continue
			}
			usedIDs[child.ID] = true
			child.Order = j + 1
			child.ParentID = item.ID

			// Handle grandchildren if any
			if len(childNode.Children) > 0 {
				gcItems := make([]*MenuItem, 0, len(childNode.Children))
				for k, gcNode := range childNode.Children {
					gc, gcExists := itemsMap[gcNode.ID]
					if !gcExists {
						continue
					}
					usedIDs[gc.ID] = true
					gc.Order = k + 1
					gc.ParentID = child.ID
					gcItems = append(gcItems, gc)
				}
				child.Children = gcItems
			} else {
				child.Children = nil
			}

			childItems = append(childItems, child)
		}
		item.Children = childItems
		newMenus = append(newMenus, item)
	}

	// Append any existing items that were not in the payload as a fallback
	for _, m := range e.menus {
		if !usedIDs[m.ID] {
			newMenus = append(newMenus, m)
		}
	}

	e.menus = newMenus
	if err := e.SaveMenus(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Failed to save: " + err.Error()})
		return
	}

	e.LogAudit(r, "MENU_REORDER", "Menus", "Reordered menu hierarchy tree")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Menu hierarchy and ordering saved successfully",
	})
}

func (e *Engine) handleMenuReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := e.Auth.GetSessionUser(r)
	if !e.IsUserSuperadmin(user) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Superadmin privilege required"})
		return
	}

	e.initDefaultMenus()
	_ = e.SaveMenus()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Menus reset to default configuration"})
}



