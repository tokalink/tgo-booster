package booster

import "github.com/tokalink/tgo-booster/pkg/cb"

// Type aliases for seamless root package import
type (
	Engine           = cb.Engine
	Controller       = cb.Controller
	Column           = cb.Column
	Field            = cb.Field
	Option           = cb.Option
	User             = cb.User
	UserAccount      = cb.UserAccount
	UserManager      = cb.UserManager
	MenuItem         = cb.MenuItem
	APIToken         = cb.APIToken
	StatWidget       = cb.StatWidget
	WidgetCard       = cb.WidgetCard
	TopAction        = cb.TopAction
	RowAction        = cb.RowAction
	FilterTab        = cb.FilterTab
	Context          = cb.Context
	HookFunc         = cb.HookFunc
	ColumnType       = cb.ColumnType
	InputType        = cb.InputType
	DataProvider     = cb.DataProvider
	MemoryStore      = cb.MemoryStore
	SQLStore         = cb.SQLStore
	SQLDB            = cb.SQLDB
	ModuleDefinition = cb.ModuleDefinition
	Role             = cb.Role
	PermissionMatrix = cb.PermissionMatrix
	CustomAdminPage  = cb.CustomAdminPage
	Tenant           = cb.Tenant
	TenantManager    = cb.TenantManager
)

const (
	ColText     = cb.ColText
	ColImage    = cb.ColImage
	ColNumber   = cb.ColNumber
	ColMoney    = cb.ColMoney
	ColBadge    = cb.ColBadge
	ColDateTime = cb.ColDateTime
	ColDate     = cb.ColDate
	ColEmail    = cb.ColEmail

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

// RegisterConstraintMessages mendaftarkan pesan ramah untuk constraint database.
func RegisterConstraintMessages(m map[string]string) { cb.RegisterConstraintMessages(m) }

// FriendlyDBError menerjemahkan galat database menjadi kalimat untuk pengguna.
func FriendlyDBError(err error) string { return cb.FriendlyDBError(err) }

// SetSessionSecret mengganti rahasia penandatangan sesi (min. 32 byte). WAJIB dipanggil aplikasi sebelum menerima lalu lintas.
func SetSessionSecret(b []byte) error { return cb.SetSessionSecret(b) }

// SessionSecretIsDefault: true bila rahasia sesi masih nilai bawaan.
func SessionSecretIsDefault() bool { return cb.SessionSecretIsDefault() }

// ValidateUsername menormalkan dan memvalidasi username (3–32 karakter, tanpa @, bukan nama cadangan).
func ValidateUsername(u string) (string, error) { return cb.ValidateUsername(u) }

// SecondFactorProvider: penyedia langkah kedua login (TOTP dsb.).
type SecondFactorProvider = cb.SecondFactorProvider
