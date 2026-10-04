package cb

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SQLDB is a type alias for *sql.DB
type SQLDB = sql.DB

// SQLStore is a multi-dialect SQL DataProvider supporting MySQL, SQLite, and PostgreSQL.
type SQLStore struct {
	db        *sql.DB
	tableName string
	idKey     string
	driver    string
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
		} else if strings.Contains(drvType, "pq") || strings.Contains(drvType, "postgres") {
			driver = "postgres"
		}
	}
	return &SQLStore{
		db:        db,
		tableName: tableName,
		idKey:     key,
		driver:    driver,
	}
}

// SetDriver configures the SQL dialect ("mysql", "sqlite", "sqlite3", "postgres", "pgsql")
func (s *SQLStore) SetDriver(driver string) *SQLStore {
	s.driver = strings.ToLower(driver)
	return s
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

// FindAll returns all records ordered by primary key descending
func (s *SQLStore) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s ORDER BY %s DESC", s.tableName, s.idKey)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
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

	if results == nil {
		results = make([]map[string]interface{}, 0)
	}
	return results, nil
}

// FindByID retrieves a single row matching the primary key ID
func (s *SQLStore) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	var query string
	if s.isPostgres() {
		query = fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", s.tableName, s.idKey)
	} else {
		query = fmt.Sprintf("SELECT * FROM %s WHERE %s = ? LIMIT 1", s.tableName, s.idKey)
	}

	rows, err := s.db.Query(query, id)
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

// Create inserts a new row into the table
func (s *SQLStore) Create(ctx *Context, data map[string]interface{}) error {
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
		err := s.db.QueryRow(query, values...).Scan(&insertedID)
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

// Update modifies an existing row identified by primary key ID
func (s *SQLStore) Update(ctx *Context, id string, data map[string]interface{}) error {
	var setClauses []string
	var values []interface{}

	idx := 1
	for k, v := range data {
		if k == s.idKey {
			continue // Do not update primary key
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

	_, err := s.db.Exec(query, values...)
	return err
}

// Delete removes a row identified by primary key ID
func (s *SQLStore) Delete(ctx *Context, id string) error {
	var query string
	if s.isPostgres() {
		query = fmt.Sprintf("DELETE FROM %s WHERE %s = $1", s.tableName, s.idKey)
	} else {
		query = fmt.Sprintf("DELETE FROM %s WHERE %s = ?", s.tableName, s.idKey)
	}
	_, err := s.db.Exec(query, id)
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

	for _, f := range fields {
		if f.Name == "" || f.Name == s.idKey {
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

	_, err := s.db.Exec(query)
	return err
}
