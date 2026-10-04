package cb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AuditLogEntry represents a security or operational event
type AuditLogEntry struct {
	ID          string `json:"id"`
	IP          string `json:"ip"`
	User        string `json:"user"`
	Action      string `json:"action"`
	Module      string `json:"module"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

// AuditLogger manages the persistent security audit trail
type AuditLogger struct {
	engine   *Engine
	mu       sync.RWMutex
	filePath string
	logs     []AuditLogEntry
	maxLogs  int
}

// NewAuditLogger initializes the persistent audit logger
func NewAuditLogger(engine *Engine) *AuditLogger {
	baseDir := "data"
	_ = os.MkdirAll(baseDir, 0755)
	filePath := filepath.Join(baseDir, "logs.json")

	al := &AuditLogger{
		engine:   engine,
		filePath: filePath,
		logs:     make([]AuditLogEntry, 0),
		maxLogs:  1000,
	}
	al.loadLogs()
	return al
}

func (al *AuditLogger) loadLogs() {
	al.mu.Lock()
	defer al.mu.Unlock()

	data, err := os.ReadFile(al.filePath)
	if err == nil && len(data) > 0 {
		var list []AuditLogEntry
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			al.logs = list
			return
		}
	}

	// Seed initial baseline records
	now := time.Now().Format("2006-01-02 15:04:05")
	initial := []AuditLogEntry{
		{
			ID:          "1",
			IP:          "127.0.0.1",
			User:        "System Kernel",
			Action:      "SYSTEM_BOOT",
			Module:      "Core",
			Description: "TGo Booster Enterprise Engine initialized successfully",
			CreatedAt:   now,
		},
		{
			ID:          "2",
			IP:          "127.0.0.1",
			User:        "Super Administrator",
			Action:      "LOGIN",
			Module:      "Auth",
			Description: "Session authenticated via local administration console",
			CreatedAt:   now,
		},
	}
	al.logs = initial
	_ = al.saveUnlocked()
}

func (al *AuditLogger) saveUnlocked() error {
	bytes, err := json.MarshalIndent(al.logs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(al.filePath, bytes, 0644)
}

// Log records a new audit trail entry
func (al *AuditLogger) Log(r *http.Request, action, module, description string) {
	if al.engine != nil {
		st := al.engine.GetSettings()
		if !st.EnableAuditLog {
			return
		}
	}

	clientIP := "127.0.0.1"
	userName := "System"

	if r != nil {
		clientIP = r.RemoteAddr
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			clientIP = strings.Split(xff, ",")[0]
		} else if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
			clientIP = xrip
		}

		if al.engine != nil && al.engine.Auth != nil {
			if u := al.engine.Auth.GetSessionUser(r); u != nil {
				userName = u.Name
				if userName == "" {
					userName = u.Email
				}
			}
		}
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	nextID := 1
	if len(al.logs) > 0 {
		if lastID, err := strconv.Atoi(al.logs[0].ID); err == nil {
			nextID = lastID + 1
		} else {
			nextID = len(al.logs) + 1
		}
	}

	entry := AuditLogEntry{
		ID:          strconv.Itoa(nextID),
		IP:          clientIP,
		User:        userName,
		Action:      strings.ToUpper(strings.TrimSpace(action)),
		Module:      module,
		Description: description,
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// Prepend to show latest first
	al.logs = append([]AuditLogEntry{entry}, al.logs...)
	if len(al.logs) > al.maxLogs {
		al.logs = al.logs[:al.maxLogs]
	}

	_ = al.saveUnlocked()
}

// GetAllLogs returns all audit trail entries
func (al *AuditLogger) GetAllLogs() []AuditLogEntry {
	al.mu.RLock()
	defer al.mu.RUnlock()

	out := make([]AuditLogEntry, len(al.logs))
	copy(out, al.logs)
	return out
}

// -------------------------------------------------------------
// AuditLogDataProvider implements DataProvider for /admin/logs
// -------------------------------------------------------------

type AuditLogDataProvider struct {
	logger *AuditLogger
}

func (al *AuditLogger) NewDataProvider() DataProvider {
	return &AuditLogDataProvider{logger: al}
}

func (adp *AuditLogDataProvider) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	logs := adp.logger.GetAllLogs()
	out := make([]map[string]interface{}, len(logs))
	for i, l := range logs {
		out[i] = map[string]interface{}{
			"id":          l.ID,
			"ip":          l.IP,
			"user":        l.User,
			"action":      l.Action,
			"module":      l.Module,
			"description": l.Description,
			"created_at":  l.CreatedAt,
		}
	}
	return out, nil
}

func (adp *AuditLogDataProvider) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	adp.logger.mu.RLock()
	defer adp.logger.mu.RUnlock()

	for _, l := range adp.logger.logs {
		if l.ID == id {
			return map[string]interface{}{
				"id":          l.ID,
				"ip":          l.IP,
				"user":        l.User,
				"action":      l.Action,
				"module":      l.Module,
				"description": l.Description,
				"created_at":  l.CreatedAt,
			}, nil
		}
	}
	return nil, fmt.Errorf("audit log #%s not found", id)
}

func (adp *AuditLogDataProvider) Create(ctx *Context, data map[string]interface{}) error {
	action, _ := data["action"].(string)
	module, _ := data["module"].(string)
	desc, _ := data["description"].(string)
	var r *http.Request
	if ctx != nil {
		r = ctx.Request
	}
	adp.logger.Log(r, action, module, desc)
	return nil
}

func (adp *AuditLogDataProvider) Update(ctx *Context, id string, data map[string]interface{}) error {
	// Audit logs are immutable
	return fmt.Errorf("audit logs are immutable and cannot be modified")
}

func (adp *AuditLogDataProvider) Delete(ctx *Context, id string) error {
	// Audit logs are immutable
	return fmt.Errorf("audit logs cannot be deleted manually")
}
