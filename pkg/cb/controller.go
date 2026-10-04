package cb

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"

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
	GetIndex      func(w http.ResponseWriter, r *http.Request) bool
	GetDetail     func(w http.ResponseWriter, r *http.Request, id string) bool
	GetAdd        func(w http.ResponseWriter, r *http.Request) bool
	GetEdit       func(w http.ResponseWriter, r *http.Request, id string) bool
	CustomActions map[string]func(w http.ResponseWriter, r *http.Request, subPath string)
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
	case path == "add" || path == "create":
		action = "create"
	case strings.HasPrefix(path, "edit/"):
		action = "update"
	case strings.HasPrefix(path, "delete/") || path == "bulk-action":
		action = "delete"
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
	return &Context{
		Ctx:        r.Context(),
		Request:    r,
		User:       user,
		TenantSlug: middleware.GetTenant(r.Context()),
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
				editForms[i].Value = fmt.Sprint(val)
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

func (c *Controller) handleCreateSubmit(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	data := make(map[string]interface{})
	for k, v := range r.PostForm {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	ctx := c.makeContext(r)
	if c.HookBeforeAdd != nil {
		if err := c.HookBeforeAdd(ctx, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Create(ctx, data); err != nil {
			http.Error(w, "Failed to create record: "+err.Error(), http.StatusInternalServerError)
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
	if c.HookBeforeEdit != nil {
		if err := c.HookBeforeEdit(ctx, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Update(ctx, id, data); err != nil {
			http.Error(w, "Failed to update record: "+err.Error(), http.StatusInternalServerError)
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
	ctx := c.makeContext(r)
	if c.HookBeforeDelete != nil {
		if err := c.HookBeforeDelete(ctx, map[string]interface{}{c.PrimaryKey: id}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if c.DataProvider != nil {
		if err := c.DataProvider.Delete(ctx, id); err != nil {
			http.Error(w, "Failed to delete record: "+err.Error(), http.StatusInternalServerError)
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
	_ = r.ParseForm()
	action := r.FormValue("action")
	idsStr := r.FormValue("ids")
	if idsStr == "" {
		idsStr = r.URL.Query().Get("ids")
	}
	ids := strings.Split(idsStr, ",")

	ctx := c.makeContext(r)
	count := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if action == "delete" {
			if c.DataProvider != nil {
				_ = c.DataProvider.Delete(ctx, id)
				count++
			}
		}
	}

	if c.Engine != nil {
		c.Engine.LogAudit(r, "BULK_DELETE", c.Title, fmt.Sprintf("Bulk processed %d records in %s (%s)", count, c.Title, action))
	}

	if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"action":  action,
			"count":   count,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s?alert=%s&alert_type=success", c.BasePath, url.QueryEscape(fmt.Sprintf("%d records processed successfully", count))), http.StatusSeeOther)
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
					<a href="%s/edit/%s" class="btn btn-primary btn-sm">
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/></svg>
						<span>Edit Record</span>
					</a>
				</div>
			</div>
			<table class="detail-table" style="width: 100%%;">
				<tbody>
	`, c.Title, id, c.BasePath, c.BasePath, id)

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
		`, col.Label, template.HTMLEscapeString(valStr))
	}

	html += `
				</tbody>
			</table>
		</div>
	`

	c.Engine.RenderLayout(w, r, c.Title+" #"+id, template.HTML(html))
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

	var rows []map[string]interface{}

	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			w.Header().Set("Content-Type", "application/json")
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
			if err != nil {
				continue
			}
			row := make(map[string]interface{})
			for i, h := range headers {
				if i < len(record) {
					key := strings.TrimSpace(h)
					row[key] = record[i]
				}
			}
			rows = append(rows, row)
		}
	} else {
		var payload struct {
			Rows []map[string]interface{} `json:"rows"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid CSV import payload: " + err.Error(),
			})
			return
		}
		rows = payload.Rows
	}

	ctx := c.makeContext(r)
	successCount := 0
	for _, row := range rows {
		if c.HookBeforeAdd != nil {
			if err := c.HookBeforeAdd(ctx, row); err != nil {
				continue
			}
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
		return sqlStore.AutoMigrate(c.Forms...)
	}
	return nil
}

