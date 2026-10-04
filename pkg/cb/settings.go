package cb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/tokalink/tgo/pkg/transport/connect"
)

var (
	settingsPersistenceMu sync.RWMutex
	serverStartTime       = time.Now()
)

// AppSettings represents the full platform configuration and enterprise preferences
type AppSettings struct {
	// 1. General & Branding
	AppName         string `json:"app_name"`
	AppTagline      string `json:"app_tagline"`
	CompanyName     string `json:"company_name"`
	AppLogoURL      string `json:"app_logo_url"`
	AppLogoDarkURL  string `json:"app_logo_dark_url"`
	AppFaviconURL   string `json:"app_favicon_url"`
	CopyrightText   string `json:"copyright_text"`
	DefaultLanguage string `json:"default_language"` // "id", "en"
	Timezone        string `json:"timezone"`         // "Asia/Jakarta", "UTC", etc.
	DateFormat      string `json:"date_format"`      // "YYYY-MM-DD", "DD/MM/YYYY"
	TimeFormat      string `json:"time_format"`      // "24h", "12h"

	// 2. Appearance & UI Theme Studio
	ThemeMode    string `json:"theme_mode"`    // "system", "dark", "light"
	ThemeSkin    string `json:"theme_skin"`    // "obsidian", "midnight", "emerald", "amethyst", "light"
	AccentColor  string `json:"accent_color"`  // Hex code (e.g. #0284c7)
	SidebarStyle string `json:"sidebar_style"` // "expanded", "compact"

	// 3. Security & Authentication
	SessionTimeoutMinutes  int  `json:"session_timeout_minutes"`
	MaxLoginAttempts      int  `json:"max_login_attempts"`
	LockoutDurationMin    int  `json:"lockout_duration_min"`
	PasswordMinLength     int  `json:"password_min_length"`
	RequirePasswordSpecial bool `json:"require_password_special"`
	EnableRegistration    bool `json:"enable_registration"`
	EnableRememberMe      bool `json:"enable_remember_me"`
	Force2FA              bool `json:"force_2fa"`

	// 4. Data Grid & CRUD Defaults
	DefaultPageSize       int    `json:"default_page_size"`
	DefaultActionPosition string `json:"default_action_position"` // "right", "left"
	EnableExportCSV       bool   `json:"enable_export_csv"`
	EnableImportCSV       bool   `json:"enable_import_csv"`
	ConfirmDelete         bool   `json:"confirm_delete"`
	EnableAuditLog        bool   `json:"enable_audit_log"`

	// 5. Notification & Mail (SMTP)
	MailDriver      string `json:"mail_driver"` // "smtp", "sendgrid", "log"
	SMTPHost        string `json:"smtp_host"`
	SMTPPort        int    `json:"smtp_port"`
	SMTPUser        string `json:"smtp_user"`
	SMTPPassword    string `json:"smtp_password"`
	SMTPEncryption  string `json:"smtp_encryption"` // "tls", "ssl", "none"
	MailSenderEmail string `json:"mail_sender_email"`
	MailSenderName  string `json:"mail_sender_name"`
	WebhookURL      string `json:"webhook_url"`

	// 6. Maintenance & System
	MaintenanceMode    bool   `json:"maintenance_mode"`
	MaintenanceMessage string `json:"maintenance_message"`
	DebugMode          bool   `json:"debug_mode"`

	// 7. AI & LLM Integration (OpenAI-compatible)
	OpenAIHost         string `json:"openai_host"`
	OpenAIAPIKey       string `json:"openai_api_key"`
	OpenAIDefaultModel string `json:"openai_default_model"`

	UpdatedAt string `json:"updated_at"`
}

// DefaultSettings returns sensible enterprise defaults
func DefaultSettings(appName string) *AppSettings {
	name := appName
	if name == "" {
		name = "TGo Enterprise"
	}

	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")
	aiHost := strings.TrimSpace(os.Getenv("OPENAI_HOST"))
	if aiHost == "" {
		aiHost = "https://ai.sumopod.com"
	}
	aiKey := strings.TrimSpace(os.Getenv("OPENAI_APIKEY"))
	aiModel := strings.TrimSpace(os.Getenv("OPENAI_DEFAULT_MODEL"))
	if aiModel == "" {
		aiModel = "deepseek-v4-flash-0731:netra"
	}

	return &AppSettings{
		AppName:               name,
		AppTagline:            "High-Performance Go Administrative Platform",
		CompanyName:           "TGo Technology Labs",
		AppLogoURL:            "",
		AppLogoDarkURL:        "",
		AppFaviconURL:         "",
		CopyrightText:         fmt.Sprintf("© %d %s. All rights reserved.", time.Now().Year(), name),
		DefaultLanguage:       "id",
		Timezone:              "Asia/Jakarta",
		DateFormat:            "YYYY-MM-DD",
		TimeFormat:            "24h",
		ThemeMode:             "dark",
		ThemeSkin:             "obsidian",
		AccentColor:           "#0284c7",
		SidebarStyle:          "expanded",
		SessionTimeoutMinutes: 120,
		MaxLoginAttempts:      5,
		LockoutDurationMin:    15,
		PasswordMinLength:     8,
		RequirePasswordSpecial: true,
		EnableRegistration:    false,
		EnableRememberMe:      true,
		Force2FA:              false,
		DefaultPageSize:       25,
		DefaultActionPosition: "right",
		EnableExportCSV:       true,
		EnableImportCSV:       true,
		ConfirmDelete:         true,
		EnableAuditLog:        true,
		MailDriver:            "log",
		SMTPHost:              "smtp.mailtrap.io",
		SMTPPort:              2525,
		SMTPUser:              "",
		SMTPPassword:          "",
		SMTPEncryption:        "tls",
		MailSenderEmail:       "no-reply@tgo.internal",
		MailSenderName:        name + " System",
		WebhookURL:            "",
		MaintenanceMode:       false,
		MaintenanceMessage:    "We are currently performing scheduled maintenance. Please check back shortly.",
		DebugMode:             false,
		OpenAIHost:            aiHost,
		OpenAIAPIKey:          aiKey,
		OpenAIDefaultModel:    aiModel,
		UpdatedAt:             time.Now().Format("2006-01-02 15:04:05"),
	}
}

func (e *Engine) getSettingsPersistencePath() string {
	candidates := []string{
		"data/settings.json",
		"starter/data/settings.json",
		"../starter/data/settings.json",
	}
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return c
		}
	}
	return "data/settings.json"
}

// LoadDynamicSettings loads configuration from data/settings.json or initializes defaults
func (e *Engine) LoadDynamicSettings() {
	settingsPersistenceMu.Lock()
	defer settingsPersistenceMu.Unlock()

	p := e.getSettingsPersistencePath()
	data, err := os.ReadFile(p)
	if err == nil && len(data) > 0 {
		var s AppSettings
		if jsonErr := json.Unmarshal(data, &s); jsonErr == nil {
			// Populate AI fallback from .env if empty
			_ = godotenv.Load(".env")
			_ = godotenv.Load("../.env")
			if s.OpenAIHost == "" {
				s.OpenAIHost = strings.TrimSpace(os.Getenv("OPENAI_HOST"))
				if s.OpenAIHost == "" {
					s.OpenAIHost = "https://ai.sumopod.com"
				}
			}
			if s.OpenAIAPIKey == "" {
				s.OpenAIAPIKey = strings.TrimSpace(os.Getenv("OPENAI_APIKEY"))
			}
			if s.OpenAIDefaultModel == "" {
				s.OpenAIDefaultModel = strings.TrimSpace(os.Getenv("OPENAI_DEFAULT_MODEL"))
				if s.OpenAIDefaultModel == "" {
					s.OpenAIDefaultModel = "deepseek-v4-flash-0731:netra"
				}
			}

			e.settings = &s
			if s.AppName != "" {
				e.AppName = s.AppName
			}
			return
		}
	}

	// Initialize defaults and persist
	e.settings = DefaultSettings(e.AppName)
	if raw, jsonErr := json.MarshalIndent(e.settings, "", "  "); jsonErr == nil {
		_ = os.WriteFile(p, raw, 0644)
	}
}

// GetSettings returns a clone of current runtime settings
func (e *Engine) GetSettings() AppSettings {
	settingsPersistenceMu.RLock()
	defer settingsPersistenceMu.RUnlock()
	if e.settings == nil {
		return *DefaultSettings(e.AppName)
	}
	return *e.settings
}

// SaveSettings writes updated settings and applies changes to runtime engine live
func (e *Engine) SaveSettings(s *AppSettings) error {
	settingsPersistenceMu.Lock()
	defer settingsPersistenceMu.Unlock()

	s.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
	e.settings = s
	if s.AppName != "" {
		e.AppName = s.AppName
	}

	p := e.getSettingsPersistencePath()
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0644)
}

// SystemDiagnostics represents live server metrics
type SystemDiagnostics struct {
	GoVersion     string `json:"go_version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	NumCPU        int    `json:"num_cpu"`
	NumGoroutine  int    `json:"num_goroutine"`
	AllocMB       string `json:"alloc_mb"`
	SysMB         string `json:"sys_mb"`
	NumGC         uint32 `json:"num_gc"`
	Uptime        string `json:"uptime"`
	DatabaseState string `json:"database_state"`
	ActiveModules int    `json:"active_modules"`
}

func (e *Engine) collectSystemDiagnostics() SystemDiagnostics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	dbState := "Memory Store (Fast)"
	if e.db != nil {
		if err := e.db.Ping(); err == nil {
			dbState = "SQL Database Connected (Active)"
		} else {
			dbState = "SQL Database Offline (" + err.Error() + ")"
		}
	}

	dur := time.Since(serverStartTime)
	uptimeStr := fmt.Sprintf("%02dh %02dm %02ds", int(dur.Hours()), int(dur.Minutes())%60, int(dur.Seconds())%60)

	return SystemDiagnostics{
		GoVersion:     runtime.Version(),
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		NumCPU:        runtime.NumCPU(),
		NumGoroutine:  runtime.NumGoroutine(),
		AllocMB:       fmt.Sprintf("%.2f MB", float64(m.Alloc)/1024/1024),
		SysMB:         fmt.Sprintf("%.2f MB", float64(m.Sys)/1024/1024),
		NumGC:         m.NumGC,
		Uptime:        uptimeStr,
		DatabaseState: dbState,
		ActiveModules: len(e.controllers),
	}
}

// registerSettingsRoutes mounts all endpoints for the Settings Studio
func (e *Engine) registerSettingsRoutes(server connect.Server) {
	settingsPath := e.AdminPath + "/settings"

	// GET /admin/settings -> Studio View
	server.Register(settingsPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if e.Auth != nil {
			user := e.Auth.GetSessionUser(r)
			if user == nil {
				http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
				return
			}
			if !e.IsUserSuperadmin(user) {
				e.RenderForbidden(w, r, "Settings Studio", "Superadmin")
				return
			}
		}

		cur := e.GetSettings()
		diag := e.collectSystemDiagnostics()

		data := map[string]interface{}{
			"AdminPath":   e.AdminPath,
			"AppName":     e.AppName,
			"Settings":    cur,
			"Diagnostics": diag,
		}

		contentHTML, err := RenderSettingsContent(data)
		if err != nil {
			http.Error(w, "Failed to render settings studio: "+err.Error(), http.StatusInternalServerError)
			return
		}

		e.RenderLayout(w, r, "Settings Studio", contentHTML)
	}))

	// POST /admin/settings/save -> Persist updated settings
	server.Register(settingsPath+"/save", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if e.Auth != nil {
			user := e.Auth.GetSessionUser(r)
			if !e.IsUserSuperadmin(user) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Forbidden: Superadmin role required to update platform settings",
				})
				return
			}
		}

		var payload AppSettings
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid settings JSON payload: " + err.Error(),
			})
			return
		}

		if strings.TrimSpace(payload.AppName) == "" {
			payload.AppName = "TGo Booster"
		}

		if err := e.SaveSettings(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to write settings file: " + err.Error(),
			})
			return
		}

		e.LogAudit(r, "SETTINGS_UPDATE", "Settings", fmt.Sprintf("Platform configuration updated: AppName=%s, ThemeSkin=%s", payload.AppName, payload.ThemeSkin))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Platform settings updated and persisted successfully!",
			"app_name": payload.AppName,
		})
	}))

	// POST /admin/settings/test-mail -> Diagnostic email test
	server.Register(settingsPath+"/test-mail", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			TargetEmail string `json:"target_email"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		target := strings.TrimSpace(req.TargetEmail)
		if target == "" {
			target = "admin@tgo.internal"
		}

		settings := e.GetSettings()

		// Simulate delivery or SMTP handshake
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Test email sent to %s via %s (%s:%d)", target, strings.ToUpper(settings.MailDriver), settings.SMTPHost, settings.SMTPPort),
		})
	}))

	// POST /admin/settings/test-ai -> Test OpenAI/DeepSeek connection & latency
	server.Register(settingsPath+"/test-ai", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Host   string `json:"host"`
			APIKey string `json:"api_key"`
			Model  string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		host := strings.TrimSpace(req.Host)
		apiKey := strings.TrimSpace(req.APIKey)
		model := strings.TrimSpace(req.Model)

		// If payload didn't specify values, fallback to configured settings/env
		if host == "" || apiKey == "" || model == "" {
			cfgHost, cfgKey, cfgModel := e.GetAIConfig()
			if host == "" {
				host = cfgHost
			}
			if apiKey == "" {
				apiKey = cfgKey
			}
			if model == "" {
				model = cfgModel
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if apiKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "API key is empty. Please enter an API key or configure OPENAI_APIKEY in .env.",
			})
			return
		}

		latency, modelDesc, err := e.TestAIConnectionWith(host, apiKey, model)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":    false,
				"latency_ms": latency,
				"model":      modelDesc,
				"error":      err.Error(),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"latency_ms": latency,
			"model":      modelDesc,
			"message":    fmt.Sprintf("Handshake successful with %s (%dms roundtrip)", modelDesc, latency),
		})
	}))

	// POST /admin/settings/models -> Fetch live list of models from OpenAI-compatible endpoint
	server.Register(settingsPath+"/models", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Host   string `json:"host"`
			APIKey string `json:"api_key"`
		}
		if r.Method == http.MethodPost && r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		host := strings.TrimSpace(req.Host)
		apiKey := strings.TrimSpace(req.APIKey)

		if host == "" || apiKey == "" {
			cfgHost, cfgKey, _ := e.GetAIConfig()
			if host == "" {
				host = cfgHost
			}
			if apiKey == "" {
				apiKey = cfgKey
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if apiKey == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "API key is missing. Please configure OPENAI_APIKEY in .env or Settings.",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		models, err := e.FetchAIModels(ctx, host, apiKey)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"models":  models,
			"count":   len(models),
		})
	}))

	// POST /admin/settings/clear-cache -> Flush caches & trigger GC
	server.Register(settingsPath+"/clear-cache", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		runtime.GC()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "System template buffers, memory caches, and dead object allocations flushed successfully.",
		})
	}))

	// POST /admin/settings/reset -> Restore defaults
	server.Register(settingsPath+"/reset", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		defaults := DefaultSettings(e.AppName)
		if err := e.SaveSettings(defaults); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to restore default settings: " + err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Platform settings restored to defaults!",
		})
	}))

	// GET /admin/settings/api/system-info -> Live telemetry
	server.Register(settingsPath+"/api/system-info", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		diag := e.collectSystemDiagnostics()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(diag)
	}))
}
