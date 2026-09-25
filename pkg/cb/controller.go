package cb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/middleware"
)

// Controller represents a CRUDBooster CBController
type Controller struct {
	Engine     *Engine
	Title      string
	Table      string
	Icon       string
	PrimaryKey string
	OrderBy    string
	Columns    []Column
	Forms      []Field
	BasePath   string

	// Hooks
	HookBeforeAdd    HookFunc
	HookAfterAdd     HookFunc
	HookBeforeEdit   HookFunc
	HookAfterEdit    HookFunc
	HookBeforeDelete HookFunc
	HookAfterDelete  HookFunc
	HookRowListing   func(row map[string]interface{}) map[string]interface{}
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

func (c *Controller) initDefaults() {
	if c.PrimaryKey == "" {
		c.PrimaryKey = "id"
	}
	if c.OrderBy == "" {
		c.OrderBy = fmt.Sprintf("%s DESC", c.PrimaryKey)
	}
	if c.Icon == "" {
		c.Icon = "📋"
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

	switch {
	case path == "" || path == "data":
		if r.Header.Get("Accept") == "application/json" || strings.HasSuffix(r.URL.Path, "/data") {
			c.handleDataJSON(w, r)
		} else {
			c.handleIndexHTML(w, r)
		}
	case path == "add" || path == "create":
		if r.Method == http.MethodPost {
			c.handleCreateSubmit(w, r)
		} else {
			c.handleCreateHTML(w, r)
		}
	case strings.HasPrefix(path, "edit/"):
		id := strings.TrimPrefix(path, "edit/")
		if r.Method == http.MethodPost {
			c.handleEditSubmit(w, r, id)
		} else {
			c.handleEditHTML(w, r, id)
		}
	case strings.HasPrefix(path, "delete/"):
		id := strings.TrimPrefix(path, "delete/")
		c.handleDelete(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (c *Controller) handleIndexHTML(w http.ResponseWriter, r *http.Request) {
	rows := c.fetchRows(r)
	data := map[string]interface{}{
		"Title":    c.Title,
		"Icon":     c.Icon,
		"Table":    c.Table,
		"BasePath": c.BasePath,
		"Columns":  c.Columns,
		"Rows":     rows,
	}

	contentHTML, err := RenderTableContent(data)
	if err != nil {
		http.Error(w, "Failed to render table: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, c.Title, contentHTML)
}

func (c *Controller) handleCreateHTML(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      c.Title,
		"BasePath":   c.BasePath,
		"FormAction": c.BasePath + "/add",
		"Forms":      c.Forms,
		"IsEdit":     false,
	}

	contentHTML, err := RenderFormContent(data)
	if err != nil {
		http.Error(w, "Failed to render form: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, "Add "+c.Title, contentHTML)
}

func (c *Controller) handleEditHTML(w http.ResponseWriter, r *http.Request, id string) {
	data := map[string]interface{}{
		"Title":      c.Title,
		"BasePath":   c.BasePath,
		"RecordID":   id,
		"FormAction": c.BasePath + "/edit/" + id,
		"Forms":      c.Forms,
		"IsEdit":     true,
	}

	contentHTML, err := RenderFormContent(data)
	if err != nil {
		http.Error(w, "Failed to render form: "+err.Error(), http.StatusInternalServerError)
		return
	}

	c.Engine.RenderLayout(w, r, "Edit "+c.Title, contentHTML)
}

func (c *Controller) fetchRows(r *http.Request) []map[string]interface{} {
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

	if c.HookBeforeAdd != nil {
		ctx := &Context{Ctx: r.Context(), Request: r, User: c.Engine.Auth.GetSessionUser(r), TenantSlug: middleware.GetTenant(r.Context())}
		if err := c.HookBeforeAdd(ctx, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, c.BasePath, http.StatusSeeOther)
}

func (c *Controller) handleEditSubmit(w http.ResponseWriter, r *http.Request, id string) {
	_ = r.ParseForm()
	data := make(map[string]interface{})
	for k, v := range r.PostForm {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}

	if c.HookBeforeEdit != nil {
		ctx := &Context{Ctx: r.Context(), Request: r, User: c.Engine.Auth.GetSessionUser(r), TenantSlug: middleware.GetTenant(r.Context())}
		if err := c.HookBeforeEdit(ctx, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, c.BasePath, http.StatusSeeOther)
}

func (c *Controller) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if c.HookBeforeDelete != nil {
		ctx := &Context{Ctx: r.Context(), Request: r, User: c.Engine.Auth.GetSessionUser(r), TenantSlug: middleware.GetTenant(r.Context())}
		if err := c.HookBeforeDelete(ctx, map[string]interface{}{"id": id}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, c.BasePath, http.StatusSeeOther)
}
