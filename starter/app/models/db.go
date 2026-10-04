package models

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/joho/godotenv"
	"github.com/tokalink/tgo-booster"
)

// DB holds the active database connection pool
var DB *sql.DB

// InitDatabase reads database configurations from .env and connects to MySQL, SQLite, or PostgreSQL
func InitDatabase() (*sql.DB, error) {
	// 1. Load environment variables (.env)
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	driver := strings.ToLower(getEnv("DB_DRIVER", getEnv("DB_CONNECTION", "mysql")))

	switch driver {
	case "sqlite", "sqlite3":
		return initSQLite()
	case "postgres", "pgsql", "postgresql":
		return initPostgres()
	case "mysql":
		return initMySQL()
	default:
		log.Printf("ℹ️ [Database] Unrecognized or in-memory driver (%s). Retaining MemoryStore.", driver)
		return nil, nil
	}
}

func initMySQL() (*sql.DB, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USERNAME", "root")
	pass := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_DATABASE", "tgo_booster")

	// Ensure database exists
	serverDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/?parseTime=true", user, pass, host, port)
	serverDB, err := sql.Open("mysql", serverDSN)
	if err == nil {
		_, _ = serverDB.Exec(fmt.Sprintf(
			"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
			dbName,
		))
		_ = serverDB.Close()
	}

	appDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, dbName)
	db, err := sql.Open("mysql", appDSN)
	if err != nil {
		log.Printf("⚠️ [MySQL] Failed to open connection: %v. Using MemoryStore fallback.", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ [MySQL] Connection to %s:%s/%s failed: %v. Using MemoryStore fallback.", host, port, dbName, err)
		return nil, err
	}

	DB = db

	queries := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price VARCHAR(100) NOT NULL,
			stock INT NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'Active',
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,

		`CREATE TABLE IF NOT EXISTS customers (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'Active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,

		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(50) PRIMARY KEY,
			customer_name VARCHAR(255) NOT NULL,
			total_amount VARCHAR(100) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'Processing',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("⚠️ [MySQL] Migration warning: %v", err)
		}
	}

	seedIfEmpty(db, "mysql")

	ProductRepo = booster.NewSQLStore(db, "products", "id").SetDriver("mysql")
	CustomerRepo = booster.NewSQLStore(db, "customers", "id").SetDriver("mysql")
	OrderRepo = booster.NewSQLStore(db, "orders", "id").SetDriver("mysql")

	log.Printf("🐬 [MySQL] Connected successfully to %s:%s/%s", host, port, dbName)
	log.Printf("   ├─ Tables: `products`, `customers`, `orders` verified")
	log.Printf("   └─ Active DataProvider: SQLStore (Live MySQL Persistence)")

	return db, nil
}

func initSQLite() (*sql.DB, error) {
	dbPath := getEnv("DB_DATABASE", "data/tgo_booster.db")
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("⚠️ [SQLite] Failed to open database: %v. Using MemoryStore fallback.", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ [SQLite] Ping failed: %v. Using MemoryStore fallback.", err)
		return nil, err
	}

	DB = db

	queries := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price TEXT NOT NULL,
			stock INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'Active',
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS customers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'Active',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS orders (
			id TEXT PRIMARY KEY,
			customer_name TEXT NOT NULL,
			total_amount TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'Processing',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("⚠️ [SQLite] Migration warning: %v", err)
		}
	}

	seedIfEmpty(db, "sqlite")

	ProductRepo = booster.NewSQLStore(db, "products", "id").SetDriver("sqlite")
	CustomerRepo = booster.NewSQLStore(db, "customers", "id").SetDriver("sqlite")
	OrderRepo = booster.NewSQLStore(db, "orders", "id").SetDriver("sqlite")

	log.Printf("💾 [SQLite] Connected successfully to %s", dbPath)
	log.Printf("   ├─ Tables: `products`, `customers`, `orders` verified")
	log.Printf("   └─ Active DataProvider: SQLStore (Live SQLite Persistence)")

	return db, nil
}

func initPostgres() (*sql.DB, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USERNAME", "postgres")
	pass := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_DATABASE", "tgo_booster")
	sslMode := getEnv("DB_SSLMODE", "disable")

	// Ensure database exists
	serverDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=%s", host, port, user, pass, sslMode)
	if sdb, err := sql.Open("postgres", serverDSN); err == nil {
		var exists bool
		_ = sdb.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
		if !exists {
			_, _ = sdb.Exec(fmt.Sprintf("CREATE DATABASE \"%s\"", dbName))
		}
		_ = sdb.Close()
	}

	appDSN := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, dbName, sslMode)
	db, err := sql.Open("postgres", appDSN)
	if err != nil {
		log.Printf("⚠️ [PostgreSQL] Failed to open connection: %v. Using MemoryStore fallback.", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ [PostgreSQL] Connection to %s:%s/%s failed: %v. Using MemoryStore fallback.", host, port, dbName, err)
		return nil, err
	}

	DB = db

	queries := []string{
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			price VARCHAR(100) NOT NULL,
			stock INT NOT NULL DEFAULT 0,
			status VARCHAR(50) NOT NULL DEFAULT 'Active',
			description TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS customers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'Active',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS orders (
			id VARCHAR(50) PRIMARY KEY,
			customer_name VARCHAR(255) NOT NULL,
			total_amount VARCHAR(100) NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'Processing',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Printf("⚠️ [PostgreSQL] Migration warning: %v", err)
		}
	}

	seedIfEmpty(db, "postgres")

	ProductRepo = booster.NewSQLStore(db, "products", "id").SetDriver("postgres")
	CustomerRepo = booster.NewSQLStore(db, "customers", "id").SetDriver("postgres")
	OrderRepo = booster.NewSQLStore(db, "orders", "id").SetDriver("postgres")

	log.Printf("🐘 [PostgreSQL] Connected successfully to %s:%s/%s", host, port, dbName)
	log.Printf("   ├─ Tables: `products`, `customers`, `orders` verified")
	log.Printf("   └─ Active DataProvider: SQLStore (Live PostgreSQL Persistence)")

	return db, nil
}

func seedIfEmpty(db *sql.DB, driver string) {
	// Seed Products
	var prodCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&prodCount)
	if prodCount == 0 {
		ph := "?, ?, ?, ?, ?"
		if driver == "postgres" || driver == "pgsql" {
			ph = "$1, $2, $3, $4, $5"
		}
		for _, p := range InitialProducts {
			_, _ = db.Exec(
				fmt.Sprintf("INSERT INTO products (name, price, stock, status, description) VALUES (%s)", ph),
				p["name"], p["price"], p["stock"], p["status"], p["description"],
			)
		}
	}

	// Seed Customers
	var custCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM customers").Scan(&custCount)
	if custCount == 0 {
		ph := "?, ?, ?"
		if driver == "postgres" || driver == "pgsql" {
			ph = "$1, $2, $3"
		}
		for _, c := range InitialCustomers {
			_, _ = db.Exec(
				fmt.Sprintf("INSERT INTO customers (name, email, status) VALUES (%s)", ph),
				c["name"], c["email"], c["status"],
			)
		}
	}

	// Seed Orders
	var ordCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&ordCount)
	if ordCount == 0 {
		ph := "?, ?, ?, ?"
		if driver == "postgres" || driver == "pgsql" {
			ph = "$1, $2, $3, $4"
		}
		for _, o := range InitialOrders {
			_, _ = db.Exec(
				fmt.Sprintf("INSERT INTO orders (id, customer_name, total_amount, status) VALUES (%s)", ph),
				o["id"], o["customer_name"], o["total_amount"], o["status"],
			)
		}
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
