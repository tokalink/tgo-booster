package cb

import (
	"context"
	"net/http"
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
)

// Column represents a CRUDBooster Data Grid column
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

// Field represents a CRUDBooster Form Input field
type Field struct {
	Label        string     `json:"label"`
	Name         string     `json:"name"`
	Type         InputType  `json:"type"`
	Required     bool       `json:"required"`
	Placeholder  string     `json:"placeholder,omitempty"`
	DefaultValue string     `json:"default_value,omitempty"`
	HelpText     string     `json:"help_text,omitempty"`
	Options      []Option   `json:"options,omitempty"`
	DataQuery    string     `json:"data_query,omitempty"`
	UploadPath   string     `json:"upload_path,omitempty"`
	ReadOnly     bool       `json:"read_only,omitempty"`
	Validation   string     `json:"validation,omitempty"`
}

// MenuItem represents a dynamic sidebar menu item
type MenuItem struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Icon        string     `json:"icon"`
	Path        string     `json:"path"`
	Badge       string     `json:"badge,omitempty"`
	BadgeColor  string     `json:"badge_color,omitempty"`
	PrivilegeID string     `json:"privilege_id,omitempty"`
	Children    []MenuItem `json:"children,omitempty"`
	IsActive    bool       `json:"is_active,omitempty"`
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
