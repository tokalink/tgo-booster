package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const version = "1.0.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printHelp()
		return
	}

	command := strings.ToLower(args[0])
	switch command {
	case "new", "create", "init":
		handleNewProject(args[1:])
	case "make:controller", "controller":
		handleMakeController(args[1:])
	case "version", "-v", "--version":
		fmt.Printf("⚡ TGo Booster CLI v%s\n", version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Printf("❌ Unknown command '%s'\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	banner := `⚡ TGo Booster CLI — The Ultra-Fast Go Admin & Project Scaffolder

Usage:
  booster <command> [arguments] [flags]

Commands:
  new <project-name>           Scaffold a complete, production-ready TGo Booster project
  make:controller <Name>       Generate a new declarative CRUD controller
  version                      Display CLI version
  help                         Display this help message

Options for 'new':
  --port <port>                Specify web port (default: auto-detected free port 8080/8082)
  --dir <path>                 Custom destination folder (default: ../<project-name>)
  --title <title>              Custom project display title
  --db <sqlite|mysql|memory>   Default database driver (default: sqlite)

Examples:
  booster new my-portal
  booster new erp-system --port 8085 --db sqlite
  booster make:controller Products
`
	fmt.Println(banner)
}

func handleNewProject(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Error: Project name is required.")
		fmt.Println("Usage: booster new <project-name> [--port 8080] [--dir ./path]")
		os.Exit(1)
	}

	projectName := args[0]
	// Parse optional flags
	var customPort string
	var customDir string
	var customTitle string
	defaultDB := "sqlite"

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 < len(args) {
				customPort = args[i+1]
				i++
			}
		case "--dir":
			if i+1 < len(args) {
				customDir = args[i+1]
				i++
			}
		case "--title":
			if i+1 < len(args) {
				customTitle = args[i+1]
				i++
			}
		case "--db":
			if i+1 < len(args) {
				defaultDB = strings.ToLower(args[i+1])
				i++
			}
		}
	}

	// Validate project name
	validName := regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
	if !validName.MatchString(projectName) {
		fmt.Println("❌ Error: Project name must only contain alphanumeric characters, dashes, and underscores.")
		os.Exit(1)
	}

	// Determine destination directory
	var destDir string
	if customDir != "" {
		destDir = customDir
	} else {
		// If current working directory is "tgo-booster" (or contains booster.go), create in sibling ".."
		// If running from parent folder (e.g. d:\Projects\tgo), create in "./<projectName>"
		if _, err := os.Stat("booster.go"); err == nil {
			destDir = filepath.Join("..", projectName)
		} else {
			destDir = filepath.Join(".", projectName)
		}
	}

	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		fmt.Printf("❌ Failed to resolve destination directory: %v\n", err)
		os.Exit(1)
	}

	if _, err := os.Stat(absDestDir); err == nil {
		entries, _ := os.ReadDir(absDestDir)
		if len(entries) > 0 {
			fmt.Printf("❌ Error: Target directory '%s' already exists and is not empty.\n", absDestDir)
			os.Exit(1)
		}
	}

	// Determine port
	port := customPort
	if port == "" {
		port = findAvailablePort(8080, 8082, 8083, 8085, 9000)
	}

	// Determine Title
	title := customTitle
	if title == "" {
		words := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(projectName, "-", " "), "_", " "))
		for i, w := range words {
			if len(w) > 0 {
				words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
			}
		}
		title = strings.Join(words, " ") + " • Admin Studio"
	}

	fmt.Printf("\n🚀 Scaffolding new TGo Booster project '%s'...\n", projectName)
	fmt.Printf("   ├─ Destination: %s\n", absDestDir)
	fmt.Printf("   ├─ Port:        %s\n", port)
	fmt.Printf("   ├─ Title:       %s\n", title)
	fmt.Printf("   └─ Database:    %s\n\n", defaultDB)

	// Check if local sibling modules exist
	localTGoCore := findSiblingDir(absDestDir, "tgo-core")
	localBooster := findSiblingDir(absDestDir, "tgo-booster")

	// 1. Create directories
	dirs := []string{
		absDestDir,
		filepath.Join(absDestDir, "app"),
		filepath.Join(absDestDir, "app", "controllers"),
		filepath.Join(absDestDir, "app", "models"),
		filepath.Join(absDestDir, "data"),
		filepath.Join(absDestDir, "tmp"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Printf("❌ Failed to create directory %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	// 2. Generate files
	safeName := strings.ReplaceAll(strings.ToLower(projectName), "-", "_")

	files := map[string]string{
		"go.mod":                                 renderGoMod(projectName, localTGoCore, localBooster),
		".env":                                   renderEnv(title, port, defaultDB, safeName),
		".env.example":                           renderEnv(title, port, defaultDB, safeName),
		".air.toml":                              airTomlContent,
		".gitignore":                             gitignoreContent,
		"main.go":                                renderMainGo(projectName, title, port),
		"app/models/db.go":                       renderDBGo(safeName),
		"app/controllers/admin_items_controller.go": renderItemsController(projectName),
		"data/.gitkeep":                          "",
		"README.md":                              renderReadme(projectName, title, port),
	}

	for relPath, content := range files {
		fullPath := filepath.Join(absDestDir, relPath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			fmt.Printf("❌ Failed to create file %s: %v\n", relPath, err)
			os.Exit(1)
		}
		fmt.Printf("   ✅ Created %s\n", relPath)
	}

	// 3. Run go mod tidy
	fmt.Printf("\n📦 Running 'go mod tidy' in %s...\n", projectName)
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = absDestDir
	cmdTidy.Stdout = os.Stdout
	cmdTidy.Stderr = os.Stderr
	if err := cmdTidy.Run(); err != nil {
		fmt.Printf("⚠️ Warning: 'go mod tidy' returned: %v\n", err)
	} else {
		fmt.Println("   ✅ Dependencies synchronized successfully.")
	}

	// 4. Test compilation check
	fmt.Printf("\n🧪 Validating project build...\n")
	testExe := filepath.Join(absDestDir, "tmp", "build_check.exe")
	cmdBuild := exec.Command("go", "build", "-o", testExe, ".")
	cmdBuild.Dir = absDestDir
	buildOutput, err := cmdBuild.CombinedOutput()
	if err != nil {
		fmt.Printf("⚠️ Build notice:\n%s\n", string(buildOutput))
	} else {
		_ = os.Remove(testExe)
		fmt.Println("   ✅ Zero-error compilation verified!")
	}

	// 5. Success summary
	fmt.Println("\n==================================================================")
	fmt.Printf("🎉 PROJECT '%s' CREATED SUCCESSFULLY!\n", projectName)
	fmt.Println("==================================================================")
	fmt.Printf("To run your new project:\n\n")
	fmt.Printf("  cd %s\n", destDir)
	fmt.Printf("  go run main.go\n\n")
	fmt.Println("Or run with live hot-reload:")
	fmt.Println("  air")
	fmt.Println("\nAccess Admin Studio:")
	fmt.Printf("  URL:      http://localhost:%s/admin\n", port)
	fmt.Println("  Email:    admin@tgo.io")
	fmt.Println("  Password: admin123")
	fmt.Println("==================================================================\n")
}

func handleMakeController(args []string) {
	if len(args) == 0 {
		fmt.Println("❌ Error: Controller name is required.")
		fmt.Println("Usage: booster make:controller <Name>")
		fmt.Println("Example: booster make:controller Products")
		os.Exit(1)
	}

	name := args[0]
	// Clean name
	cleanName := strings.TrimSuffix(name, "Controller")
	cleanName = strings.TrimSuffix(cleanName, "controller")
	cleanName = strings.ToUpper(cleanName[:1]) + cleanName[1:]

	snakeName := toSnakeCase(cleanName)
	fileName := fmt.Sprintf("admin_%s_controller.go", snakeName)
	targetPath := filepath.Join("app", "controllers", fileName)

	// Check if file exists
	if _, err := os.Stat(targetPath); err == nil {
		fmt.Printf("❌ Error: Controller file '%s' already exists.\n", targetPath)
		os.Exit(1)
	}

	modName := detectCurrentModuleName()
	modelsImport := "models"
	if modName != "" {
		modelsImport = modName + "/app/models"
	}

	content := fmt.Sprintf(`package controllers

import (
	booster "github.com/tokalink/tgo-booster"
	"%s"
)

// NewAdmin%sController builds the declarative CRUD controller for %s
func NewAdmin%sController() *booster.Controller {
	ctrl := &booster.Controller{
		Title: "%s",
		Table: "%s",
		Icon:  `+"`<svg width=\"18\" height=\"18\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\"/><path d=\"M3 9h18\"/><path d=\"M9 21V9\"/></svg>`"+`,
	}

	if models.DB != nil {
		ctrl.DataProvider = booster.NewSQLStore(models.DB, "%s", "id")
	} else {
		ctrl.DataProvider = booster.NewMemoryStore()
	}

	// 1. Data Grid Columns
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Title", "title", booster.ColText, true, true).
		AddCol("Category", "category", booster.ColText, true, true).
		AddCol("Status", "status", booster.ColBadge, false, true).
		AddCol("Created At", "created_at", booster.ColDateTime, false, true)

	// 2. Form Input Fields
	ctrl.
		AddForm("Title", "title", booster.InputText, true, "Enter title...").
		AddSelect("Category", "category", true, "Select category...",
			booster.Option{Value: "General", Label: "General"},
			booster.Option{Value: "Featured", Label: "Featured"},
		).
		AddSelect("Status", "status", true, "Select status...",
			booster.Option{Value: "Active", Label: "Active"},
			booster.Option{Value: "Draft", Label: "Draft"},
			booster.Option{Value: "Archived", Label: "Archived"},
		).
		AddForm("Description", "description", booster.InputWYSIWYG, false, "Detailed content...")

	return ctrl
}
`, modelsImport, cleanName, cleanName, cleanName, cleanName, snakeName, snakeName)

	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		fmt.Printf("❌ Failed to create controller: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Controller created at %s\n", targetPath)
	fmt.Printf("   Remember to register it in main.go: admin.Register(controllers.NewAdmin%sController())\n", cleanName)
}

func detectCurrentModuleName() string {
	data, err := os.ReadFile("go.mod")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "module ") {
				return strings.TrimSpace(strings.TrimPrefix(l, "module "))
			}
		}
	}
	return ""
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, []rune(strings.ToLower(string(r)))...)
	}
	return string(result)
}

func findAvailablePort(candidates ...int) string {
	for _, p := range candidates {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return strconv.Itoa(p)
		}
	}
	return "8085"
}

func findSiblingDir(destDir, name string) string {
	candidates := []string{
		filepath.Join(filepath.Dir(destDir), name),
		filepath.Join(filepath.Dir(destDir), "tgo", name),
		filepath.Join(destDir, "..", name),
		filepath.Join(destDir, "..", "tgo", name),
		filepath.Join(".", name),
	}
	for _, target := range candidates {
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			rel, err := filepath.Rel(destDir, target)
			if err == nil {
				return filepath.ToSlash(rel)
			}
		}
	}
	return ""
}

func renderGoMod(modName, localTGo, localBooster string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("module %s\n\ngo 1.25.3\n\nrequire (\n", modName))
	sb.WriteString("\tgithub.com/go-sql-driver/mysql v1.10.1\n")
	sb.WriteString("\tgithub.com/joho/godotenv v1.5.1\n")
	sb.WriteString("\tgithub.com/mattn/go-sqlite3 v1.14.52\n")
	sb.WriteString("\tgithub.com/tokalink/tgo v0.1.3\n")
	sb.WriteString("\tgithub.com/tokalink/tgo-booster v0.0.0\n")
	sb.WriteString(")\n\n")

	if localTGo != "" {
		sb.WriteString(fmt.Sprintf("replace github.com/tokalink/tgo => %s\n\n", localTGo))
	}
	if localBooster != "" {
		sb.WriteString(fmt.Sprintf("replace github.com/tokalink/tgo-booster => %s\n", localBooster))
	}

	return sb.String()
}

func renderEnv(title, port, dbDriver, safeName string) string {
	return fmt.Sprintf(`APP_NAME="%s"
APP_PORT=%s
APP_ENV=development

# Database Configuration (sqlite | mysql | memory)
DB_DRIVER=%s
DB_DATABASE=data/%s.db

# MySQL Settings (Optional: aktifkan jika DB_DRIVER=mysql)
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USERNAME=root
DB_PASSWORD=

# AI Custom Page Generator Studio (Optional)
OPENAI_HOST=https://ai.sumopod.com
OPENAI_APIKEY=
OPENAI_DEFAULT_MODEL=deepseek-v4-flash-0731:netra
`, title, port, dbDriver, safeName)
}

func renderMainGo(modName, title, port string) string {
	return fmt.Sprintf(`package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/tokalink/tgo/pkg/app"
	booster "github.com/tokalink/tgo-booster"
	"%s/app/controllers"
	"%s/app/models"
)

func main() {
	_ = godotenv.Load(".env")

	// 1. Inisialisasi Database (SQLite otomatis / MySQL dari .env)
	db, err := models.InitDatabase()
	if err != nil {
		log.Printf("⚠️ Database notice: %%v", err)
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "%s"
	}

	// 2. Inisialisasi HTTP Kernel TGo
	application := app.New().SetAddr(":" + port)

	// 3. Inisialisasi TGo Booster Admin Engine
	admin := booster.NewEngine("%s")
	if db != nil {
		admin.SetDB(db)
	}

	// 4. Daftarkan Modul CRUD Deklaratif
	admin.Register(controllers.NewAdminItemsController())

	// 5. Eksekusi Auto-Migrate Skema Database (Otomatis Buat Tabel)
	if err := admin.AutoMigrateAll(); err != nil {
		log.Printf("⚠️ AutoMigrate notice: %%v", err)
	}

	// 6. Mount Admin Studio Console ke /admin
	admin.Mount(application.Server(), "/admin")

	// 7. Route Root (/): Melayani Halaman CMS / Landing Page dari Pages Studio
	application.Server().Register("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			if admin.ServePublicPage(w, r) {
				return
			}
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}))

	log.Println("==================================================================")
	log.Printf("⚡ Server running on http://localhost:%%s\n", port)
	log.Printf("   ├─ Admin Studio:  http://localhost:%%s/admin\n", port)
	log.Printf("   ├─ Login Page:    http://localhost:%%s/admin/login\n", port)
	log.Println("   ├─ Default Email: admin@tgo.io")
	log.Println("   └─ Password:      admin123")
	log.Println("==================================================================")

	if err := application.Run(); err != nil {
		log.Fatalf("Server shutdown: %%v", err)
	}
}
`, modName, modName, port, title)
}

func renderDBGo(safeName string) string {
	return fmt.Sprintf(`package models

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// DB holds the active database connection pool
var DB *sql.DB

// InitDatabase initializes SQLite or MySQL based on .env configuration
func InitDatabase() (*sql.DB, error) {
	_ = godotenv.Load(".env")

	driver := strings.ToLower(getEnv("DB_DRIVER", "sqlite"))

	switch driver {
	case "sqlite", "sqlite3":
		return initSQLite()
	case "mysql":
		return initMySQL()
	default:
		log.Printf("ℹ️ [Database] In-memory mode active (driver: %%s)", driver)
		return nil, nil
	}
}

func initSQLite() (*sql.DB, error) {
	dbFile := getEnv("DB_DATABASE", "data/%s.db")
	_ = os.MkdirAll(filepath.Dir(dbFile), 0755)

	db, err := sql.Open("sqlite3", dbFile+"?_journal=WAL&_busy_timeout=5000")
	if err != nil {
		log.Printf("⚠️ [SQLite] Failed to connect: %%v", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ [SQLite] Ping failed: %%v", err)
		return nil, err
	}

	DB = db
	log.Printf("💾 [SQLite] Connected successfully to %%s", dbFile)
	return db, nil
}

func initMySQL() (*sql.DB, error) {
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "3306")
	user := getEnv("DB_USERNAME", "root")
	pass := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_DATABASE", "%s_db")

	// Ensure database exists
	serverDSN := fmt.Sprintf("%%s:%%s@tcp(%%s:%%s)/?parseTime=true", user, pass, host, port)
	serverDB, err := sql.Open("mysql", serverDSN)
	if err == nil {
		_, _ = serverDB.Exec(fmt.Sprintf(
			"CREATE DATABASE IF NOT EXISTS `+"`%s`"+` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
			dbName,
		))
		_ = serverDB.Close()
	}

	appDSN := fmt.Sprintf("%%s:%%s@tcp(%%s:%%s)/%%s?parseTime=true", user, pass, host, port, dbName)
	db, err := sql.Open("mysql", appDSN)
	if err != nil {
		log.Printf("⚠️ [MySQL] Open failed: %%v. Fallback to SQLite.", err)
		return initSQLite()
	}

	if err := db.Ping(); err != nil {
		log.Printf("⚠️ [MySQL] Ping failed: %%v. Fallback to SQLite.", err)
		return initSQLite()
	}

	DB = db
	log.Printf("🐬 [MySQL] Connected successfully to %%s on %%s:%%s", dbName, host, port)
	return db, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
`, safeName, safeName)
}

func renderItemsController(modName string) string {
	return fmt.Sprintf(`package controllers

import (
	booster "github.com/tokalink/tgo-booster"
	"%s/app/models"
)

// NewAdminItemsController builds the starter CRUD controller for Items
func NewAdminItemsController() *booster.Controller {
	ctrl := &booster.Controller{
		Title: "Items & Inventory",
		Table: "items",
		Icon:  `+"`<svg width=\"18\" height=\"18\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><rect width=\"18\" height=\"18\" x=\"3\" y=\"3\" rx=\"2\"/><path d=\"M3 9h18\"/><path d=\"M9 21V9\"/></svg>`"+`,
	}

	if models.DB != nil {
		ctrl.DataProvider = booster.NewSQLStore(models.DB, "items", "id")
	} else {
		ctrl.DataProvider = booster.NewMemoryStore()
	}

	// 1. Data Grid Columns
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Item Name", "name", booster.ColText, true, true).
		AddCol("Category", "category", booster.ColText, true, true).
		AddCol("SKU Code", "sku", booster.ColText, true, true).
		AddCol("Price", "price", booster.ColMoney, false, true).
		AddCol("Status", "status", booster.ColBadge, false, true)

	// 2. Form Input Fields
	ctrl.
		AddForm("Item Name", "name", booster.InputText, true, "e.g. Ergonomic Office Chair").
		AddSelect("Category", "category", true, "Select category...",
			booster.Option{Value: "Electronics", Label: "Electronics"},
			booster.Option{Value: "Furniture", Label: "Furniture"},
			booster.Option{Value: "Software", Label: "Software License"},
			booster.Option{Value: "Services", Label: "Professional Services"},
		).
		AddForm("SKU Code", "sku", booster.InputText, true, "e.g. FUR-CHR-001").
		AddForm("Price (IDR)", "price", booster.InputMoney, true, "1500000").
		AddSelect("Status", "status", true, "Select status...",
			booster.Option{Value: "In Stock", Label: "In Stock"},
			booster.Option{Value: "Low Stock", Label: "Low Stock"},
			booster.Option{Value: "Out of Stock", Label: "Out of Stock"},
		).
		AddForm("Description", "description", booster.InputWYSIWYG, false, "Detailed specifications and notes...")

	// 3. Top Actions (Index Grid Toolbar)
	ctrl.AddTopAction(booster.TopAction{
		Label:   "Export CSV",
		Icon:    `+"`<svg width=\"14\" height=\"14\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2\"><path d=\"M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4\"/><polyline points=\"7 10 12 15 17 10\"/><line x1=\"12\" y1=\"15\" x2=\"12\" y2=\"3\"/></svg>`"+`,
		Class:   "btn-secondary",
		OnClick: "window.location.href='/admin/items/data'",
	})

	return ctrl
}
`, modName)
}

func renderReadme(projectName, title, port string) string {
	return fmt.Sprintf(`# %s

> Generated with **TGo Booster** — The Ultra-Fast Go Admin & Project Framework.

## 🚀 Quick Start

### 1. Jalankan Aplikasi Langsung
`+"```bash"+`
go run main.go
`+"```"+`

### 2. Jalankan dengan Live Hot-Reload (Air)
`+"```bash"+`
air
`+"```"+`

Buka di browser:
- **Admin Studio**: [http://localhost:%s/admin](http://localhost:%s/admin)
- **Login Email**: `+"`admin@tgo.io`"+`
- **Password**: `+"`admin123`"+`

## 🗄️ Database
- Secara default, aplikasi menggunakan database **SQLite** mandiri yang tersimpan di `+"`data/%s.db`"+`.
- Anda dapat mengubah ke **MySQL** cukup dengan mengedit berkas `+"`.env`"+`.
- Tabel skema akan dibuat otomatis (**Auto-Migrate**) saat server pertama kali dijalankan.
`, title, port, port, projectName)
}

const airTomlContent = `root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
args_bin = []
bin = "./tmp/main.exe"
cmd = "go build -o ./tmp/main.exe ."
delay = 500
exclude_dir = ["assets", "tmp", "vendor", "testdata", "data"]
exclude_file = []
exclude_regex = ["_test.go"]
exclude_unchanged = false
follow_symlink = false
full_bin = ""
include_dir = []
include_ext = ["go", "tpl", "tmpl", "html", "env"]
kill_delay = "0s"
log = "build-errors.log"
send_interrupt = false
stop_on_error = true

[color]
app = ""
build = "yellow"
main = "magenta"
runner = "green"
watcher = "cyan"

[log]
main_only = false
time = false

[misc]
clean_on_exit = false

[screen]
clear_on_rebuild = false
keep_scroll = true
`

const gitignoreContent = `tmp/
*.exe
*.log
data/*.db
data/*.db-wal
data/*.db-shm
.env
`
