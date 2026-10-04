package cb

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EmailTemplate represents an automated email layout & trigger
type EmailTemplate struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	Subject   string `json:"subject"`
	Content   string `json:"content"`
	Status    string `json:"status"` // "Active", "Draft"
	CreatedAt string `json:"created_at"`
}

// EmailTemplateManager manages persistent email templates in data/email_templates.json
type EmailTemplateManager struct {
	engine    *Engine
	mu        sync.RWMutex
	filePath  string
	templates []*EmailTemplate
}

// NewEmailTemplateManager initializes the email templates manager
func NewEmailTemplateManager(engine *Engine) *EmailTemplateManager {
	baseDir := "data"
	_ = os.MkdirAll(baseDir, 0755)
	filePath := filepath.Join(baseDir, "email_templates.json")

	em := &EmailTemplateManager{
		engine:    engine,
		filePath:  filePath,
		templates: make([]*EmailTemplate, 0),
	}
	em.loadTemplates()
	return em
}

func (em *EmailTemplateManager) loadTemplates() {
	em.mu.Lock()
	defer em.mu.Unlock()

	data, err := os.ReadFile(em.filePath)
	if err == nil && len(data) > 0 {
		var list []*EmailTemplate
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			em.templates = list
			return
		}
	}

	// Default seed templates
	now := time.Now().Format("2006-01-02")
	defaults := []*EmailTemplate{
		{
			ID:        "1",
			Title:     "Welcome New Administrator",
			Slug:      "auth.welcome_user",
			Subject:   "Welcome to the Enterprise Admin Console",
			Content:   "<p>Dear <strong>{{name}}</strong>,</p><p>Your administrator access to TGo Booster has been activated. Please log in using your registered credentials.</p>",
			Status:    "Active",
			CreatedAt: now,
		},
		{
			ID:        "2",
			Title:     "Order Payment Confirmation",
			Slug:      "order.paid",
			Subject:   "Your Order #{{order_id}} has been paid successfully",
			Content:   "<p>Hi <strong>{{customer_name}}</strong>,</p><p>We have received your payment for order <strong>#{{order_id}}</strong> totaling <strong>{{total_amount}}</strong>. Your package is now in preparation.</p>",
			Status:    "Active",
			CreatedAt: now,
		},
		{
			ID:        "3",
			Title:     "Password Reset Notification",
			Slug:      "auth.password_reset",
			Subject:   "Security Notice: Password Reset Request",
			Content:   "<p>A request was made to reset your administrator password. If you did not make this request, please contact your Security Team immediately.</p>",
			Status:    "Active",
			CreatedAt: now,
		},
	}

	em.templates = defaults
	_ = em.saveUnlocked()
}

func (em *EmailTemplateManager) saveUnlocked() error {
	bytes, err := json.MarshalIndent(em.templates, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(em.filePath, bytes, 0644)
}

// GetAll returns cloned slice of email templates
func (em *EmailTemplateManager) GetAll() []*EmailTemplate {
	em.mu.RLock()
	defer em.mu.RUnlock()

	out := make([]*EmailTemplate, len(em.templates))
	for i, t := range em.templates {
		cp := *t
		out[i] = &cp
	}
	return out
}

// FindByID finds a template by string ID
func (em *EmailTemplateManager) FindByID(id string) *EmailTemplate {
	em.mu.RLock()
	defer em.mu.RUnlock()

	for _, t := range em.templates {
		if t.ID == id {
			cp := *t
			return &cp
		}
	}
	return nil
}

// FindBySlug finds a template by event slug
func (em *EmailTemplateManager) FindBySlug(slug string) *EmailTemplate {
	em.mu.RLock()
	defer em.mu.RUnlock()

	s := strings.ToLower(strings.TrimSpace(slug))
	for _, t := range em.templates {
		if strings.ToLower(t.Slug) == s {
			cp := *t
			return &cp
		}
	}
	return nil
}

// -------------------------------------------------------------
// EmailTemplateDataProvider implements DataProvider for /admin/email_templates
// -------------------------------------------------------------

type EmailTemplateDataProvider struct {
	manager *EmailTemplateManager
}

func (em *EmailTemplateManager) NewDataProvider() DataProvider {
	return &EmailTemplateDataProvider{manager: em}
}

func (edp *EmailTemplateDataProvider) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	templates := edp.manager.GetAll()
	out := make([]map[string]interface{}, len(templates))
	for i, t := range templates {
		out[i] = map[string]interface{}{
			"id":         t.ID,
			"title":      t.Title,
			"slug":       t.Slug,
			"subject":    t.Subject,
			"content":    t.Content,
			"status":     t.Status,
			"created_at": t.CreatedAt,
		}
	}
	return out, nil
}

func (edp *EmailTemplateDataProvider) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	t := edp.manager.FindByID(id)
	if t == nil {
		return nil, errors.New("email template not found")
	}
	return map[string]interface{}{
		"id":         t.ID,
		"title":      t.Title,
		"slug":       t.Slug,
		"subject":    t.Subject,
		"content":    t.Content,
		"status":     t.Status,
		"created_at": t.CreatedAt,
	}, nil
}

func (edp *EmailTemplateDataProvider) Create(ctx *Context, data map[string]interface{}) error {
	edp.manager.mu.Lock()
	defer edp.manager.mu.Unlock()

	title, _ := data["title"].(string)
	slug, _ := data["slug"].(string)
	subject, _ := data["subject"].(string)
	content, _ := data["content"].(string)
	status, _ := data["status"].(string)

	if status == "" {
		status = "Active"
	}
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(title, " ", "_"))
	}

	maxID := 0
	for _, t := range edp.manager.templates {
		if num, err := strconv.Atoi(t.ID); err == nil && num > maxID {
			maxID = num
		}
	}

	newT := &EmailTemplate{
		ID:        strconv.Itoa(maxID + 1),
		Title:     title,
		Slug:      slug,
		Subject:   subject,
		Content:   content,
		Status:    status,
		CreatedAt: time.Now().Format("2006-01-02"),
	}

	edp.manager.templates = append(edp.manager.templates, newT)
	return edp.manager.saveUnlocked()
}

func (edp *EmailTemplateDataProvider) Update(ctx *Context, id string, data map[string]interface{}) error {
	edp.manager.mu.Lock()
	defer edp.manager.mu.Unlock()

	var target *EmailTemplate
	for _, t := range edp.manager.templates {
		if t.ID == id {
			target = t
			break
		}
	}
	if target == nil {
		return errors.New("email template not found")
	}

	if title, ok := data["title"].(string); ok && title != "" {
		target.Title = title
	}
	if slug, ok := data["slug"].(string); ok && slug != "" {
		target.Slug = slug
	}
	if subject, ok := data["subject"].(string); ok && subject != "" {
		target.Subject = subject
	}
	if content, ok := data["content"].(string); ok {
		target.Content = content
	}
	if status, ok := data["status"].(string); ok && status != "" {
		target.Status = status
	}

	return edp.manager.saveUnlocked()
}

func (edp *EmailTemplateDataProvider) Delete(ctx *Context, id string) error {
	edp.manager.mu.Lock()
	defer edp.manager.mu.Unlock()

	found := false
	newList := make([]*EmailTemplate, 0, len(edp.manager.templates))
	for _, t := range edp.manager.templates {
		if t.ID == id {
			found = true
			continue
		}
		newList = append(newList, t)
	}
	if !found {
		return errors.New("email template not found")
	}

	edp.manager.templates = newList
	return edp.manager.saveUnlocked()
}
