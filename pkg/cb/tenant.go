package cb

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Tenant represents an isolated customer/organization account in the SaaS platform
type Tenant struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Domain       string `json:"domain,omitempty"`        // Custom Domain (e.g. "erp.client.com")
	Subdomain    string `json:"subdomain,omitempty"`     // Subdomain slug (e.g. "client1")
	Plan         string `json:"plan"`                    // "Free", "Starter", "Pro", "Enterprise"
	Status       string `json:"status"`                  // "Active", "Suspended", "Trial"
	OwnerEmail   string `json:"owner_email"`
	MaxUsers     int    `json:"max_users"`
	MaxStorageMB int    `json:"max_storage_mb"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// TenantManager handles thread-safe persistence and domain/subdomain resolution for SaaS tenants
type TenantManager struct {
	engine   *Engine
	mu       sync.RWMutex
	filePath string
	tenants  []*Tenant
}

// NewTenantManager initializes the manager with data/tenants.json persistence
func NewTenantManager(engine *Engine) *TenantManager {
	tm := &TenantManager{
		engine:   engine,
		filePath: filepath.Join("data", "tenants.json"),
		tenants:  make([]*Tenant, 0),
	}
	tm.load()
	return tm
}

func (tm *TenantManager) load() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := tm.engine.docLoad(DocTenants)
	if err != nil || len(data) == 0 {
		tm.seedDefaults()
		return
	}

	var list []*Tenant
	if err := json.Unmarshal(data, &list); err != nil {
		tm.seedDefaults()
		return
	}

	tm.tenants = list
	if len(tm.tenants) == 0 {
		tm.seedDefaults()
	}
}

func (tm *TenantManager) seedDefaults() {
	now := time.Now().Format("2006-01-02 15:04:05")
	appName := "Primary Organization"
	if tm.engine != nil && tm.engine.AppName != "" {
		appName = tm.engine.AppName
	}

	tm.tenants = []*Tenant{
		{
			ID:           "1",
			Name:         appName,
			Slug:         "default",
			Plan:         "Enterprise",
			Status:       "Active",
			OwnerEmail:   "admin@tgo.io",
			MaxUsers:     100,
			MaxStorageMB: 5120,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           "2",
			Name:         "Demo Enterprise",
			Slug:         "demo-enterprise",
			Subdomain:    "demo",
			Plan:         "Pro",
			Status:       "Active",
			OwnerEmail:   "manager@tgo.io",
			MaxUsers:     25,
			MaxStorageMB: 2048,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	tm.saveLocked()
}

func (tm *TenantManager) saveLocked() error {
	data, err := json.MarshalIndent(tm.tenants, "", "  ")
	if err != nil {
		return err
	}
	return tm.engine.docSave(DocTenants, data)
}

// GetAll returns a copy of all registered tenants
func (tm *TenantManager) GetAll() []*Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Tenant, len(tm.tenants))
	copy(result, tm.tenants)
	return result
}

// GetByID finds a tenant by its ID
func (tm *TenantManager) GetByID(id string) *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.tenants {
		if t.ID == id {
			clone := *t
			return &clone
		}
	}
	return nil
}

// GetBySlug finds a tenant by its slug (case-insensitive)
func (tm *TenantManager) GetBySlug(slug string) *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	target := strings.ToLower(strings.TrimSpace(slug))
	for _, t := range tm.tenants {
		if strings.ToLower(t.Slug) == target {
			clone := *t
			return &clone
		}
	}
	return nil
}

// GetByDomain finds a tenant by custom domain (case-insensitive)
func (tm *TenantManager) GetByDomain(domain string) *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	target := strings.ToLower(strings.TrimSpace(domain))
	if target == "" {
		return nil
	}

	for _, t := range tm.tenants {
		if t.Domain != "" && strings.ToLower(strings.TrimSpace(t.Domain)) == target {
			clone := *t
			return &clone
		}
	}
	return nil
}

// GetBySubdomain finds a tenant by subdomain (case-insensitive)
func (tm *TenantManager) GetBySubdomain(sub string) *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	target := strings.ToLower(strings.TrimSpace(sub))
	if target == "" || target == "www" {
		return nil
	}

	for _, t := range tm.tenants {
		if t.Subdomain != "" && strings.ToLower(strings.TrimSpace(t.Subdomain)) == target {
			clone := *t
			return &clone
		}
		if strings.ToLower(strings.TrimSpace(t.Slug)) == target {
			clone := *t
			return &clone
		}
	}
	return nil
}

// GetDefaultTenant returns the first active tenant or default
func (tm *TenantManager) GetDefaultTenant() *Tenant {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, t := range tm.tenants {
		if strings.ToLower(t.Slug) == "default" {
			clone := *t
			return &clone
		}
	}
	if len(tm.tenants) > 0 {
		clone := *tm.tenants[0]
		return &clone
	}
	return nil
}

// UserCanAccess checks if a user has access to a specific tenant
func (tm *TenantManager) UserCanAccess(user *User, t *Tenant) bool {
	if t == nil {
		return false
	}
	if user == nil {
		return true // Unauthenticated / public request
	}

	// Superadmins always have full access
	if tm.engine != nil && tm.engine.IsUserSuperadmin(user) {
		return true
	}

	for _, accessible := range user.Tenants {
		if accessible == "*" || strings.EqualFold(accessible, t.Slug) || accessible == t.ID {
			return true
		}
	}

	// Default fallback: allow access to "default" if user has no explicit tenant assignments
	if len(user.Tenants) == 0 && strings.EqualFold(t.Slug, "default") {
		return true
	}

	return false
}

// GetAvailableTenantsForUser returns the list of tenants accessible by a user
func (tm *TenantManager) GetAvailableTenantsForUser(user *User) []*Tenant {
	all := tm.GetAll()
	if user == nil || (tm.engine != nil && tm.engine.IsUserSuperadmin(user)) {
		return all
	}

	for _, accessible := range user.Tenants {
		if accessible == "*" {
			return all
		}
	}

	filtered := make([]*Tenant, 0)
	for _, t := range all {
		if tm.UserCanAccess(user, t) {
			filtered = append(filtered, t)
		}
	}

	if len(filtered) == 0 {
		def := tm.GetDefaultTenant()
		if def != nil {
			filtered = append(filtered, def)
		}
	}

	return filtered
}

// Save creates or updates a tenant
func (tm *TenantManager) Save(t *Tenant) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now().Format("2006-01-02 15:04:05")
	t.Slug = strings.ToLower(strings.TrimSpace(t.Slug))
	if t.Slug == "" {
		t.Slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(t.Name), " ", "-"))
	}
	t.Domain = strings.ToLower(strings.TrimSpace(t.Domain))
	t.Subdomain = strings.ToLower(strings.TrimSpace(t.Subdomain))

	if t.Plan == "" {
		t.Plan = "Free"
	}
	if t.Status == "" {
		t.Status = "Active"
	}

	found := false
	for i, existing := range tm.tenants {
		if existing.ID == t.ID || (t.ID == "" && strings.EqualFold(existing.Slug, t.Slug)) {
			t.ID = existing.ID
			t.CreatedAt = existing.CreatedAt
			t.UpdatedAt = now
			tm.tenants[i] = t
			found = true
			break
		}
	}

	if !found {
		maxID := 0
		for _, ex := range tm.tenants {
			if idInt, err := strconv.Atoi(ex.ID); err == nil && idInt > maxID {
				maxID = idInt
			}
		}
		t.ID = strconv.Itoa(maxID + 1)
		t.CreatedAt = now
		t.UpdatedAt = now
		tm.tenants = append(tm.tenants, t)
	}

	return tm.saveLocked()
}

// Delete removes a tenant by ID (refusing to delete the last or default tenant)
func (tm *TenantManager) Delete(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tenants) <= 1 {
		return errors.New("cannot delete the only remaining tenant")
	}

	idx := -1
	for i, t := range tm.tenants {
		if t.ID == id {
			if strings.EqualFold(t.Slug, "default") {
				return errors.New("cannot delete the primary default tenant")
			}
			idx = i
			break
		}
	}

	if idx == -1 {
		return errors.New("tenant not found")
	}

	tm.tenants = append(tm.tenants[:idx], tm.tenants[idx+1:]...)
	return tm.saveLocked()
}

// TenantDataProvider implements DataProvider for the /admin/tenants controller
type TenantDataProvider struct {
	manager *TenantManager
}

// NewTenantDataProvider wraps TenantManager as a standard Booster DataProvider
func (tm *TenantManager) NewDataProvider() *TenantDataProvider {
	return &TenantDataProvider{manager: tm}
}

func (p *TenantDataProvider) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	tenants := p.manager.GetAll()
	rows := make([]map[string]interface{}, len(tenants))
	for i, t := range tenants {
		rows[i] = map[string]interface{}{
			"id":             t.ID,
			"name":           t.Name,
			"slug":           t.Slug,
			"domain":         t.Domain,
			"subdomain":      t.Subdomain,
			"plan":           t.Plan,
			"status":         t.Status,
			"owner_email":    t.OwnerEmail,
			"max_users":      t.MaxUsers,
			"max_storage_mb": t.MaxStorageMB,
			"created_at":     t.CreatedAt,
			"updated_at":     t.UpdatedAt,
		}
	}
	return rows, nil
}

func (p *TenantDataProvider) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	t := p.manager.GetByID(id)
	if t == nil {
		return nil, fmt.Errorf("tenant with id %s not found", id)
	}
	return map[string]interface{}{
		"id":             t.ID,
		"name":           t.Name,
		"slug":           t.Slug,
		"domain":         t.Domain,
		"subdomain":      t.Subdomain,
		"plan":           t.Plan,
		"status":         t.Status,
		"owner_email":    t.OwnerEmail,
		"max_users":      t.MaxUsers,
		"max_storage_mb": t.MaxStorageMB,
		"created_at":     t.CreatedAt,
		"updated_at":     t.UpdatedAt,
	}, nil
}

func (p *TenantDataProvider) Create(ctx *Context, data map[string]interface{}) error {
	t := &Tenant{
		Name:         fmt.Sprint(data["name"]),
		Slug:         fmt.Sprint(data["slug"]),
		Domain:       fmt.Sprint(data["domain"]),
		Subdomain:    fmt.Sprint(data["subdomain"]),
		Plan:         fmt.Sprint(data["plan"]),
		Status:       fmt.Sprint(data["status"]),
		OwnerEmail:   fmt.Sprint(data["owner_email"]),
		MaxUsers:     parseInt(data["max_users"], 10),
		MaxStorageMB: parseInt(data["max_storage_mb"], 1024),
	}
	err := p.manager.Save(t)
	if err == nil {
		data["id"] = t.ID
	}
	return err
}

func (p *TenantDataProvider) Update(ctx *Context, id string, data map[string]interface{}) error {
	t := p.manager.GetByID(id)
	if t == nil {
		return fmt.Errorf("tenant with id %s not found", id)
	}

	if v, ok := data["name"]; ok {
		t.Name = fmt.Sprint(v)
	}
	if v, ok := data["slug"]; ok {
		t.Slug = fmt.Sprint(v)
	}
	if v, ok := data["domain"]; ok {
		t.Domain = fmt.Sprint(v)
	}
	if v, ok := data["subdomain"]; ok {
		t.Subdomain = fmt.Sprint(v)
	}
	if v, ok := data["plan"]; ok {
		t.Plan = fmt.Sprint(v)
	}
	if v, ok := data["status"]; ok {
		t.Status = fmt.Sprint(v)
	}
	if v, ok := data["owner_email"]; ok {
		t.OwnerEmail = fmt.Sprint(v)
	}
	if v, ok := data["max_users"]; ok {
		t.MaxUsers = parseInt(v, t.MaxUsers)
	}
	if v, ok := data["max_storage_mb"]; ok {
		t.MaxStorageMB = parseInt(v, t.MaxStorageMB)
	}

	return p.manager.Save(t)
}

func (p *TenantDataProvider) Delete(ctx *Context, id string) error {
	return p.manager.Delete(id)
}

func parseInt(v interface{}, fallback int) int {
	if v == nil {
		return fallback
	}
	s := strings.TrimSpace(fmt.Sprint(v))
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return fallback
}

// NewController builds the declarative CRUD controller for SaaS Organizations & Tenants
func (tm *TenantManager) NewController() *Controller {
	ctrl := &Controller{
		Title:                "Organizations & Tenants",
		Table:                "tenants",
		Icon:                 `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 21h18"/><path d="M5 21V7l8-4v18"/><path d="M19 21V11l-6-4"/><path d="M9 9h1"/><path d="M9 13h1"/><path d="M9 17h1"/></svg>`,
		DataProvider:         tm.NewDataProvider(),
		TenantScoped:         false,
		TenantScopedExplicit: true,
	}

	ctrl.
		AddCol("ID", "id", ColText, false, true).
		AddCol("Organization Name", "name", ColText, true, true).
		AddCol("Workspace Slug", "slug", ColBadge, true, true).
		AddCol("Custom Domain", "domain", ColText, true, true).
		AddCol("Subdomain", "subdomain", ColText, false, true).
		AddCol("Plan Tier", "plan", ColBadge, false, true).
		AddCol("Status", "status", ColBadge, false, true).
		AddCol("Owner Email", "owner_email", ColEmail, true, true).
		AddCol("Created At", "created_at", ColText, false, true)

	ctrl.
		AddForm("Organization Name", "name", InputText, true, "e.g. PT Kriva Solusi Digital").
		AddForm("Workspace Slug", "slug", InputText, true, "e.g. kriva-solusi").
		AddForm("Custom Domain", "domain", InputText, false, "e.g. erp.client.com (Optional)").
		AddForm("Subdomain Prefix", "subdomain", InputText, false, "e.g. client (Optional)").
		AddSelect("Plan Tier", "plan", true, "Select plan...",
			Option{Label: "Enterprise", Value: "Enterprise"},
			Option{Label: "Pro", Value: "Pro"},
			Option{Label: "Starter", Value: "Starter"},
			Option{Label: "Free", Value: "Free"},
		).
		AddSelect("Account Status", "status", true, "Select status...",
			Option{Label: "Active", Value: "Active"},
			Option{Label: "Trial", Value: "Trial"},
			Option{Label: "Suspended", Value: "Suspended"},
		).
		AddForm("Owner Email", "owner_email", InputEmail, true, "owner@organization.com").
		AddForm("Max Users", "max_users", InputNumber, false, "10").
		AddForm("Max Storage (MB)", "max_storage_mb", InputNumber, false, "1024")

	return ctrl
}
