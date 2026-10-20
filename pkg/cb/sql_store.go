package cb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ErrTenantRequired dikembalikan bila operasi pada store ber-tenant dijalankan tanpa tenant saat StrictTenancy aktif.
var ErrTenantRequired = errors.New("tenant tidak ditentukan")

// SQLDB is a type alias for *sql.DB
type SQLDB = sql.DB

// SQLStore is a multi-dialect SQL DataProvider supporting MySQL, SQLite, and PostgreSQL.
// It supports native SaaS row-level multi-tenancy with automatic column injection and filtering.
type SQLStore struct {
	db            *sql.DB
	tableName     string
	idKey         string
	driver        string
	tenantScoped  atomic.Bool
	strictTenancy atomic.Bool
	tenantCol     string
}

// NewSQLStore creates a new SQL DataProvider for the specified table and primary key.
func NewSQLStore(db *sql.DB, tableName string, idKey ...string) *SQLStore {
	key := "id"
	if len(idKey) > 0 && idKey[0] != "" {
		key = idKey[0]
	}
	driver := "mysql"
	if db != nil {
		drvType := strings.ToLower(fmt.Sprintf("%T", db.Driver()))
		if strings.Contains(drvType, "sqlite") {
			driver = "sqlite3"
		} else if strings.Contains(drvType, "pq") || strings.Contains(drvType, "postgres") || strings.Contains(drvType, "pgx") || strings.Contains(drvType, "stdlib") {
			driver = "postgres"
		}
	}
	return &SQLStore{
		db:        db,
		tableName: tableName,
		idKey:     key,
		driver:    driver,
		tenantCol: "tenant_id",
	}
}

// SetDriver configures the SQL dialect ("mysql", "sqlite", "sqlite3", "postgres", "pgsql")
func (s *SQLStore) SetDriver(driver string) *SQLStore {
	s.driver = strings.ToLower(driver)
	return s
}

// SetTenantScoped configures whether this table isolates records by tenant ID (defaults to "tenant_id")
func (s *SQLStore) SetTenantScoped(scoped bool, col ...string) *SQLStore {
	s.tenantScoped.Store(scoped)
	if len(col) > 0 && col[0] != "" {
		s.tenantCol = col[0]
	}
	return s
}

// SetStrictTenancy configures whether operations without an active tenant are rejected when tenant-scoped
func (s *SQLStore) SetStrictTenancy(strict bool) *SQLStore {
	s.strictTenancy.Store(strict)
	return s
}

// IsTenantScoped returns true if the store enforces tenant isolation
func (s *SQLStore) IsTenantScoped() bool {
	return s.tenantScoped.Load()
}

// TenantCol returns the column name used for tenant isolation (default: "tenant_id")
func (s *SQLStore) TenantCol() string {
	if s.tenantCol == "" {
		return "tenant_id"
	}
	return s.tenantCol
}

// DB returns the underlying *sql.DB connection pool
func (s *SQLStore) DB() *sql.DB {
	return s.db
}

// TableName returns the configured table name
func (s *SQLStore) TableName() string {
	return s.tableName
}

// isPostgres checks if the driver uses PostgreSQL syntax
func (s *SQLStore) isPostgres() bool {
	return s.driver == "postgres" || s.driver == "pgsql"
}

// resolveTenant retrieves the active tenant slug or fallback to default if scoped
func (s *SQLStore) resolveTenant(ctx *Context) string {
	if ctx != nil {
		if ctx.TenantSlug == "*" {
			return "*"
		}
		if ctx.TenantSlug != "" {
			return ctx.TenantSlug
		}
	}
	if s.tenantScoped.Load() {
		if s.strictTenancy.Load() {
			return ""
		}
		return "default"
	}
	return ""
}

// withTx executes a function inside a transaction, setting PostgreSQL tenant config if scoped
func (s *SQLStore) withTx(ctx *Context, fn func(tx *sql.Tx) error) error {
	if s.db == nil {
		return errors.New("database connection is nil")
	}
	cCtx := context.Background()
	if ctx != nil && ctx.Ctx != nil {
		cCtx = ctx.Ctx
	}

	tx, err := s.db.BeginTx(cCtx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tenant := s.resolveTenant(ctx)
	if s.isPostgres() && s.tenantScoped.Load() && tenant != "" && tenant != "*" {
		if _, err := tx.ExecContext(cCtx, "SELECT set_config('app.tenant_id', $1, true)", tenant); err != nil {
			return err
		}
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

// FindAll returns all records ordered by primary key descending, scoped by tenant if enabled
func (s *SQLStore) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	scoped := s.tenantScoped.Load()
	tenantSlug := s.resolveTenant(ctx)
	if scoped && tenantSlug == "" {
		return nil, ErrTenantRequired
	}

	var query string
	var args []interface{}

	if scoped && tenantSlug != "" && tenantSlug != "*" {
		col := s.TenantCol()
		if s.isPostgres() {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 ORDER BY %s DESC", s.tableName, col, s.idKey)
		} else {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ? ORDER BY %s DESC", s.tableName, col, s.idKey)
		}
		args = append(args, tenantSlug)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s ORDER BY %s DESC", s.tableName, s.idKey)
	}

	var results []map[string]interface{}

	if s.isPostgres() {
		err := s.withTx(ctx, func(tx *sql.Tx) error {
			cCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				cCtx = ctx.Ctx
			}
			rows, err := tx.QueryContext(cCtx, query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			for rows.Next() {
				rowMap, err := s.scanRow(rows, cols)
				if err != nil {
					return err
				}
				results = append(results, rowMap)
			}
			return rows.Err()
		})
		if err != nil {
			return nil, err
		}
	} else {
		rows, err := s.db.Query(query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			rowMap, err := s.scanRow(rows, cols)
			if err != nil {
				return nil, err
			}
			results = append(results, rowMap)
		}

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	if results == nil {
		results = make([]map[string]interface{}, 0)
	}
	return results, nil
}

// FindByID retrieves a single row matching primary key ID and tenant scope
func (s *SQLStore) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	scoped := s.tenantScoped.Load()
	tenantSlug := s.resolveTenant(ctx)
	if scoped && tenantSlug == "" {
		return nil, ErrTenantRequired
	}

	var query string
	var args []interface{}

	if scoped && tenantSlug != "" && tenantSlug != "*" {
		col := s.TenantCol()
		if s.isPostgres() {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 AND %s = $2 LIMIT 1", s.tableName, s.idKey, col)
		} else {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ? AND %s = ? LIMIT 1", s.tableName, s.idKey, col)
		}
		args = append(args, id, tenantSlug)
	} else {
		if s.isPostgres() {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", s.tableName, s.idKey)
		} else {
			query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ? LIMIT 1", s.tableName, s.idKey)
		}
		args = append(args, id)
	}

	var record map[string]interface{}

	if s.isPostgres() {
		err := s.withTx(ctx, func(tx *sql.Tx) error {
			cCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				cCtx = ctx.Ctx
			}
			rows, err := tx.QueryContext(cCtx, query, args...)
			if err != nil {
				return err
			}
			defer rows.Close()

			cols, err := rows.Columns()
			if err != nil {
				return err
			}

			if rows.Next() {
				rec, err := s.scanRow(rows, cols)
				if err != nil {
					return err
				}
				record = rec
				return nil
			}
			return fmt.Errorf("record with %s=%s not found in %s", s.idKey, id, s.tableName)
		})
		if err != nil {
			return nil, err
		}
		return record, nil
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if rows.Next() {
		return s.scanRow(rows, cols)
	}

	return nil, fmt.Errorf("record with %s=%s not found in %s", s.idKey, id, s.tableName)
}

// columnNameRe: satu-satunya bentuk nama kolom yang boleh disambung ke SQL. Key data berasal dari input pengguna
// (form/CSV/JSON); tanpa validasi ini key berbentuk ekspresi menjadi injeksi SQL lewat nama kolom.
var columnNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// validateColumns menolak key data yang bukan identifier SQL polos.
func validateColumns(data map[string]interface{}) error {
	for k := range data {
		if !columnNameRe.MatchString(k) {
			return fmt.Errorf("nama kolom tidak valid: %q", k)
		}
	}
	return nil
}

// Create inserts a new row into the table, automatically injecting the tenant ID if scoped
func (s *SQLStore) Create(ctx *Context, data map[string]interface{}) error {
	scoped := s.tenantScoped.Load()
	tenantSlug := s.resolveTenant(ctx)
	if scoped && tenantSlug == "" {
		return ErrTenantRequired
	}

	if scoped && tenantSlug != "" && tenantSlug != "*" {
		col := s.TenantCol()
		data[col] = tenantSlug
	}

	if err := validateColumns(data); err != nil {
		return err
	}

	var cols []string
	var placeholders []string
	var values []interface{}

	idx := 1
	for k, v := range data {
		cols = append(cols, k)
		if s.isPostgres() {
			placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		} else {
			placeholders = append(placeholders, "?")
		}
		values = append(values, v)
		idx++
	}

	if len(cols) == 0 {
		return errors.New("no columns provided for insert")
	}

	if s.isPostgres() {
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
			s.tableName,
			strings.Join(cols, ", "),
			strings.Join(placeholders, ", "),
			s.idKey,
		)
		var insertedID interface{}
		err := s.withTx(ctx, func(tx *sql.Tx) error {
			cCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				cCtx = ctx.Ctx
			}
			return tx.QueryRowContext(cCtx, query, values...).Scan(&insertedID)
		})
		if err != nil {
			return err
		}
		if insertedID != nil {
			data[s.idKey] = fmt.Sprint(insertedID)
		}
		return nil
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		s.tableName,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	res, err := s.db.Exec(query, values...)
	if err != nil {
		return err
	}

	if lastID, err := res.LastInsertId(); err == nil && lastID > 0 {
		data[s.idKey] = strconv.FormatInt(lastID, 10)
	}

	return nil
}

// Update modifies an existing row identified by primary key ID and tenant scope
func (s *SQLStore) Update(ctx *Context, id string, data map[string]interface{}) error {
	scoped := s.tenantScoped.Load()
	tenantSlug := s.resolveTenant(ctx)
	if scoped && tenantSlug == "" {
		return ErrTenantRequired
	}
	col := s.TenantCol()

	if err := validateColumns(data); err != nil {
		return err
	}

	var setClauses []string
	var values []interface{}

	idx := 1
	for k, v := range data {
		if k == s.idKey || (scoped && k == col) {
			continue // Do not update primary key or tenant column
		}
		if s.isPostgres() {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, idx))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", k))
		}
		values = append(values, v)
		idx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	values = append(values, id)
	var query string
	if scoped && tenantSlug != "" && tenantSlug != "*" {
		values = append(values, tenantSlug)
		if s.isPostgres() {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d AND %s = $%d",
				s.tableName,
				strings.Join(setClauses, ", "),
				s.idKey,
				idx,
				col,
				idx+1,
			)
		} else {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE %s = ? AND %s = ?",
				s.tableName,
				strings.Join(setClauses, ", "),
				s.idKey,
				col,
			)
		}
	} else {
		if s.isPostgres() {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d",
				s.tableName,
				strings.Join(setClauses, ", "),
				s.idKey,
				idx,
			)
		} else {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
				s.tableName,
				strings.Join(setClauses, ", "),
				s.idKey,
			)
		}
	}

	if s.isPostgres() {
		return s.withTx(ctx, func(tx *sql.Tx) error {
			cCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				cCtx = ctx.Ctx
			}
			_, err := tx.ExecContext(cCtx, query, values...)
			return err
		})
	}

	_, err := s.db.Exec(query, values...)
	return err
}

// Delete removes a row identified by primary key ID and tenant scope
func (s *SQLStore) Delete(ctx *Context, id string) error {
	scoped := s.tenantScoped.Load()
	tenantSlug := s.resolveTenant(ctx)
	if scoped && tenantSlug == "" {
		return ErrTenantRequired
	}
	col := s.TenantCol()

	var query string
	var args []interface{}

	if scoped && tenantSlug != "" && tenantSlug != "*" {
		if s.isPostgres() {
			query = fmt.Sprintf("DELETE FROM %s WHERE %s = $1 AND %s = $2", s.tableName, s.idKey, col)
		} else {
			query = fmt.Sprintf("DELETE FROM %s WHERE %s = ? AND %s = ?", s.tableName, s.idKey, col)
		}
		args = append(args, id, tenantSlug)
	} else {
		if s.isPostgres() {
			query = fmt.Sprintf("DELETE FROM %s WHERE %s = $1", s.tableName, s.idKey)
		} else {
			query = fmt.Sprintf("DELETE FROM %s WHERE %s = ?", s.tableName, s.idKey)
		}
		args = append(args, id)
	}

	if s.isPostgres() {
		return s.withTx(ctx, func(tx *sql.Tx) error {
			cCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				cCtx = ctx.Ctx
			}
			_, err := tx.ExecContext(cCtx, query, args...)
			return err
		})
	}

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *SQLStore) scanRow(rows *sql.Rows, cols []string) (map[string]interface{}, error) {
	columnPointers := make([]interface{}, len(cols))
	columnValues := make([]interface{}, len(cols))
	for i := range columnValues {
		columnPointers[i] = &columnValues[i]
	}

	if err := rows.Scan(columnPointers...); err != nil {
		return nil, err
	}

	row := make(map[string]interface{}, len(cols))
	for i, colName := range cols {
		val := columnValues[i]
		switch v := val.(type) {
		case []byte:
			row[colName] = string(v)
		case time.Time:
			row[colName] = v.Format("2006-01-02 15:04:05")
		case nil:
			row[colName] = ""
		default:
			row[colName] = fmt.Sprint(v)
		}
	}
	return row, nil
}

// AutoMigrate creates the table in the database if it doesn't already exist, inferring column types from Field definitions.
// If tenantScoped is true, it automatically adds the tenant isolation column (default: "tenant_id") and indexes it.
func (s *SQLStore) AutoMigrate(fields ...Field) error {
	if s.db == nil || s.tableName == "" {
		return errors.New("database connection or table name not set")
	}

	colDefs := []string{}
	// Primary key definition
	switch s.driver {
	case "sqlite", "sqlite3":
		colDefs = append(colDefs, fmt.Sprintf("%s INTEGER PRIMARY KEY AUTOINCREMENT", s.idKey))
	case "postgres", "pgsql":
		colDefs = append(colDefs, fmt.Sprintf("%s SERIAL PRIMARY KEY", s.idKey))
	default: // mysql
		colDefs = append(colDefs, fmt.Sprintf("`%s` INT AUTO_INCREMENT PRIMARY KEY", s.idKey))
	}

	// Add tenant isolation column if tenant-scoped
	if s.tenantScoped.Load() {
		col := s.TenantCol()
		hasTenantCol := false
		for _, f := range fields {
			if f.Name == col {
				hasTenantCol = true
				break
			}
		}
		if !hasTenantCol {
			if s.driver == "mysql" {
				colDefs = append(colDefs, fmt.Sprintf("`%s` VARCHAR(64) NOT NULL DEFAULT 'default'", col))
			} else {
				colDefs = append(colDefs, fmt.Sprintf("%s VARCHAR(64) NOT NULL DEFAULT 'default'", col))
			}
		}
	}

	for _, f := range fields {
		if f.Name == "" || f.Name == s.idKey || (s.tenantScoped.Load() && f.Name == s.TenantCol()) {
			continue
		}

		var sqlType string
		switch f.Type {
		case InputNumber:
			sqlType = "INT"
		case InputWYSIWYG, InputTextarea:
			sqlType = "TEXT"
		case InputMoney:
			sqlType = "VARCHAR(100)"
		case InputDate, InputDateTime:
			switch s.driver {
			case "sqlite", "sqlite3":
				sqlType = "TEXT"
			case "postgres", "pgsql":
				sqlType = "TIMESTAMP"
			default:
				sqlType = "TIMESTAMP NULL"
			}
		default:
			sqlType = "VARCHAR(255)"
		}

		if s.driver == "mysql" {
			colDefs = append(colDefs, fmt.Sprintf("`%s` %s", f.Name, sqlType))
		} else {
			colDefs = append(colDefs, fmt.Sprintf("%s %s", f.Name, sqlType))
		}
	}

	// Add standard timestamps
	switch s.driver {
	case "sqlite", "sqlite3":
		colDefs = append(colDefs, "created_at DATETIME DEFAULT CURRENT_TIMESTAMP")
		colDefs = append(colDefs, "updated_at DATETIME DEFAULT CURRENT_TIMESTAMP")
	case "postgres", "pgsql":
		colDefs = append(colDefs, "created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
		colDefs = append(colDefs, "updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
	default: // mysql
		colDefs = append(colDefs, "`created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP")
		colDefs = append(colDefs, "`updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP")
	}

	var query string
	if s.driver == "mysql" {
		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` (\n  %s\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;",
			s.tableName, strings.Join(colDefs, ",\n  "))
	} else {
		query = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n);",
			s.tableName, strings.Join(colDefs, ",\n  "))
	}

	if _, err := s.db.Exec(query); err != nil {
		return err
	}

	// Ensure tenant column exists and is indexed if tenant-scoped
	if s.tenantScoped.Load() {
		col := s.TenantCol()
		var alterQuery string
		if s.driver == "mysql" {
			alterQuery = fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` VARCHAR(64) NOT NULL DEFAULT 'default';", s.tableName, col)
		} else {
			alterQuery = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s VARCHAR(64) NOT NULL DEFAULT 'default';", s.tableName, col)
		}
		_, _ = s.db.Exec(alterQuery) // safely ignore error if column already exists

		idxName := fmt.Sprintf("idx_%s_%s", s.tableName, col)
		var idxQuery string
		if s.driver == "mysql" {
			idxQuery = fmt.Sprintf("CREATE INDEX `%s` ON `%s` (`%s`);", idxName, s.tableName, col)
		} else {
			idxQuery = fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s (%s);", idxName, s.tableName, col)
		}
		_, _ = s.db.Exec(idxQuery) // safely ignore error if index already exists
	}

	return nil
}
