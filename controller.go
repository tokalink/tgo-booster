package booster

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/middleware"
)

// Controller represents a declarative CRUDBooster-style module controller
type Controller struct {
	Title       string
	Table       string
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
}

func (c *Controller) initDefaults() {
	if c.PrimaryKey == "" {
		c.PrimaryKey = "id"
	}
	if c.OrderBy == "" {
		c.OrderBy = fmt.Sprintf("%s DESC", c.PrimaryKey)
	}
	if c.BasePath == "" {
		c.BasePath = fmt.Sprintf("/admin/%s", c.Table)
	}
}

// ServeHTTP handles routing for Index (GET), Add (GET/POST), Edit (GET/POST), Delete (POST)
func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.initDefaults()

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
	tenant := middleware.GetTenant(r.Context())
	if tenant == "" {
		tenant = "public"
	}

	tableHeaders := ""
	for _, col := range c.Columns {
		tableHeaders += fmt.Sprintf("<th>%s</th>", col.Label)
	}
	tableHeaders += "<th style='width: 120px; text-align: right;'>Actions</th>"

	body := fmt.Sprintf(`
		<div class="card">
			<div class="table-toolbar">
				<div>
					<input type="text" class="search-input" placeholder="Search %s..." id="table-search" />
				</div>
				<div>
					<a href="%s/add" class="btn btn-primary">+ Add New</a>
				</div>
			</div>
			<div style="overflow-x: auto;">
				<table class="custom-table" id="crud-table">
					<thead>
						<tr>%s</tr>
					</thead>
					<tbody id="crud-tbody">
						<tr><td colspan="%d" style="text-align: center;">Loading data...</td></tr>
					</tbody>
				</table>
			</div>
		</div>

		<script>
		async function loadData() {
			try {
				const res = await fetch('%s/data', { headers: { 'Accept': 'application/json' } });
				const data = await res.json();
				const tbody = document.getElementById('crud-tbody');
				if (!data || data.length === 0) {
					tbody.innerHTML = '<tr><td colspan="%d" style="text-align: center; color: #94a3b8;">No records found</td></tr>';
					return;
				}
				tbody.innerHTML = data.map(row => {
					let cells = '';
					%s
					cells += '<td style="text-align: right;" class="actions-cell">' +
						'<a href="%s/edit/' + row.%s + '" class="btn btn-secondary btn-sm">Edit</a>' +
						'<a href="%s/delete/' + row.%s + '" onclick="return confirm(\'Delete this item?\')" class="btn btn-danger btn-sm">Delete</a>' +
					'</td>';
					return '<tr>' + cells + '</tr>';
				}).join('');
			} catch(e) {
				console.error(e);
			}
		}
		loadData();
		</script>
	`, c.Title, c.BasePath, tableHeaders, len(c.Columns)+1, c.BasePath, len(c.Columns)+1, c.generateRowMapperJS(), c.BasePath, c.PrimaryKey, c.BasePath, c.PrimaryKey)

	c.renderPage(w, c.Title, tenant, template.HTML(body))
}

func (c *Controller) generateRowMapperJS() string {
	js := ""
	for _, col := range c.Columns {
		if col.Type == TypeImage {
			js += fmt.Sprintf(`cells += '<td><img src="' + (row.%s || '') + '" style="height:32px;border-radius:4px;"/></td>';`+"\n", col.Name)
		} else if col.Type == TypeBadge {
			js += fmt.Sprintf(`cells += '<td><span class="badge badge-success">' + (row.%s || '') + '</span></td>';`+"\n", col.Name)
		} else {
			js += fmt.Sprintf(`cells += '<td>' + (row.%s !== undefined ? row.%s : '') + '</td>';`+"\n", col.Name, col.Name)
		}
	}
	return js
}

func (c *Controller) handleCreateHTML(w http.ResponseWriter, r *http.Request) {
	tenant := middleware.GetTenant(r.Context())
	formFields := ""
	for _, f := range c.Forms {
		formFields += fmt.Sprintf(`
			<div class="form-group">
				<label>%s %s</label>
				<input type="%s" name="%s" class="form-control" placeholder="%s" %s />
				%s
			</div>
		`, f.Label, ifThen(f.Required, "<span style='color:red;'>*</span>", ""), mapInputType(f.Type), f.Name, f.Placeholder, ifThen(f.Required, "required", ""), ifThen(f.HelpText != "", fmt.Sprintf("<small style='color:#94a3b8;'>%s</small>", f.HelpText), ""))
	}

	body := fmt.Sprintf(`
		<div class="card" style="max-width: 680px; margin: 0 auto;">
			<h3 style="margin-bottom: 1.5rem;">Create New %s</h3>
			<form method="POST" action="%s/add">
				%s
				<div style="display: flex; gap: 10px; margin-top: 2rem;">
					<button type="submit" class="btn btn-primary">Save Data</button>
					<a href="%s" class="btn btn-secondary">Cancel</a>
				</div>
			</form>
		</div>
	`, c.Title, c.BasePath, formFields, c.BasePath)

	c.renderPage(w, "Add "+c.Title, tenant, template.HTML(body))
}

func (c *Controller) handleEditHTML(w http.ResponseWriter, r *http.Request, id string) {
	tenant := middleware.GetTenant(r.Context())
	formFields := ""
	for _, f := range c.Forms {
		formFields += fmt.Sprintf(`
			<div class="form-group">
				<label>%s %s</label>
				<input type="%s" name="%s" id="field_%s" class="form-control" placeholder="%s" %s />
			</div>
		`, f.Label, ifThen(f.Required, "<span style='color:red;'>*</span>", ""), mapInputType(f.Type), f.Name, f.Name, f.Placeholder, ifThen(f.Required, "required", ""))
	}

	body := fmt.Sprintf(`
		<div class="card" style="max-width: 680px; margin: 0 auto;">
			<h3 style="margin-bottom: 1.5rem;">Edit %s (#%s)</h3>
			<form method="POST" action="%s/edit/%s">
				%s
				<div style="display: flex; gap: 10px; margin-top: 2rem;">
					<button type="submit" class="btn btn-primary">Update Data</button>
					<a href="%s" class="btn btn-secondary">Cancel</a>
				</div>
			</form>
		</div>
	`, c.Title, id, c.BasePath, id, formFields, c.BasePath)

	c.renderPage(w, "Edit "+c.Title, tenant, template.HTML(body))
}

func (c *Controller) handleDataJSON(w http.ResponseWriter, r *http.Request) {
	// Mock / dynamic row data for scaffolded demo
	rows := []map[string]interface{}{
		{"id": 1, "name": "Standard Laptop Pro 15", "price": "Rp 15.000.000", "stock": 42, "status": "In Stock"},
		{"id": 2, "name": "Wireless Mechanical Keyboard", "price": "Rp 1.250.000", "stock": 18, "status": "In Stock"},
		{"id": 3, "name": "Ultra-Wide Gaming Monitor", "price": "Rp 5.750.000", "stock": 5, "status": "Low Stock"},
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
		ctx := &Context{Ctx: r.Context(), Request: r, TenantSlug: middleware.GetTenant(r.Context())}
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
		ctx := &Context{Ctx: r.Context(), Request: r, TenantSlug: middleware.GetTenant(r.Context())}
		if err := c.HookBeforeEdit(ctx, data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, c.BasePath, http.StatusSeeOther)
}

func (c *Controller) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	if c.HookBeforeDelete != nil {
		ctx := &Context{Ctx: r.Context(), Request: r, TenantSlug: middleware.GetTenant(r.Context())}
		if err := c.HookBeforeDelete(ctx, map[string]interface{}{"id": id}); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	http.Redirect(w, r, c.BasePath, http.StatusSeeOther)
}

func (c *Controller) renderPage(w http.ResponseWriter, title, tenant string, content template.HTML) {
	tmpl, _ := template.New("base").Parse(AdminBaseHTML)
	data := map[string]interface{}{
		"Title":       title,
		"TenantSlug":  tenant,
		"BodyContent": content,
		"Modules": []map[string]interface{}{
			{"Title": c.Title, "Path": c.BasePath, "IsActive": true},
		},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
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
	default:
		return "text"
	}
}
