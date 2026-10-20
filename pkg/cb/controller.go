package cb

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tokalink/tgo/pkg/transport/middleware"
)

// Controller represents a declarative admin resource controller
type Controller struct {
	Engine       *Engine
	Title        string
	Table        string
	Icon         string
	PrimaryKey   string
	OrderBy      string
	Columns      []Column
	Forms        []Field
	BasePath     string
	DataProvider DataProvider

	// Custom HTML Injections (like CRUDBooster pre/post HTML)
	PreIndexHTML  template.HTML
	PostIndexHTML template.HTML
	PreFormHTML   template.HTML
	PostFormHTML  template.HTML

	// Executive Widgets, Actions & Tabs
	Widgets        []WidgetCard
	TopActions     []TopAction
	RowActions     []RowAction
	FilterTabs     []FilterTab
	WidgetProvider func(r *http.Request) []WidgetCard

	// Hooks
	HookBeforeAdd    HookFunc
	HookAfterAdd     HookFunc
	HookBeforeEdit   HookFunc
	HookAfterEdit    HookFunc
	HookBeforeDelete HookFunc
	HookAfterDelete  HookFunc
	HookRowListing   func(row map[string]interface{}) map[string]interface{}

	// Custom Page & Action Overrides (CRUDBooster getIndex & getDetail equivalents)
	GetIndex  func(w http.ResponseWriter, r *http.Request) bool
	GetDetail func(w http.ResponseWriter, r *http.Request, id string) bool
	GetAdd    func(w http.ResponseWriter, r *http.Request) bool
	GetEdit   func(w http.ResponseWriter, r *http.Request, id string) bool
	// HideFromMenu: controller tidak otomatis ditambahkan ke sidebar (mis. endpoint pencarian internal).
	HideFromMenu  bool
	CustomActions map[string]func(w http.ResponseWriter, r *http.Request, subPath string)

	// SaaS Multi-Tenancy Scoping
	TenantScoped         bool
	TenantScopedExplicit bool

	// Mass-assignment & Public REST security controls
	WritableFields []string
	PublicAPI      bool
}

// warnedEmptyWritable mencatat controller yang sudah diperingatkan (di luar struct agar Controller tetap aman disalin).
var warnedEmptyWritable sync.Map

// AllowFields registers additional column names that are permitted in create/edit/import/REST payloads
// even if they are not present as editable fields in c.Forms.
func (c *Controller) AllowFields(names ...string) *Controller {
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			c.WritableFields = append(c.WritableFields, trimmed)
		}
	}
	return c
}

// EnablePublicAPI explicitly opts this controller into the public /api/v1/{table} REST endpoints and OpenAPI spec.
func (c *Controller) EnablePublicAPI() *Controller {
	c.PublicAPI = true
	return c
}

func (c *Controller) allowedWritableSet() map[string]bool {
	allowed := make(map[string]bool)
	for _, f := range c.Forms {
		if !f.ReadOnly && f.Name != "" {
			allowed[f.Name] = true
		}
	}
	for _, name := range c.WritableFields {
		if name != "" {
			allowed[name] = true
		}
	}
	return allowed
}

func (c *Controller) filterWritable(data map[string]interface{}) map[string]interface{} {
	allowed := c.allowedWritableSet()
	if len(allowed) == 0 {
		if _, warned := warnedEmptyWritable.LoadOrStore(c, true); !warned {
			log.Printf("[security] controller %q (table=%q) has no writable form fields or WritableFields; all incoming fields were rejected", c.Title, c.Table)
		}
		return make(map[string]interface{})
	}
	filtered := make(map[string]interface{}, len(data))
	for k, v := range data {
		if allowed[k] {
			filtered[k] = v
		}
	}
	return filtered
}

func isMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}
	return strings.Contains(err.Error(), "http: request body too large")
}

// SetTenantScoped explicitly configures tenant data isolation for this controller
func (c *Controller) SetTenantScoped(scoped bool) *Controller {
	c.TenantScoped = scoped
	c.TenantScopedExplicit = true
	if sqlStore, ok := c.DataProvider.(*SQLStore); ok && sqlStore != nil {
		sqlStore.SetTenantScoped(scoped)
	}
	if memStore, ok := c.DataProvider.(*MemoryStore); ok && memStore != nil {
		memStore.SetTenantScoped(scoped)
	}
	return c
}

// IsTenantScoped resolves whether this controller is scoped to the active tenant
func (c *Controller) IsTenantScoped() bool {
	if c.TenantScopedExplicit {
		return c.TenantScoped
	}
	if c.Engine != nil && c.Engine.MultiTenancyEnabled {
		switch c.Table {
		case "users", "roles", "privileges", "menus", "settings", "tenants", "api_tokens", "pages", "logs", "email_templates":
			return false
		default:
			return true
		}
	}
	return c.TenantScoped
}

// SetDataProvider assigns the Model DataProvider for MVC architecture
func (c *Controller) SetDataProvider(dp DataProvider) *Controller {
	c.DataProvider = dp
	return c
}

// AddCol adds a data grid column fluently
func (c *Controller) AddCol(label, name string, colType ColumnType, searchable, sortable bool) *Controller {
	c.Columns = append(c.Columns, Column{
		Label:      label,
		Name:       name,
		Type:       colType,
		Searchable: searchable,
		Sortable:   sortable,
	})
	return c
}

// AddForm adds a form input field fluently
func (c *Controller) AddForm(label, name string, inputType InputType, required bool, placeholder string) *Controller {
	c.Forms = append(c.Forms, Field{
		Label:       label,
		Name:        name,
		Type:        inputType,
		Required:    required,
		Placeholder: placeholder,
	})
	return c
}

// AddSelect adds a dropdown select form input field fluently
func (c *Controller) AddSelect(label, name string, required bool, placeholder string, options ...Option) *Controller {
	c.Forms = append(c.Forms, Field{
		Label:       label,
		Name:        name,
		Type:        InputSelect,
		Required:    required,
		Placeholder: placeholder,
		Options:     options,
	})
	return c
}

// AddSelectTable adds a dropdown select whose options are automatically populated from another table (e.g. "customers,name" or "categories,title,id")
func (c *Controller) AddSelectTable(label, name, dataTable string, required bool, placeholder ...string) *Controller {
	ph := "Select " + label + "..."
	if len(placeholder) > 0 && placeholder[0] != "" {
		ph = placeholder[0]
	}
	c.Forms = append(c.Forms, Field{
		Label:       label,
		Name:        name,
		Type:        InputSelect,
		Required:    required,
		Placeholder: ph,
		DataTable:   dataTable,
	})
	return c
}

// AddRadioTable adds radio options populated dynamically from another table
func (c *Controller) AddRadioTable(label, name, dataTable string, required bool) *Controller {
	c.Forms = append(c.Forms, Field{
		Label:     label,
		Name:      name,
		Type:      InputRadio,
		Required:  required,
		DataTable: dataTable,
	})
	return c
}

// AddLOV adds an interactive List of Values modal picker component (like enterprise ERP / CRUDBooster datatable modal)
func (c *Controller) AddLOV(label, name, dataTable string, required bool, placeholder ...string) *Controller {
	ph := "Click to select " + label + "..."
	if len(placeholder) > 0 && placeholder[0] != "" {
		ph = placeholder[0]
	}
	format := ""
	parts := strings.Split(dataTable, ",")
	if len(parts) >= 4 {
		format = strings.TrimSpace(parts[3])
		dataTable = strings.Join(parts[:3], ",")
	}
	c.Forms = append(c.Forms, Field{
		Label:       label,
		Name:        name,
		Type:        InputLOV,
		Required:    required,
		Placeholder: ph,
		DataTable:   dataTable,
		Format:      format,
	})
	return c
}

// AddLOVFormatted adds an interactive LOV modal picker with custom display formatting (e.g. "{code} - {name}" or "{id} - {title}")
func (c *Controller) AddLOVFormatted(label, name, dataTable, format string, required bool, placeholder ...string) *Controller {
	ph := "Click to select " + label + "..."
	if len(placeholder) > 0 && placeholder[0] != "" {
		ph = placeholder[0]
	}
	c.Forms = append(c.Forms, Field{
		Label:       label,
		Name:        name,
		Type:        InputLOV,
		Required:    required,
		Placeholder: ph,
		DataTable:   dataTable,
		Format:      format,
	})
	return c
}

// SetPreIndexHTML sets custom HTML markup injected before the data table
func (c *Controller) SetPreIndexHTML(html string) *Controller {
	c.PreIndexHTML = template.HTML(html)
	return c
}

// SetPostIndexHTML sets custom HTML markup injected after the data table
func (c *Controller) SetPostIndexHTML(html string) *Controller {
	c.PostIndexHTML = template.HTML(html)
	return c
}

// SetPreFormHTML sets custom HTML markup injected before create/edit forms
func (c *Controller) SetPreFormHTML(html string) *Controller {
	c.PreFormHTML = template.HTML(html)
	return c
}

// SetPostFormHTML sets custom HTML markup injected after create/edit forms
func (c *Controller) SetPostFormHTML(html string) *Controller {
	c.PostFormHTML = template.HTML(html)
	return c
}

// AddCustomAction registers a custom sub-page or endpoint on the controller
// e.g. ctrl.AddCustomAction("invoice", func(w http.ResponseWriter, r *http.Request, id string) { ... })
func (c *Controller) AddCustomAction(actionSlug string, handler func(w http.ResponseWriter, r *http.Request, subPath string)) *Controller {
	if c.CustomActions == nil {
		c.CustomActions = make(map[string]func(w http.ResponseWriter, r *http.Request, subPath string))
	}
	c.CustomActions[strings.Trim(actionSlug, "/")] = handler
	return c
}

// RenderView renders custom HTML content wrapped inside the full TGo Booster admin layout
func (c *Controller) RenderView(w http.ResponseWriter, r *http.Request, title string, contentHTML template.HTML) {
	if c.Engine != nil {
		c.Engine.RenderLayout(w, r, title, contentHTML)
	} else {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(contentHTML))
	}
}

// FetchRows returns all records from DataProvider or seeded mock
func (c *Controller) FetchRows(r *http.Request) []map[string]interface{} {
	return c.fetchRows(r)
}

// FindRecord looks up a single record by primary key
func (c *Controller) FindRecord(r *http.Request, id string) map[string]interface{} {
	ctx := c.makeContext(r)
	if c.DataProvider != nil {
		if rec, err := c.DataProvider.FindByID(ctx, id); err == nil && rec != nil {
			return rec
		}
	}
	for _, row := range c.fetchRows(r) {
		pk := c.PrimaryKey
		if pk == "" {
			pk = "id"
		}
		if fmt.Sprint(row[pk]) == id {
			return row
		}
	}
	return nil
}

// AddWidget appends a stat KPI widget card atop the module index table
func (c *Controller) AddWidget(w WidgetCard) *Controller {
	c.Widgets = append(c.Widgets, w)
	return c
}

// SetWidgetProvider assigns a dynamic callback to calculate live widgets per request
func (c *Controller) SetWidgetProvider(fn func(r *http.Request) []WidgetCard) *Controller {
	c.WidgetProvider = fn
	return c
}

// AddTopAction appends a custom button to the table toolbar (CRUDBooster index_button)
func (c *Controller) AddTopAction(action TopAction) *Controller {
	c.TopActions = append(c.TopActions, action)
	return c
}

// AddRowAction appends a custom action button to each data row (CRUDBooster addaction)
func (c *Controller) AddRowAction(action RowAction) *Controller {
	c.RowActions = append(c.RowActions, action)
	return c
}

// AddFilterTab appends a quick filter pill tab above the data table
func (c *Controller) AddFilterTab(label, column, value string) *Controller {
	c.FilterTabs = append(c.FilterTabs, FilterTab{
		Label:  label,
		Column: column,
		Value:  value,
	})
	return c
}

func (c *Controller) initDefaults() {
	if c.PrimaryKey == "" {
		c.PrimaryKey = "id"
	}
	if c.OrderBy == "" {
		c.OrderBy = fmt.Sprintf("%s DESC", c.PrimaryKey)
	}
	if c.Icon == "" {
		c.Icon = `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>`
	}
	if c.IsTenantScoped() {
		if sqlStore, ok := c.DataProvider.(*SQLStore); ok && sqlStore != nil {
			sqlStore.SetTenantScoped(true)
		}
		if memStore, ok := c.DataProvider.(*MemoryStore); ok && memStore != nil {
			memStore.SetTenantScoped(true)
		}
	}
	if c.Engine != nil && c.Engine.StrictTenancy {
		if sqlStore, ok := c.DataProvider.(*SQLStore); ok && sqlStore != nil {
			sqlStore.SetStrictTenancy(true)
		}
		if memStore, ok := c.DataProvider.(*MemoryStore); ok && memStore != nil {
			memStore.SetStrictTenancy(true)
		}
	}
}

// ServeHTTP handles all CRUD routing (Index, Data JSON, Add, Edit, Delete)
func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.initDefaults()

	// Auth check
	if c.Engine != nil && c.Engine.Auth != nil {
		if user := c.Engine.Auth.GetSessionUser(r); user == nil {
			http.Redirect(w, r, c.Engine.AdminPath+"/login", http.StatusSeeOther)
			return
		}
	}

	path := strings.TrimPrefix(r.URL.Path, c.BasePath)
	path = strings.TrimPrefix(path, "/")

	// RBAC Permission Check ala CRUDBooster
	action := "read"
	switch {
	case path == "add" || path == "create" || path == "import-csv":
		action = "create"
	case strings.HasPrefix(path, "edit/"):
		action = "update"
	case strings.HasPrefix(path, "delete/") || path == "bulk-action":
		action = "delete"
	case path == "save" || strings.HasPrefix(path, "save/"):
		action = "create"
	}
	// Aksi kustom bermetode tulis (void, reverse, lock, close-year, dst.) tidak boleh lolos sebagai "read".
	if action == "read" && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
		action = "update"
	}

	if c.Engine != nil && !c.Engine.CheckPermission(r, c.Table, action) {
		if r.Header.Get("Accept") == "application/json" || strings.HasSuffix(r.URL.Path, "/data") || r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Access Denied (403): Role does not have '%s' permission on %s", action, c.Title),
			})
			return
		}
		c.Engine.RenderForbidden(w, r, c.Title, action)
		return
	}

	switch {
	case path == "" || path == "data":
		if r.Header.Get("Accept") == "application/json" || strings.HasSuffix(r.URL.Path, "/data") {
			c.handleDataJSON(w, r)
		} else {
			if c.GetIndex != nil && c.GetIndex(w, r) {
				return
			}
			c.handleIndexHTML(w, r)
		}
	case path == "add" || path == "create":
		if r.Method == http.MethodPost {
			c.handleCreateSubmit(w, r)
		} else {
			if c.GetAdd != nil && c.GetAdd(w, r) {
				return
			}
			c.handleCreateHTML(w, r)
		}
	case strings.HasPrefix(path, "edit/"):
		id := strings.TrimPrefix(path, "edit/")
		if r.Method == http.MethodPost {
			c.handleEditSubmit(w, r, id)
		} else {
			if c.GetEdit != nil && c.GetEdit(w, r, id) {
				return
			}
			c.handleEditHTML(w, r, id)
		}
	case strings.HasPrefix(path, "delete/"):
		id := strings.TrimPrefix(path, "delete/")
		c.handleDelete(w, r, id)
	case path == "bulk-action":
		c.handleBulkAction(w, r)
	case path == "import-csv" && r.Method == http.MethodPost:
		c.handleImportCSV(w, r)
	case strings.HasPrefix(path, "detail/"):
		id := strings.TrimPrefix(path, "detail/")
		if c.GetDetail != nil && c.GetDetail(w, r, id) {
			return
		}
		c.handleDetailHTML(w, r, id)
	default:
		// Check CustomActions map (e.g. "invoice", "print", "audit")
		for actionKey, handler := range c.CustomActions {
			cleanKey := strings.Trim(actionKey, "/")
			if path == cleanKey || strings.HasPrefix(path, cleanKey+"/") {
				sub := strings.TrimPrefix(path, cleanKey)
				sub = strings.TrimPrefix(sub, "/")
				handler(w, r, sub)
				return
			}
		}
		http.NotFound(w, r)
	}
}

func (c *Controller) handleIndexHTML(w http.ResponseWriter, r *http.Request) {
	rows := c.fetchRows(r)
	widgets := c.Widgets
	if c.WidgetProvider != nil {
		widgets = c.WidgetProvider(r)
	}

	data := map[string]interface{}{
		"Title":         c.Title,
		"Icon":          c.Icon,
		"Table":         c.Table,
		"PrimaryKey":    c.PrimaryKey,
		"BasePath":      c.BasePath,
		"Columns":       c.Columns,
		"Rows":          rows,
		"PreIndexHTML":  c.PreIndexHTML,
		"PostIndexHTML": c.PostIndexHTML,
		"Widgets":       widgets,
		"TopActions":    c.TopActions,
		"RowActions":    c.RowActions,
		"FilterTabs":    c.FilterTabs,
	}

	contentHTML, err := RenderTableContent(data)
	if err != nil {
		http.Error(w, "Failed to render table: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, c.Title, contentHTML)
}

func (c *Controller) resolveDynamicFormFields(r *http.Request, fields []Field) []Field {
	res := make([]Field, len(fields))
	copy(res, fields)

	for i, f := range res {
		if f.DataTable != "" && c.Engine != nil {
			parts := strings.Split(f.DataTable, ",")
			tbl := strings.TrimSpace(parts[0])
			labelCol := "name"
			if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
				labelCol = strings.TrimSpace(parts[1])
			}
			keyCol := "id"
			if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
				keyCol = strings.TrimSpace(parts[2])
			}

			var targetRows []map[string]interface{}
			if targetCtrl := c.Engine.FindController(tbl); targetCtrl != nil {
				targetRows = targetCtrl.fetchRows(r)
			} else if db := c.Engine.FindDB(); db != nil {
				query := fmt.Sprintf("SELECT %s, %s FROM %s ORDER BY %s ASC LIMIT 100", keyCol, labelCol, tbl, labelCol)
				rows, err := db.Query(query)
				if err == nil {
					defer rows.Close()
					for rows.Next() {
						var k, l interface{}
						if err := rows.Scan(&k, &l); err == nil {
							targetRows = append(targetRows, map[string]interface{}{
								keyCol:   fmt.Sprint(k),
								labelCol: fmt.Sprint(l),
							})
						}
					}
				}
			}

			// For Select, Select2, and Radio: auto populate Options
			if f.Type == InputSelect || f.Type == InputSelect2 || f.Type == InputRadio {
				if len(f.Options) == 0 && len(targetRows) > 0 {
					opts := make([]Option, 0, len(targetRows))
					for _, row := range targetRows {
						val := fmt.Sprint(row[keyCol])
						lbl := fmt.Sprint(row[labelCol])
						if lbl == "" || lbl == "<nil>" {
							lbl = val
						}
						opts = append(opts, Option{Value: val, Label: lbl})
					}
					res[i].Options = opts
				}
			}

			// For LOV: resolve DisplayValue for existing Value
			if f.Type == InputLOV && res[i].Value != nil {
				valStr := fmt.Sprint(res[i].Value)
				for _, row := range targetRows {
					if fmt.Sprint(row[keyCol]) == valStr {
						if f.Format != "" {
							disp := f.Format
							for k, v := range row {
								disp = strings.ReplaceAll(disp, "{"+k+"}", fmt.Sprint(v))
								disp = strings.ReplaceAll(disp, "["+k+"]", fmt.Sprint(v))
							}
							res[i].DisplayValue = disp
						} else {
							res[i].DisplayValue = fmt.Sprint(row[labelCol])
						}
						break
					}
				}
				if res[i].DisplayValue == "" {
					res[i].DisplayValue = valStr
				}
			}
		}
	}
	return res
}

func (c *Controller) handleCreateHTML(w http.ResponseWriter, r *http.Request) {
	createForms := make([]Field, len(c.Forms))
	for i, f := range c.Forms {
		createForms[i] = f
		if f.DefaultValue != "" {
			createForms[i].Value = f.DefaultValue
		}
	}
	createForms = c.resolveDynamicFormFields(r, createForms)

	adminPath := "/admin"
	if c.Engine != nil {
		adminPath = c.Engine.AdminPath
	}

	data := map[string]interface{}{
		"Title":        c.Title,
		"AdminPath":    adminPath,
		"BasePath":     c.BasePath,
		"FormAction":   c.BasePath + "/add",
		"Forms":        createForms,
		"IsEdit":       false,
		"PreFormHTML":  c.PreFormHTML,
		"PostFormHTML": c.PostFormHTML,
	}

	contentHTML, err := RenderFormContent(data)
	if err != nil {
		http.Error(w, "Failed to render form: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, "Add "+c.Title, contentHTML)
}

func (c *Controller) makeContext(r *http.Request) *Context {
	var user *User
	if c.Engine != nil && c.Engine.Auth != nil {
		user = c.Engine.Auth.GetSessionUser(r)
	}
	tenantSlug := ""
	if c.Engine != nil {
		tenantSlug = c.Engine.ResolveTenant(r, user)
	} else {
		tenantSlug = middleware.GetTenant(r.Context())
	}
	return &Context{
		Ctx:        r.Context(),
		Request:    r,
		User:       user,
		TenantSlug: tenantSlug,
	}
}

func (c *Controller) handleEditHTML(w http.ResponseWriter, r *http.Request, id string) {
	var record map[string]interface{}
	if c.DataProvider != nil {
		ctx := c.makeContext(r)
		if rec, err := c.DataProvider.FindByID(ctx, id); err == nil {
			record = rec
		}
	} else {
		for _, row := range c.fetchRows(r) {
			if fmt.Sprint(row[c.PrimaryKey]) == id {
				record = row
				break
			}
		}
	}

	// Populate field values for editing
	editForms := make([]Field, len(c.Forms))
	for i, f := range c.Forms {
		editForms[i] = f
		if record != nil {
			if val, ok := record[f.Name]; ok && val != nil {
				editForms[i].Value = formatFieldValue(f, val)
			}
		}
	}
	editForms = c.resolveDynamicFormFields(r, editForms)

	adminPath := "/admin"
	if c.Engine != nil {
		adminPath = c.Engine.AdminPath
	}

	data := map[string]interface{}{
		"Title":        c.Title,
		"AdminPath":    adminPath,
		"BasePath":     c.BasePath,
		"RecordID":     id,
		"FormAction":   c.BasePath + "/edit/" + id,
		"Forms":        editForms,
		"IsEdit":       true,
		"PreFormHTML":  c.PreFormHTML,
		"PostFormHTML": c.PostFormHTML,
	}

	contentHTML, err := RenderFormContent(data)
	if err != nil {
		http.Error(w, "Failed to render form: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, "Edit "+c.Title, contentHTML)
}

func (c *Controller) fetchRows(r *http.Request) []map[string]interface{} {
	if c.DataProvider != nil {
		ctx := c.makeContext(r)
		if rows, err := c.DataProvider.FindAll(ctx); err == nil && rows != nil {
			return rows
		}
	}

	switch c.Table {
	case "products":
		return []map[string]interface{}{
			{"id": "101", "name": "Apple MacBook Pro 16 M3 Max", "price": "Rp 42.000.000", "stock": "14", "status": "Active"},
			{"id": "102", "name": "Keychron Q1 Pro Wireless Mechanical Keyboard", "price": "Rp 2.850.000", "stock": "25", "status": "Active"},
			{"id": "103", "name": "LG UltraFine 4K 32-inch Ergonomic Display", "price": "Rp 11.500.000", "stock": "4", "status": "Processing"},
			{"id": "104", "name": "Sony WH-1000XM5 Noise Cancelling Headphones", "price": "Rp 4.999.000", "stock": "18", "status": "Active"},
			{"id": "105", "name": "Logitech MX Master 3S Wireless Mouse", "price": "Rp 1.650.000", "stock": "32", "status": "Active"},
		}
	case "customers":
		return []map[string]interface{}{
			{"id": "201", "name": "Budi Cahyono", "email": "budi.c@enterprise.co.id", "status": "Active"},
			{"id": "202", "name": "Siti Wulandari", "email": "siti.wulandari@gmail.com", "status": "Active"},
			{"id": "203", "name": "Ahmad Ridwan", "email": "aridwan@tokalink.io", "status": "Active"},
			{"id": "204", "name": "Dewi Sartika", "email": "dewi.sartika@startup.id", "status": "Active"},
			{"id": "205", "name": "Hendra Pratama", "email": "hendra.p@cloudtech.com", "status": "Active"},
		}
	case "orders":
		return []map[string]interface{}{
			{"id": "ORD-9024", "customer_name": "Budi Cahyono", "total_amount": "Rp 4.850.000", "status": "Completed"},
			{"id": "ORD-9023", "customer_name": "Siti Wulandari", "total_amount": "Rp 12.400.000", "status": "In Transit"},
			{"id": "ORD-9022", "customer_name": "Ahmad Ridwan", "total_amount": "Rp 1.250.000", "status": "Processing"},
			{"id": "ORD-9021", "customer_name": "Dewi Sartika", "total_amount": "Rp 42.000.000", "status": "Completed"},
		}
	default:
		return []map[string]interface{}{}
	}
}

func (c *Controller) handleDataJSON(w http.ResponseWriter, r *http.Request) {
	rows := c.fetchRows(r)
	if c.HookRowListing != nil {
		for i, row := range rows {
			rows[i] = c.HookRowListing(row)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func isISODatePrefix(s string) bool {
	if len(s) < 10 {
		return false
	}
	for _, idx := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if s[idx] < '0' || s[idx] > '9' {
			return false
		}
	}
	return s[4] == '-' && s[7] == '-'
}

func formatFieldValue(f Field, v interface{}) string {
	if v == nil {
		return ""
	}
	if b, ok := v.([]byte); ok {
		v = string(b)
	}
	switch f.Type {
	case InputDate:
		switch tv := v.(type) {
		case time.Time:
			if tv.IsZero() {
				return ""
			}
			return tv.Format("2006-01-02")
		case *time.Time:
			if tv == nil || tv.IsZero() {
				return ""
			}
			return tv.Format("2006-01-02")
		case string:
			s := strings.TrimSpace(tv)
			if isISODatePrefix(s) {
				return s[:10]
			}
			return s
		}
	case InputDateTime:
		switch tv := v.(type) {
		case time.Time:
			if tv.IsZero() {
				return ""
			}
			return tv.Format("2006-01-02T15:04")
		case *time.Time:
			if tv == nil || tv.IsZero() {
				return ""
			}
			return tv.Format("2006-01-02T15:04")
		case string:
			s := strings.TrimSpace(tv)
			if len(s) >= 16 && isISODatePrefix(s) && (s[10] == 'T' || s[10] == ' ') && s[13] == ':' {
				return s[:10] + "T" + s[11:16]
			}
			if parsed, err := time.Parse(time.RFC3339, s); err == nil {
				return parsed.Format("2006-01-02T15:04")
			}
			return s
		}
	}
	return fmt.Sprint(v)
}

func resolveCheckboxSubmitValue(f Field, oldVal interface{}, hasOld bool, checked bool) interface{} {
	if len(f.Options) >= 2 {
		if checked {
			return f.Options[0].Value
		}
		return f.Options[1].Value
	}
	if hasOld && oldVal != nil {
		if rawBytes, ok := oldVal.([]byte); ok {
			oldVal = string(rawBytes)
		}
		switch ov := oldVal.(type) {
		case bool:
			return checked
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			if checked {
				return 1
			}
			return 0
		case string:
			switch strings.ToLower(strings.TrimSpace(ov)) {
			case "active", "inactive":
				if checked {
					return "Active"
				}
				return "Inactive"
			case "true", "false", "t", "f":
				if checked {
					return "true"
				}
				return "false"
			case "yes", "no":
				if checked {
					return "yes"
				}
				return "no"
			case "y", "n":
				if checked {
					return "Y"
				}
				return "N"
			case "1", "0":
				if checked {
					return "1"
				}
				return "0"
			}
		}
	}
	if checked {
		return "1"
	}
	return "0"
}

func (c *Controller) normalizeSubmittedFormData(r *http.Request, data map[string]interface{}, oldRecord map[string]interface{}, isEdit bool) {
	for _, f := range c.Forms {
		if f.ReadOnly || f.Name == "" {
			continue
		}
		switch f.Type {
		case InputCheckbox:
			checked := false
			if vals, exists := r.PostForm[f.Name]; exists && len(vals) > 0 {
				v := vals[0]
				if len(f.Options) >= 2 && v == f.Options[0].Value {
					checked = true
				} else {
					checked = isChecked(v)
				}
			}
			var oldVal interface{}
			hasOld := false
			if isEdit && oldRecord != nil {
				oldVal, hasOld = oldRecord[f.Name]
			}
			data[f.Name] = resolveCheckboxSubmitValue(f, oldVal, hasOld, checked)

		case InputPassword:
			if isEdit {
				if val, exists := data[f.Name]; !exists || fmt.Sprint(val) == "" {
					delete(data, f.Name)
				}
			}

		case InputDate:
			if val, exists := data[f.Name]; exists {
				if s, ok := val.(string); ok && strings.TrimSpace(s) == "" {
					data[f.Name] = nil
				}
			}

		case InputDateTime:
			if val, exists := data[f.Name]; exists {
				if s, ok := val.(string); ok {
					trimmed := strings.TrimSpace(s)
					if trimmed == "" {
						data[f.Name] = nil
					} else if len(trimmed) == 16 && isISODatePrefix(trimmed) && trimmed[10] == 'T' && trimmed[13] == ':' {
						data[f.Name] = trimmed[:10] + " " + trimmed[11:16] + ":00"
					}
				}
			}

		case InputWYSIWYG:
			if val, exists := data[f.Name]; exists {
				if s, ok := val.(string); ok {
					data[f.Name] = string(SanitizeRichHTML(s))
				}
			}
		}
	}
}

func (c *Controller) handleCreateSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	data := make(map[string]interface{})
	for k, v := range r.PostForm {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	c.normalizeSubmittedFormData(r, data, nil, false)
	data = c.filterWritable(data)

	ctx := c.makeContext(r)
	if c.HookBeforeAdd != nil {
		if err := c.HookBeforeAdd(ctx, data); err != nil {
			c.writeFormError(w, r, http.StatusBadRequest, "Isian belum valid", err.Error())
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Create(ctx, data); err != nil {
			if m := FriendlyDBError(err); m != "" {
				c.writeFormError(w, r, http.StatusUnprocessableEntity, "Data tidak dapat disimpan", m)
			} else {
				c.writeFormError(w, r, http.StatusInternalServerError, "Data tidak dapat disimpan", "Failed to create record: "+err.Error())
			}
			return
		}
	}

	if c.HookAfterAdd != nil {
		_ = c.HookAfterAdd(ctx, data)
	}

	if c.Engine != nil {
		recID := fmt.Sprint(data[c.PrimaryKey])
		if recID == "" || recID == "<nil>" {
			recID = "new"
		}
		c.Engine.LogAudit(r, "CREATE", c.Title, fmt.Sprintf("Created new %s record (ID: %s)", c.Title, recID))
	}

	http.Redirect(w, r, c.BasePath+"?alert="+url.QueryEscape("Record created successfully!")+"&alert_type=success", http.StatusSeeOther)
}

func (c *Controller) handleEditSubmit(w http.ResponseWriter, r *http.Request, id string) {
	_ = r.ParseForm()
	data := make(map[string]interface{})
	for k, v := range r.PostForm {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	ctx := c.makeContext(r)
	var oldRecord map[string]interface{}
	if c.DataProvider != nil {
		if rec, err := c.DataProvider.FindByID(ctx, id); err == nil {
			oldRecord = rec
		}
	}

	c.normalizeSubmittedFormData(r, data, oldRecord, true)
	data = c.filterWritable(data)

	if c.HookBeforeEdit != nil {
		if err := c.HookBeforeEdit(ctx, data); err != nil {
			c.writeFormError(w, r, http.StatusBadRequest, "Isian belum valid", err.Error())
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Update(ctx, id, data); err != nil {
			if m := FriendlyDBError(err); m != "" {
				c.writeFormError(w, r, http.StatusUnprocessableEntity, "Data tidak dapat disimpan", m)
			} else {
				c.writeFormError(w, r, http.StatusInternalServerError, "Data tidak dapat disimpan", "Failed to update record: "+err.Error())
			}
			return
		}
	}

	if c.HookAfterEdit != nil {
		_ = c.HookAfterEdit(ctx, data)
	}

	if c.Engine != nil {
		c.Engine.LogAudit(r, "UPDATE", c.Title, fmt.Sprintf("Updated %s record #%s", c.Title, id))
	}

	http.Redirect(w, r, c.BasePath+"?alert="+url.QueryEscape("Record updated successfully!")+"&alert_type=success", http.StatusSeeOther)
}

func (c *Controller) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	// Aksi yang mengubah data hanya lewat POST/DELETE; GET/HEAD (mis. CSRF via navigasi atau <img>) ditolak.
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "POST, DELETE")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := c.makeContext(r)
	if c.HookBeforeDelete != nil {
		if err := c.HookBeforeDelete(ctx, map[string]interface{}{c.PrimaryKey: id}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Delete(ctx, id); err != nil {
			if m := FriendlyDBError(err); m != "" {
				http.Error(w, m, http.StatusConflict)
			} else {
				http.Error(w, "Failed to delete record: "+err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}

	if c.HookAfterDelete != nil {
		_ = c.HookAfterDelete(ctx, map[string]interface{}{c.PrimaryKey: id})
	}

	if c.Engine != nil {
		c.Engine.LogAudit(r, "DELETE", c.Title, fmt.Sprintf("Deleted %s record #%s", c.Title, id))
	}

	http.Redirect(w, r, c.BasePath+"?alert="+url.QueryEscape("Record deleted successfully!")+"&alert_type=success", http.StatusSeeOther)
}

func (c *Controller) handleBulkAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if c.Engine != nil && !c.Engine.CheckPermission(r, c.Table, "delete") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Forbidden: insufficient delete privileges",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		if isMaxBytesError(err) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Request body too large",
			})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid form payload",
		})
		return
	}

	action := r.FormValue("action")
	if action != "delete" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unsupported bulk action",
		})
		return
	}

	idsStr := r.FormValue("ids")
	if idsStr == "" {
		idsStr = r.URL.Query().Get("ids")
	}
	rawIDs := strings.Split(idsStr, ",")
	ids := make([]string, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id := strings.TrimSpace(raw)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "No IDs specified",
		})
		return
	}
	if len(ids) > 1000 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Too many IDs (maximum 1000)",
		})
		return
	}

	pk := c.PrimaryKey
	if pk == "" {
		pk = "id"
	}

	ctx := c.makeContext(r)
	count := 0
	failed := 0
	firstErr := ""

	for _, id := range ids {
		hookPayload := map[string]interface{}{pk: id, "id": id}
		if c.HookBeforeDelete != nil {
			if err := c.HookBeforeDelete(ctx, hookPayload); err != nil {
				failed++
				if firstErr == "" {
					firstErr = err.Error()
				}
				continue
			}
		}
		if c.DataProvider != nil {
			if err := c.DataProvider.Delete(ctx, id); err != nil {
				failed++
				if firstErr == "" {
					firstErr = err.Error()
				}
				continue
			}
		}
		if c.HookAfterDelete != nil {
			_ = c.HookAfterDelete(ctx, hookPayload)
		}
		count++
	}

	if c.Engine != nil && count > 0 {
		c.Engine.LogAudit(r, "BULK_DELETE", c.Title, fmt.Sprintf("Bulk processed %d records in %s (%s)", count, c.Title, action))
	}

	if !strings.Contains(r.Header.Get("Accept"), "application/json") && r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		alertType, msg := "success", fmt.Sprintf("%d records processed successfully", count)
		if failed > 0 {
			alertType, msg = "error", fmt.Sprintf("%d records processed, %d failed: %s", count, failed, firstErr)
		}
		http.Redirect(w, r, fmt.Sprintf("%s?alert=%s&alert_type=%s", c.BasePath, url.QueryEscape(msg), alertType), http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	resp := map[string]interface{}{
		"success": failed == 0,
		"action":  action,
		"count":   count,
		"failed":  failed,
	}
	if failed > 0 {
		resp["error"] = firstErr
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *Controller) handleDetailHTML(w http.ResponseWriter, r *http.Request, id string) {
	var record map[string]interface{}
	if c.DataProvider != nil {
		ctx := c.makeContext(r)
		if rec, err := c.DataProvider.FindByID(ctx, id); err == nil {
			record = rec
		}
	} else {
		for _, row := range c.fetchRows(r) {
			if fmt.Sprint(row[c.PrimaryKey]) == id {
				record = row
				break
			}
		}
	}

	if record == nil {
		http.Redirect(w, r, c.BasePath+"?alert="+url.QueryEscape("Record not found")+"&alert_type=error", http.StatusSeeOther)
		return
	}

	pk := c.PrimaryKey
	if pk == "" {
		pk = "id"
	}
	recID := id
	if rawID, ok := record[pk]; ok && rawID != nil {
		if s := fmt.Sprint(rawID); s != "" && s != "<nil>" {
			recID = s
		}
	}

	safeTitle := template.HTMLEscapeString(c.Title)
	safeRecID := template.HTMLEscapeString(recID)
	safeBasePath := template.HTMLEscapeString(c.BasePath)
	safeEditHref := template.HTMLEscapeString(c.BasePath + "/edit/" + url.PathEscape(recID))

	html := fmt.Sprintf(`
		<div class="cb-card" style="max-width: 760px; margin: 0 auto;">
			<div class="cb-card-header">
				<div>
					<h2 class="cb-card-title">
						<span>%s Record <span class="cell-id">#%s</span></span>
					</h2>
					<p class="cb-card-sub">Detailed attributes and properties for this entry</p>
				</div>
				<div style="display: flex; gap: 8px;">
					<a href="%s" class="btn btn-secondary btn-sm">
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m15 18-6-6 6-6"/></svg>
						<span>Back</span>
					</a>
					<a href="%s" class="btn btn-primary btn-sm">
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg>
						<span>Edit Record</span>
					</a>
				</div>
			</div>
			<table class="detail-table" style="width: 100%%;">
				<tbody>
	`, safeTitle, safeRecID, safeBasePath, safeEditHref)

	for _, col := range c.Columns {
		val := record[col.Name]
		valStr := "-"
		if val != nil {
			valStr = fmt.Sprint(val)
		}
		html += fmt.Sprintf(`
			<tr>
				<td class="dt-label">%s</td>
				<td class="dt-val">%s</td>
			</tr>
		`, template.HTMLEscapeString(col.Label), template.HTMLEscapeString(valStr))
	}

	html += `
				</tbody>
			</table>
		</div>
	`

	c.Engine.RenderLayout(w, r, c.Title+" #"+recID, template.HTML(html))
}

func (c *Controller) handleImportCSV(w http.ResponseWriter, r *http.Request) {
	if c.Engine != nil && !c.Engine.CheckPermission(r, c.Table, "create") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Access Denied (403): Role does not have 'create' permission on " + c.Title,
		})
		return
	}

	const maxImportBytes = 10 << 20
	const maxImportRows = 5000
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)

	var rows []map[string]interface{}

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(maxImportBytes); err != nil {
			w.Header().Set("Content-Type", "application/json")
			if isMaxBytesError(err) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV file exceeds 10 MiB limit",
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to parse multipart form: " + err.Error(),
			})
			return
		}
		file, _, err := r.FormFile("csv_file")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Missing csv_file in upload: " + err.Error(),
			})
			return
		}
		defer file.Close()

		reader := csv.NewReader(file)
		headers, err := reader.Read()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if isMaxBytesError(err) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV file exceeds 10 MiB limit",
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to read CSV header: " + err.Error(),
			})
			return
		}

		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if isMaxBytesError(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV file exceeds 10 MiB limit",
				})
				return
			}
			if err != nil {
				continue
			}
			row := make(map[string]interface{})
			for i, h := range headers {
				if i < len(record) {
					key := strings.TrimSpace(h)
					if key != "" {
						row[key] = record[i]
					}
				}
			}
			rows = append(rows, row)
			if len(rows) > maxImportRows {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV import exceeds maximum of 5000 rows",
				})
				return
			}
		}
	} else {
		var payload struct {
			Rows []map[string]interface{} `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			if isMaxBytesError(err) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV import payload exceeds 10 MiB limit",
				})
				return
			}
			if _, copyErr := io.Copy(io.Discard, r.Body); isMaxBytesError(copyErr) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "CSV import payload exceeds 10 MiB limit",
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid CSV import payload: " + err.Error(),
			})
			return
		}
		if len(payload.Rows) > maxImportRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "CSV import exceeds maximum of 5000 rows",
			})
			return
		}
		rows = payload.Rows
	}

	ctx := c.makeContext(r)
	successCount := 0
	for _, row := range rows {
		row = c.filterWritable(row)
		if c.HookBeforeAdd != nil {
			if err := c.HookBeforeAdd(ctx, row); err != nil {
				continue
			}
		}
		if len(row) == 0 {
			continue
		}
		if c.DataProvider != nil {
			if err := c.DataProvider.Create(ctx, row); err == nil {
				successCount++
				if c.HookAfterAdd != nil {
					_ = c.HookAfterAdd(ctx, row)
				}
			}
		}
	}

	if c.Engine != nil {
		c.Engine.LogAudit(r, "IMPORT_CSV", c.Title, fmt.Sprintf("Imported %d records into %s", successCount, c.Title))
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   successCount,
		"message": fmt.Sprintf("%d records imported successfully", successCount),
	})
}

// AutoMigrate automatically creates the SQL table for the controller if backed by SQLStore.
func (c *Controller) AutoMigrate() error {
	if sqlStore, ok := c.DataProvider.(*SQLStore); ok && sqlStore != nil {
		if c.IsTenantScoped() {
			sqlStore.SetTenantScoped(true)
		}
		return sqlStore.AutoMigrate(c.Forms...)
	}
	return nil
}
