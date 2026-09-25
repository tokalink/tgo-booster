package cb

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/middleware"
)

// Controller represents a CRUDBooster CBController
type Controller struct {
	Engine      *Engine
	Title       string
	Table       string
	Icon        string
	PrimaryKey  string
	OrderBy     string
	Columns     []Column
	Forms       []Field
	BasePath    string

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
	tableHeaders := ""
	for _, col := range c.Columns {
		tableHeaders += fmt.Sprintf("<th>%s</th>", col.Label)
	}
	tableHeaders += "<th style='width: 140px; text-align: right;'>Actions</th>"

	body := fmt.Sprintf(`
		<div class="cb-card">
			<div class="cb-toolbar">
				<div>
					<input type="text" class="cb-search" placeholder="Search %s..." id="cb-table-search" oninput="filterTable()" />
				</div>
				<div style="display: flex; gap: 8px;">
					<a href="%s/add" class="btn btn-primary">+ Add New %s</a>
				</div>
			</div>
			<div style="overflow-x: auto;">
				<table class="cb-table" id="cb-main-table">
					<thead>
						<tr>%s</tr>
					</thead>
					<tbody id="cb-tbody">
						<tr><td colspan="%d" style="text-align: center; color: #94a3b8; padding: 2rem;">Loading data...</td></tr>
					</tbody>
				</table>
			</div>
		</div>

		<script>
		let rawData = [];
		async function loadData() {
			try {
				const res = await fetch('%s/data', { headers: { 'Accept': 'application/json' } });
				rawData = await res.json();
				renderRows(rawData);
			} catch(e) {
				console.error(e);
			}
		}

		function renderRows(data) {
			const tbody = document.getElementById('cb-tbody');
			if (!data || data.length === 0) {
				tbody.innerHTML = '<tr><td colspan="%d" style="text-align: center; color: #94a3b8; padding: 2rem;">No records found</td></tr>';
				return;
			}
			tbody.innerHTML = data.map(row => {
				let cells = '';
				%s
				cells += '<td style="text-align: right;"><div class="cb-actions">' +
					'<a href="%s/edit/' + row.%s + '" class="btn btn-secondary btn-sm">Edit</a>' +
					'<a href="%s/delete/' + row.%s + '" onclick="return confirm(\'Delete item #\'+row.%s+\'?\')" class="btn btn-danger btn-sm">Delete</a>' +
				'</div></td>';
				return '<tr>' + cells + '</tr>';
			}).join('');
		}

		function filterTable() {
			const q = document.getElementById('cb-table-search').value.toLowerCase();
			if (!q) { renderRows(rawData); return; }
			const filtered = rawData.filter(row => JSON.stringify(row).toLowerCase().includes(q));
			renderRows(filtered);
		}

		loadData();
		</script>
	`, c.Title, c.BasePath, c.Title, tableHeaders, len(c.Columns)+1, c.BasePath, len(c.Columns)+1, c.generateRowMapperJS(), c.BasePath, c.PrimaryKey, c.BasePath, c.PrimaryKey, c.PrimaryKey)

	c.Engine.RenderLayout(w, r, c.Title, template.HTML(body))
}

func (c *Controller) generateRowMapperJS() string {
	js := ""
	for _, col := range c.Columns {
		if col.Type == ColImage {
			js += fmt.Sprintf(`cells += '<td><img src="' + (row.%s || '') + '" style="height:32px;border-radius:4px;"/></td>';`+"\n", col.Name)
		} else if col.Type == ColBadge {
			js += fmt.Sprintf(`cells += '<td><span class="cb-badge cb-badge-success">' + (row.%s || '') + '</span></td>';`+"\n", col.Name)
		} else {
			js += fmt.Sprintf(`cells += '<td>' + (row.%s !== undefined ? row.%s : '') + '</td>';`+"\n", col.Name, col.Name)
		}
	}
	return js
}

func (c *Controller) handleCreateHTML(w http.ResponseWriter, r *http.Request) {
	formFields := ""
	for _, f := range c.Forms {
		formFields += fmt.Sprintf(`
			<div class="cb-form-group">
				<label>%s %s</label>
				<input type="%s" name="%s" class="cb-form-control" placeholder="%s" %s />
				%s
			</div>
		`, f.Label, ifThen(f.Required, "<span style='color:red;'>*</span>", ""), mapInputType(f.Type), f.Name, f.Placeholder, ifThen(f.Required, "required", ""), ifThen(f.HelpText != "", fmt.Sprintf("<span class='cb-help-text'>%s</span>", f.HelpText), ""))
	}

	body := fmt.Sprintf(`
		<div class="cb-card" style="max-width: 680px; margin: 0 auto;">
			<h3 style="font-size: 1.25rem; font-weight: 800; margin-bottom: 1.5rem;">Add New %s</h3>
			<form method="POST" action="%s/add">
				%s
				<div style="display: flex; gap: 10px; margin-top: 2rem;">
					<button type="submit" class="btn btn-primary">Save Data</button>
					<a href="%s" class="btn btn-secondary">Cancel</a>
				</div>
			</form>
		</div>
	`, c.Title, c.BasePath, formFields, c.BasePath)

	c.Engine.RenderLayout(w, r, "Add "+c.Title, template.HTML(body))
}

func (c *Controller) handleEditHTML(w http.ResponseWriter, r *http.Request, id string) {
	formFields := ""
	for _, f := range c.Forms {
		formFields += fmt.Sprintf(`
			<div class="cb-form-group">
				<label>%s %s</label>
				<input type="%s" name="%s" id="field_%s" class="cb-form-control" placeholder="%s" %s />
			</div>
		`, f.Label, ifThen(f.Required, "<span style='color:red;'>*</span>", ""), mapInputType(f.Type), f.Name, f.Name, f.Placeholder, ifThen(f.Required, "required", ""))
	}

	body := fmt.Sprintf(`
		<div class="cb-card" style="max-width: 680px; margin: 0 auto;">
			<h3 style="font-size: 1.25rem; font-weight: 800; margin-bottom: 1.5rem;">Edit %s (#%s)</h3>
			<form method="POST" action="%s/edit/%s">
				%s
				<div style="display: flex; gap: 10px; margin-top: 2rem;">
					<button type="submit" class="btn btn-primary">Update Data</button>
					<a href="%s" class="btn btn-secondary">Cancel</a>
				</div>
			</form>
		</div>
	`, c.Title, id, c.BasePath, id, formFields, c.BasePath)

	c.Engine.RenderLayout(w, r, "Edit "+c.Title, template.HTML(body))
}

func (c *Controller) handleDataJSON(w http.ResponseWriter, r *http.Request) {
	// Sample dynamic row list
	rows := []map[string]interface{}{
		{"id": 1, "name": "Apple MacBook Pro 16 M3 Max", "price": "Rp 42.000.000", "stock": 14, "status": "In Stock"},
		{"id": 2, "name": "Keychron Q1 Pro Wireless Keyboard", "price": "Rp 2.850.000", "stock": 25, "status": "In Stock"},
		{"id": 3, "name": "LG UltraFine 4K Display 32-inch", "price": "Rp 11.500.000", "stock": 4, "status": "Low Stock"},
	}

	if c.HookRowListing != nil {
		for i, r := range rows {
			rows[i] = c.HookRowListing(r)
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

func ifThen(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func mapInputType(t InputType) string {
	switch t {
	case InputNumber:
		return "number"
	case InputPassword:
		return "password"
	case InputDate:
		return "date"
	case InputEmail:
		return "email"
	default:
		return "text"
	}
}
