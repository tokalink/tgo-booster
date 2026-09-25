package booster

import (
	"context"
	"net/http"
)

// ColumnType defines the presentation type of a data grid column
type ColumnType string

const (
	TypeText     ColumnType = "text"
	TypeImage    ColumnType = "image"
	TypeNumber   ColumnType = "number"
	TypeMoney    ColumnType = "money"
	TypeBadge    ColumnType = "badge"
	TypeDateTime ColumnType = "datetime"
	TypeDate     ColumnType = "date"
	TypeJoin     ColumnType = "join"
	TypeCustom   ColumnType = "custom"
)

// InputType defines the input control type in create/edit forms
type InputType string

const (
	InputText     InputType = "text"
	InputNumber   InputType = "number"
	InputMoney    InputType = "money"
	InputTextarea InputType = "textarea"
	InputSelect   InputType = "select"
	InputSelect2  InputType = "select2"
	InputWYSIWYG  InputType = "wysiwyg"
	InputUpload   InputType = "upload"
	InputDate     InputType = "date"
	InputDateTime InputType = "datetime"
	InputCheckbox InputType = "checkbox"
	InputRadio    InputType = "radio"
	InputPassword InputType = "password"
)

// Column represents a grid column specification (CRUDBooster style)
type Column struct {
	Label       string                                         `json:"label"`
	Name        string                                         `json:"name"`
	Type        ColumnType                                     `json:"type"`
	Searchable  bool                                           `json:"searchable"`
	Sortable    bool                                           `json:"sortable"`
	JoinTable   string                                         `json:"join_table,omitempty"`
	JoinKey     string                                         `json:"join_key,omitempty"`
	JoinField   string                                         `json:"join_field,omitempty"`
	FormatFunc  func(val interface{}, row map[string]interface{}) string `json:"-"`
}

// Option represents a key-value pair for select/radio options
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Field represents a form input control specification (CRUDBooster style)
type Field struct {
	Label        string   `json:"label"`
	Name         string   `json:"name"`
	Type         InputType `json:"type"`
	Required     bool     `json:"required"`
	Placeholder  string   `json:"placeholder,omitempty"`
	DefaultValue string   `json:"default_value,omitempty"`
	HelpText     string   `json:"help_text,omitempty"`
	Options      []Option `json:"options,omitempty"`
	DataQuery    string   `json:"data_query,omitempty"`
	UploadPath   string   `json:"upload_path,omitempty"`
	ReadOnly     bool     `json:"read_only,omitempty"`
}

// Context wraps request lifecycle for booster hooks
type Context struct {
	Ctx        context.Context
	Request    *http.Request
	TenantSlug string
}

// HookFunc represents a lifecycle hook function
type HookFunc func(c *Context, data map[string]interface{}) error
