package cb

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// ColumnType defines the data grid column format
type ColumnType string

const (
	ColText     ColumnType = "text"
	ColImage    ColumnType = "image"
	ColNumber   ColumnType = "number"
	ColMoney    ColumnType = "money"
	ColBadge    ColumnType = "badge"
	ColDateTime ColumnType = "datetime"
	ColDate     ColumnType = "date"
	ColEmail    ColumnType = "email"
	ColURL      ColumnType = "url"
	ColCustom   ColumnType = "custom"
)

// InputType defines the form control input type
type InputType string

const (
	InputText     InputType = "text"
	InputNumber   InputType = "number"
	InputMoney    InputType = "money"
	InputEmail    InputType = "email"
	InputPassword InputType = "password"
	InputTextarea InputType = "textarea"
	InputWYSIWYG  InputType = "wysiwyg"
	InputSelect   InputType = "select"
	InputSelect2  InputType = "select2"
	InputUpload   InputType = "upload"
	InputDate     InputType = "date"
	InputDateTime InputType = "datetime"
	InputCheckbox InputType = "checkbox"
	InputRadio    InputType = "radio"
	InputHidden   InputType = "hidden"
	InputLOV      InputType = "lov"
)

// Column represents a Data Grid column
type Column struct {
	Label      string     `json:"label"`
	Name       string     `json:"name"`
	Type       ColumnType `json:"type"`
	Searchable bool       `json:"searchable"`
	Sortable   bool       `json:"sortable"`
	Width      string     `json:"width,omitempty"`
	JoinTable  string     `json:"join_table,omitempty"`
	JoinKey    string     `json:"join_key,omitempty"`
	JoinField  string     `json:"join_field,omitempty"`
	Format     func(val interface{}, row map[string]interface{}) string `json:"-"`
}

// Option represents a key-value selection choice
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Field represents a Form Input field
type Field struct {
	Label        string      `json:"label"`
	Name         string      `json:"name"`
	Type         InputType   `json:"type"`
	Required     bool        `json:"required"`
	Placeholder  string      `json:"placeholder,omitempty"`
	DefaultValue string      `json:"default_value,omitempty"`
	HelpText     string      `json:"help_text,omitempty"`
	Options      []Option    `json:"options,omitempty"`
	DataTable    string      `json:"datatable,omitempty"`     // e.g. "customers,name" or "categories,title,id"
	DisplayValue string      `json:"display_value,omitempty"` // Resolved label for LOV
	Format       string      `json:"format,omitempty"`        // e.g. "{code} - {name}" or "[code] - [name]"
	DataQuery    string      `json:"data_query,omitempty"`
	UploadPath   string      `json:"upload_path,omitempty"`
	ReadOnly     bool        `json:"read_only,omitempty"`
	Validation   string      `json:"validation,omitempty"`
	Value        interface{} `json:"value,omitempty"`
}

// MenuItem represents a dynamic sidebar menu item supporting groups and sub-menus
type MenuItem struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Type        string      `json:"type"` // "module", "url", "dropdown", "header"
	Icon        string      `json:"icon"`
	Path        string      `json:"path"`
	Target      string      `json:"target,omitempty"`       // "_self", "_blank"
	Badge       string      `json:"badge,omitempty"`
	BadgeColor  string      `json:"badge_color,omitempty"` // "primary", "success", "warning", "danger", "purple"
	ParentID    string      `json:"parent_id,omitempty"`
	Order       int         `json:"order"`
	Roles       []string    `json:"roles,omitempty"`        // List of allowed role slugs/names (empty or ["*"] = all roles)
	PrivilegeID string      `json:"privilege_id,omitempty"` // For backward compatibility
	Children    []*MenuItem `json:"children,omitempty"`
	IsActive    bool        `json:"is_active,omitempty"`
	IsOpen      bool        `json:"is_open,omitempty"`
}

// BadgeColorClass returns the CSS class corresponding to the badge color
func (m MenuItem) BadgeColorClass() string {
	switch m.BadgeColor {
	case "success", "#10b981":
		return "cb-badge-success"
	case "warning", "#f59e0b":
		return "cb-badge-warning"
	case "danger", "#f43f5e":
		return "cb-badge-danger"
	case "purple", "pro", "#a855f7", "#6366f1":
		return "cb-badge-pro"
	default:
		return "cb-badge-primary"
	}
}

// User represents an authenticated admin user (cb_users)
type User struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	Photo       string    `json:"photo"`
	PrivilegeID string    `json:"privilege_id"`
	RoleName    string    `json:"role_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Log represents an audit trail activity log (cb_logs)
type Log struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	UserName    string    `json:"user_name"`
	Description string    `json:"description"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

// StatWidget represents a Dashboard KPI Card
type StatWidget struct {
	Title      string `json:"title"`
	Value      string `json:"value"`
	Icon       string `json:"icon"`
	IconBg     string `json:"icon_bg"`
	SubText    string `json:"sub_text"`
	IsPositive bool   `json:"is_positive"`
}

// Context wraps request lifecycle for hooks
type Context struct {
	Ctx        context.Context
	Request    *http.Request
	User       *User
	TenantSlug string
}

// HookFunc represents a lifecycle hook function signature
type HookFunc func(c *Context, data map[string]interface{}) error

// DataProvider defines the data access contract for MVC models
type DataProvider interface {
	FindAll(ctx *Context) ([]map[string]interface{}, error)
	FindByID(ctx *Context, id string) (map[string]interface{}, error)
	Create(ctx *Context, data map[string]interface{}) error
	Update(ctx *Context, id string, data map[string]interface{}) error
	Delete(ctx *Context, id string) error
}

// WidgetCard represents an executive KPI stat card atop module index tables
type WidgetCard struct {
	Title     string `json:"title"`
	Value     string `json:"value"`
	Subtext   string `json:"subtext,omitempty"`
	Trend     string `json:"trend,omitempty"`
	TrendType string `json:"trend_type,omitempty"` // "positive", "negative", "neutral"
	Icon      string `json:"icon,omitempty"`
	Color     string `json:"color,omitempty"`      // "blue", "green", "purple", "amber"
	ColSpan   int    `json:"col_span,omitempty"`   // 1 to 12 (3 = 1/4 lebar, 4 = 1/3 lebar, 6 = 1/2 lebar, 12 = full)
	Width     string `json:"width,omitempty"`      // "col-3", "col-4", "col-6", "col-12", "half", "full", "1/2", "1/3", "1/4"
	Size      string `json:"size,omitempty"`       // "sm" (compact), "md" (default), "lg" (hero)
	Style     string `json:"style,omitempty"`      // custom inline CSS styling
}

// GetColSpan resolves the 12-column grid span for the widget card
func (w WidgetCard) GetColSpan() int {
	if w.ColSpan > 0 && w.ColSpan <= 12 {
		return w.ColSpan
	}
	switch strings.ToLower(strings.TrimSpace(w.Width)) {
	case "col-1", "1":
		return 1
	case "col-2", "2":
		return 2
	case "col-3", "col-sm-3", "col-md-3", "3", "1/4", "quarter":
		return 3
	case "col-4", "col-sm-4", "col-md-4", "4", "1/3", "third":
		return 4
	case "col-5", "5":
		return 5
	case "col-6", "col-sm-6", "col-md-6", "6", "1/2", "half":
		return 6
	case "col-7", "7":
		return 7
	case "col-8", "col-sm-8", "col-md-8", "8", "2/3":
		return 8
	case "col-9", "col-sm-9", "col-md-9", "9", "3/4":
		return 9
	case "col-10", "10":
		return 10
	case "col-11", "11":
		return 11
	case "col-12", "col-sm-12", "col-md-12", "12", "full":
		return 12
	default:
		return 4 // default 1/3 (3 cards per row)
	}
}

// GetSizeClass returns the CSS size modifier class
func (w WidgetCard) GetSizeClass() string {
	switch strings.ToLower(strings.TrimSpace(w.Size)) {
	case "sm", "compact", "small":
		return "cb-stat-card-sm"
	case "lg", "large", "hero":
		return "cb-stat-card-lg"
	default:
		return ""
	}
}

// TopAction represents custom toolbar buttons on module tables (CRUDBooster index_button)
type TopAction struct {
	Label   string `json:"label"`
	URL     string `json:"url,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Class   string `json:"class,omitempty"` // e.g. "btn-secondary", "btn-primary", "btn-danger"
	OnClick string `json:"onclick,omitempty"`
	Target  string `json:"target,omitempty"`
}

// RowAction represents custom per-row action buttons (CRUDBooster addaction)
type RowAction struct {
	Label     string                                `json:"label"`
	URL       string                                `json:"url,omitempty"`   // supports %v pattern e.g. "/admin/orders/invoice/%v"
	Icon      string                                `json:"icon,omitempty"`
	Color     string                                `json:"color,omitempty"` // "accent", "edit", "delete", "success", "warning"
	Target    string                                `json:"target,omitempty"`
	OnClick   string                                `json:"onclick,omitempty"`
	Condition func(row map[string]interface{}) bool `json:"-"`
}

// FilterTab represents quick-filtering pill tabs atop index views
type FilterTab struct {
	Label    string `json:"label"`
	Column   string `json:"column,omitempty"`
	Value    string `json:"value,omitempty"`
	Count    int    `json:"count,omitempty"`
	IsActive bool   `json:"is_active,omitempty"`
}

// ModuleDefinition represents the persistent declaration of a generated CRUD module ala CRUDBooster
type ModuleDefinition struct {
	Title        string       `json:"title"`
	Table        string       `json:"table"`
	Icon         string       `json:"icon"`
	PrimaryKey   string       `json:"primary_key"`
	OrderBy      string       `json:"order_by"`
	Columns      []Column     `json:"columns"`
	Forms        []Field      `json:"forms"`
	ProviderType string       `json:"provider_type"` // "sql" or "memory"
	Widgets      []WidgetCard `json:"widgets,omitempty"`
}

// PermissionMatrix represents granular CRUD permissions for a module (CRUDBooster privilege matrix)
type PermissionMatrix struct {
	IsVisible bool `json:"is_visible"`
	CanCreate bool `json:"can_create"`
	CanRead   bool `json:"can_read"`
	CanUpdate bool `json:"can_update"`
	CanDelete bool `json:"can_delete"`
}

// Role represents a Privilege Role with its module permissions matrix (cb_roles / cb_privileges)
type Role struct {
	ID           string                      `json:"id"`
	Name         string                      `json:"name"`
	Slug         string                      `json:"slug"`
	IsSuperadmin bool                        `json:"is_superadmin"`
	Description  string                      `json:"description"`
	UsersCount   int                         `json:"users_count"`
	Permissions  map[string]PermissionMatrix `json:"permissions"` // map[moduleTable]PermissionMatrix
	CreatedAt    time.Time                   `json:"created_at"`
}

// CanAccess evaluates whether a role is permitted to perform an action on a module
func (r *Role) CanAccess(moduleTable, action string) bool {
	if r == nil || r.IsSuperadmin || strings.EqualFold(r.Name, "superadmin") || strings.EqualFold(r.Slug, "superadmin") {
		return true
	}
	if r.Permissions == nil {
		return false
	}
	matrix, ok := r.Permissions[moduleTable]
	if !ok {
		matrix, ok = r.Permissions[strings.ToLower(moduleTable)]
		if !ok {
			return false
		}
	}
	switch strings.ToLower(action) {
	case "visible", "view", "menu":
		return matrix.IsVisible
	case "create", "add":
		return matrix.CanCreate
	case "read", "index", "detail", "data":
		return matrix.CanRead
	case "update", "edit":
		return matrix.CanUpdate
	case "delete", "remove", "bulk":
		return matrix.CanDelete
	default:
		return matrix.CanRead
	}
}



