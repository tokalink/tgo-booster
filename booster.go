package booster

import "github.com/tokalink/tgo-booster/pkg/cb"

// Type aliases for seamless root package import
type (
	Engine       = cb.Engine
	Controller   = cb.Controller
	Column       = cb.Column
	Field        = cb.Field
	Option       = cb.Option
	User         = cb.User
	MenuItem     = cb.MenuItem
	StatWidget   = cb.StatWidget
	WidgetCard   = cb.WidgetCard
	TopAction    = cb.TopAction
	RowAction    = cb.RowAction
	FilterTab    = cb.FilterTab
	Context      = cb.Context
	HookFunc     = cb.HookFunc
	ColumnType   = cb.ColumnType
	InputType    = cb.InputType
	DataProvider = cb.DataProvider
	MemoryStore      = cb.MemoryStore
	SQLStore         = cb.SQLStore
	SQLDB            = cb.SQLDB
	ModuleDefinition = cb.ModuleDefinition
	Role             = cb.Role
	PermissionMatrix = cb.PermissionMatrix
	CustomAdminPage  = cb.CustomAdminPage
)

const (
	ColText       = cb.ColText
	ColImage      = cb.ColImage
	ColNumber     = cb.ColNumber
	ColMoney      = cb.ColMoney
	ColBadge      = cb.ColBadge
	ColDateTime   = cb.ColDateTime
	ColDate       = cb.ColDate
	ColEmail      = cb.ColEmail

	InputText     = cb.InputText
	InputNumber   = cb.InputNumber
	InputMoney    = cb.InputMoney
	InputEmail    = cb.InputEmail
	InputPassword = cb.InputPassword
	InputTextarea = cb.InputTextarea
	InputWYSIWYG  = cb.InputWYSIWYG
	InputSelect   = cb.InputSelect
	InputSelect2  = cb.InputSelect2
	InputUpload   = cb.InputUpload
	InputDate     = cb.InputDate
	InputDateTime = cb.InputDateTime
	InputCheckbox = cb.InputCheckbox
	InputRadio    = cb.InputRadio
	InputHidden   = cb.InputHidden
	InputLOV      = cb.InputLOV
)

// NewEngine creates a new Booster Engine
func NewEngine(appName ...string) *Engine {
	return cb.NewEngine(appName...)
}

// NewMemoryStore creates a thread-safe in-memory DataProvider
func NewMemoryStore(initialRows ...map[string]interface{}) *MemoryStore {
	return cb.NewMemoryStore(initialRows...)
}

// NewSQLStore creates a SQL DataProvider for a database table
func NewSQLStore(db *cb.SQLDB, tableName string, idKey ...string) *SQLStore {
	return cb.NewSQLStore(db, tableName, idKey...)
}
