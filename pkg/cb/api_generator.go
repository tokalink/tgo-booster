package cb

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

var (
	tokenPersistenceMu sync.RWMutex
	rateLimitCacheMu   sync.Mutex
	rateLimitCounters  = make(map[string]*tokenRateTracker)
)

type tokenRateTracker struct {
	windowStart time.Time
	count       int
}

// APIToken represents an enterprise API Key / Bearer Access Token
type APIToken struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Token       string   `json:"token"`
	SecretMask  string   `json:"secret_mask"`
	Environment string   `json:"environment"` // "production", "staging", "sandbox"
	Scopes      []string `json:"scopes"`      // e.g. ["*"], ["read:products", "write:products"]
	RateLimit   int      `json:"rate_limit"`  // requests per minute (0 = unlimited)
	Status      string   `json:"status"`      // "active", "revoked"
	CreatedAt   string   `json:"created_at"`
	LastUsedAt  string   `json:"last_used_at"`
}

// APIEndpointInfo describes a live API endpoint in the system
type APIEndpointInfo struct {
	Method      string            `json:"method"`      // "GET", "POST", "PUT", "DELETE"
	Path        string            `json:"path"`        // "/api/v1/products"
	Module      string            `json:"module"`      // "products"
	Description string            `json:"description"` // Human-friendly summary
	Scope       string            `json:"scope"`       // Required scope (e.g. "read:products")
	Protocol    string            `json:"protocol"`    // "REST / JSON"
	SampleBody  string            `json:"sample_body,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`
}

// GenerateSecureAPIToken creates a cryptographically secure token
func GenerateSecureAPIToken(env string) string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	prefix := "tgo_live_"
	if env == "sandbox" || env == "staging" {
		prefix = "tgo_test_"
	}
	return prefix + hex.EncodeToString(bytes)
}

// MaskAPIToken formats a token with middle characters masked
func MaskAPIToken(token string) string {
	if len(token) <= 12 {
		return token
	}
	prefix := token[:9]
	suffix := token[len(token)-8:]
	return prefix + "••••••••••••••••" + suffix
}

// getAPITokensPersistencePath returns the location of api_tokens.json
func (e *Engine) getAPITokensPersistencePath() string {
	candidates := []string{
		"data/api_tokens.json",
		"starter/data/api_tokens.json",
		"../data/api_tokens.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if err := os.MkdirAll("data", 0755); err == nil {
		return "data/api_tokens.json"
	}
	return "api_tokens.json"
}

// LoadDynamicTokens loads tokens from disk into memory
func (e *Engine) LoadDynamicTokens() []*APIToken {
	tokenPersistenceMu.Lock()
	defer tokenPersistenceMu.Unlock()

	p := e.getAPITokensPersistencePath()
	raw, err := os.ReadFile(p)
	if err != nil {
		e.apiTokens = make([]*APIToken, 0)
		return e.apiTokens
	}

	var list []*APIToken
	if err := json.Unmarshal(raw, &list); err != nil {
		e.apiTokens = make([]*APIToken, 0)
		return e.apiTokens
	}

	for _, t := range list {
		if t.SecretMask == "" {
			t.SecretMask = MaskAPIToken(t.Token)
		}
	}

	e.apiTokens = list
	return e.apiTokens
}

// GetAPITokens returns all current tokens
func (e *Engine) GetAPITokens() []*APIToken {
	tokenPersistenceMu.RLock()
	defer tokenPersistenceMu.RUnlock()

	res := make([]*APIToken, len(e.apiTokens))
	copy(res, e.apiTokens)
	return res
}

// SaveAPIToken persists or updates a token
func (e *Engine) SaveAPIToken(t *APIToken) error {
	tokenPersistenceMu.Lock()
	defer tokenPersistenceMu.Unlock()

	if t.ID == "" {
		t.ID = "token_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	if t.Token == "" {
		t.Token = GenerateSecureAPIToken(t.Environment)
	}
	t.SecretMask = MaskAPIToken(t.Token)
	if t.CreatedAt == "" {
		t.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	if t.Status == "" {
		t.Status = "active"
	}
	if len(t.Scopes) == 0 {
		t.Scopes = []string{"*"}
	}

	found := false
	for i, existing := range e.apiTokens {
		if existing.ID == t.ID {
			e.apiTokens[i] = t
			found = true
			break
		}
	}
	if !found {
		e.apiTokens = append(e.apiTokens, t)
	}

	return e.persistAPITokensLocked()
}

// DeleteAPIToken deletes or revokes a token
func (e *Engine) DeleteAPIToken(id string) error {
	tokenPersistenceMu.Lock()
	defer tokenPersistenceMu.Unlock()

	newList := make([]*APIToken, 0, len(e.apiTokens))
	for _, t := range e.apiTokens {
		if t.ID != id {
			newList = append(newList, t)
		}
	}
	e.apiTokens = newList
	return e.persistAPITokensLocked()
}

func (e *Engine) persistAPITokensLocked() error {
	p := e.getAPITokensPersistencePath()
	dir := filepath.Dir(p)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}
	raw, err := json.MarshalIndent(e.apiTokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0644)
}

// ValidateAPIToken checks Bearer/X-API-Key credentials and scope authorization
func (e *Engine) ValidateAPIToken(rawToken, requiredScope string) (*APIToken, bool, string) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return nil, false, "Missing Authorization Bearer or X-API-Key token"
	}

	tokenPersistenceMu.RLock()
	defer tokenPersistenceMu.RUnlock()

	var matched *APIToken
	for _, t := range e.apiTokens {
		if t.Token == rawToken {
			matched = t
			break
		}
	}

	if matched == nil {
		return nil, false, "Invalid or unrecognized API token"
	}
	if matched.Status != "active" {
		return nil, false, "This API token has been revoked or deactivated"
	}

	// Scope checking
	if requiredScope != "" {
		hasScope := false
		for _, s := range matched.Scopes {
			if s == "*" || s == "admin:all" || s == requiredScope {
				hasScope = true
				break
			}
			// Check prefix wildcard like "read:*" or "write:*"
			parts := strings.Split(requiredScope, ":")
			if len(parts) == 2 {
				if s == parts[0]+":*" || s == parts[0]+":all" {
					hasScope = true
					break
				}
			}
		}
		if !hasScope {
			return matched, false, fmt.Sprintf("Forbidden: Token lacks required scope '%s'", requiredScope)
		}
	}

	// Rate limiting enforcement
	if matched.RateLimit > 0 {
		rateLimitCacheMu.Lock()
		tracker, exists := rateLimitCounters[matched.ID]
		now := time.Now()
		if !exists || now.Sub(tracker.windowStart) > time.Minute {
			rateLimitCounters[matched.ID] = &tokenRateTracker{
				windowStart: now,
				count:       1,
			}
		} else {
			if tracker.count >= matched.RateLimit {
				rateLimitCacheMu.Unlock()
				return matched, false, fmt.Sprintf("Rate limit exceeded: Max %d requests per minute allowed", matched.RateLimit)
			}
			tracker.count++
		}
		rateLimitCacheMu.Unlock()
	}

	// Update LastUsedAt (async/non-blocking)
	go func(tokID string) {
		tokenPersistenceMu.Lock()
		for _, t := range e.apiTokens {
			if t.ID == tokID {
				t.LastUsedAt = time.Now().Format("2006-01-02 15:04:05")
				_ = e.persistAPITokensLocked()
				break
			}
		}
		tokenPersistenceMu.Unlock()
	}(matched.ID)

	return matched, true, ""
}

// DiscoverAPIEndpoints dynamically compiles all endpoints across registered controllers
func (e *Engine) DiscoverAPIEndpoints() []APIEndpointInfo {
	endpoints := []APIEndpointInfo{
		{
			Method:      "GET",
			Path:        "/api/v1/health",
			Module:      "System",
			Description: "Health check, server uptime, and kernel status beacon",
			Scope:       "read:all",
			Protocol:    "REST / JSON",
		},
	}

	for _, c := range e.controllers {
		mod := c.Table
		title := c.Title
		if title == "" {
			title = mod
		}

		endpoints = append(endpoints,
			APIEndpointInfo{
				Method:      "GET",
				Path:        fmt.Sprintf("/api/v1/%s", mod),
				Module:      title,
				Description: fmt.Sprintf("Query and paginate %s records with search & filter support", title),
				Scope:       fmt.Sprintf("read:%s", mod),
				Protocol:    "REST / JSON",
				QueryParams: map[string]string{
					"q":     "Search query across all indexed columns",
					"page":  "Pagination index (default: 1)",
					"limit": "Max rows per page (default: 25)",
				},
			},
			APIEndpointInfo{
				Method:      "GET",
				Path:        fmt.Sprintf("/api/v1/%s/{id}", mod),
				Module:      title,
				Description: fmt.Sprintf("Retrieve a single %s item by primary key identifier", title),
				Scope:       fmt.Sprintf("read:%s", mod),
				Protocol:    "REST / JSON",
			},
			APIEndpointInfo{
				Method:      "POST",
				Path:        fmt.Sprintf("/api/v1/%s", mod),
				Module:      title,
				Description: fmt.Sprintf("Create a new %s entry with JSON payload validation", title),
				Scope:       fmt.Sprintf("write:%s", mod),
				Protocol:    "REST / JSON",
				SampleBody:  e.generateSampleJSONPayload(c),
			},
			APIEndpointInfo{
				Method:      "PUT",
				Path:        fmt.Sprintf("/api/v1/%s/{id}", mod),
				Module:      title,
				Description: fmt.Sprintf("Update fields of an existing %s item by primary key", title),
				Scope:       fmt.Sprintf("write:%s", mod),
				Protocol:    "REST / JSON",
				SampleBody:  e.generateSampleJSONPayload(c),
			},
			APIEndpointInfo{
				Method:      "DELETE",
				Path:        fmt.Sprintf("/api/v1/%s/{id}", mod),
				Module:      title,
				Description: fmt.Sprintf("Permanently remove a %s item from persistence storage", title),
				Scope:       fmt.Sprintf("write:%s", mod),
				Protocol:    "REST / JSON",
			},
		)
	}

	return endpoints
}

func (e *Engine) generateSampleJSONPayload(c *Controller) string {
	sample := make(map[string]interface{})
	for _, f := range c.Forms {
		if f.Name == "id" {
			continue
		}
		switch f.Type {
		case InputNumber, InputMoney:
			sample[f.Name] = 100
		case InputCheckbox:
			sample[f.Name] = true
		default:
			sample[f.Name] = "Sample " + f.Label
		}
	}
	raw, _ := json.MarshalIndent(sample, "", "  ")
	return string(raw)
}

// GenerateOpenAPISpec generates an OpenAPI 3.0.0 JSON specification
func (e *Engine) GenerateOpenAPISpec() map[string]interface{} {
	endpoints := e.DiscoverAPIEndpoints()

	paths := make(map[string]interface{})
	for _, ep := range endpoints {
		pathKey := ep.Path
		if _, ok := paths[pathKey]; !ok {
			paths[pathKey] = make(map[string]interface{})
		}
		pathObj := paths[pathKey].(map[string]interface{})

		methodKey := strings.ToLower(ep.Method)
		opObj := map[string]interface{}{
			"summary":     ep.Description,
			"tags":        []string{ep.Module},
			"operationId": fmt.Sprintf("%s_%s", methodKey, strings.ReplaceAll(strings.Trim(ep.Path, "/"), "/", "_")),
			"security": []map[string][]string{
				{"BearerAuth": []string{ep.Scope}},
			},
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Successful operation",
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"success": map[string]interface{}{"type": "boolean"},
									"data":    map[string]interface{}{"type": "object"},
								},
							},
						},
					},
				},
				"401": map[string]interface{}{"description": "Unauthorized - Missing or invalid token"},
				"403": map[string]interface{}{"description": "Forbidden - Insufficient scope permissions"},
				"429": map[string]interface{}{"description": "Too Many Requests - Rate limit exceeded"},
			},
		}

		if len(ep.QueryParams) > 0 {
			var params []map[string]interface{}
			for qName, qDesc := range ep.QueryParams {
				params = append(params, map[string]interface{}{
					"name":        qName,
					"in":          "query",
					"description": qDesc,
					"required":    false,
					"schema":      map[string]interface{}{"type": "string"},
				})
			}
			opObj["parameters"] = params
		}

		if ep.Method == "POST" || ep.Method == "PUT" {
			opObj["requestBody"] = map[string]interface{}{
				"required": true,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{"type": "object"},
					},
				},
			}
		}

		pathObj[methodKey] = opObj
	}

	return map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":       fmt.Sprintf("%s REST & ConnectRPC API", e.AppName),
			"version":     "1.0.0",
			"description": "High-performance, zero-alloc RESTful HTTP/2 API kernel powered by TGo Booster.",
		},
		"servers": []map[string]interface{}{
			{"url": "/"},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"BearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "APIKey",
					"description":  "Provide your API token in the Authorization header: `Bearer tgo_live_...` or via `X-API-Key` header.",
				},
			},
		},
	}
}

// registerPublicAPIRoutes mounts the live /api/v1/ public REST API
func (e *Engine) registerPublicAPIRoutes(server connect.Server) {
	// CORS handler wrapper
	withCORS := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next(w, r)
		}
	}

	// GET /api/v1/health
	server.Register("/api/v1/health", withCORS(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"status":  "healthy",
			"kernel":  "TGo HTTP/2 Zero-Alloc Core",
			"uptime":  time.Since(serverStartTime).String(),
			"modules": len(e.controllers),
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))

	// Dynamic handler for /api/v1/{module} and /api/v1/{module}/{id}
	for _, c := range e.controllers {
		mod := c.Table
		ctrl := c

		routePrefix := "/api/v1/" + mod

		// Handler for collection (/api/v1/products) and item (/api/v1/products/)
		handler := withCORS(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Extract token from Authorization header or X-API-Key or query string
			rawToken := ""
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				rawToken = strings.TrimPrefix(authHeader, "Bearer ")
			} else if k := r.Header.Get("X-API-Key"); k != "" {
				rawToken = k
			} else if qk := r.URL.Query().Get("api_key"); qk != "" {
				rawToken = qk
			}

			// Determine required scope
			requiredScope := "read:" + mod
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
				requiredScope = "write:" + mod
			}

			// Validate token
			tok, valid, errMsg := e.ValidateAPIToken(rawToken, requiredScope)
			if !valid {
				status := http.StatusUnauthorized
				if strings.Contains(errMsg, "Forbidden") {
					status = http.StatusForbidden
				} else if strings.Contains(errMsg, "Rate limit") {
					status = http.StatusTooManyRequests
					w.Header().Set("Retry-After", "60")
				}
				w.WriteHeader(status)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   errMsg,
				})
				return
			}

			// Build Context for DataProvider lifecycle
			clientName := "API Client"
			if tok != nil {
				clientName = "API Client (" + tok.Name + ")"
			}
			reqCtx := &Context{
				Ctx:     r.Context(),
				Request: r,
				User:    &User{Name: clientName, RoleName: "API Client"},
			}

			// Extract sub-path (for ID)
			subPath := strings.TrimPrefix(r.URL.Path, routePrefix)
			subPath = strings.Trim(subPath, "/")

			switch r.Method {
			case http.MethodGet:
				if subPath != "" {
					// Single item lookup
					row, err := ctrl.DataProvider.FindByID(reqCtx, subPath)
					if err != nil || row == nil {
						w.WriteHeader(http.StatusNotFound)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"success": false,
							"error":   fmt.Sprintf("Item with ID '%s' not found", subPath),
						})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": true,
						"data":    row,
					})
					return
				}

				// Collection query
				rows, err := ctrl.DataProvider.FindAll(reqCtx)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   err.Error(),
					})
					return
				}
				// Search filter
				q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
				filtered := make([]map[string]interface{}, 0, len(rows))
				for _, row := range rows {
					if q == "" {
						filtered = append(filtered, row)
						continue
					}
					match := false
					for _, v := range row {
						if strings.Contains(strings.ToLower(fmt.Sprint(v)), q) {
							match = true
							break
						}
					}
					if match {
						filtered = append(filtered, row)
					}
				}

				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"count":   len(filtered),
					"data":    filtered,
				})

			case http.MethodPost:
				var payload map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Invalid JSON request body: " + err.Error(),
					})
					return
				}
				if err := ctrl.DataProvider.Create(reqCtx, payload); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Failed to persist item: " + err.Error(),
					})
					return
				}
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": fmt.Sprintf("%s record created successfully", mod),
					"data":    payload,
				})

			case http.MethodPut:
				if subPath == "" {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Item ID path parameter required for PUT updates",
					})
					return
				}
				var payload map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Invalid JSON request body: " + err.Error(),
					})
					return
				}
				if err := ctrl.DataProvider.Update(reqCtx, subPath, payload); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Failed to update item: " + err.Error(),
					})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": fmt.Sprintf("%s record '%s' updated successfully", mod, subPath),
					"data":    payload,
				})

			case http.MethodDelete:
				if subPath == "" {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Item ID path parameter required for DELETE operations",
					})
					return
				}
				if err := ctrl.DataProvider.Delete(reqCtx, subPath); err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"success": false,
						"error":   "Failed to delete item: " + err.Error(),
					})
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": fmt.Sprintf("%s record '%s' deleted successfully", mod, subPath),
				})

			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   "Method not allowed",
				})
			}
		})

		server.Register(routePrefix, handler)
		server.Register(routePrefix+"/", handler)
	}
}

// registerAPIGeneratorRoutes mounts all Studio endpoints for /admin/api_generator
func (e *Engine) registerAPIGeneratorRoutes(server connect.Server) {
	studioPath := e.AdminPath + "/api_generator"

	// GET /admin/api_generator -> Studio Dashboard View
	server.Register(studioPath, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				e.RenderForbidden(w, r, "API & Tokens Studio", "Superadmin")
				return
			}
		}

		tokens := e.GetAPITokens()
		endpoints := e.DiscoverAPIEndpoints()

		data := map[string]interface{}{
			"AdminPath": e.AdminPath,
			"AppName":   e.AppName,
			"Tokens":    tokens,
			"Endpoints": endpoints,
			"Modules":   e.controllers,
		}

		contentHTML, err := RenderAPIGeneratorContent(data)
		if err != nil {
			http.Error(w, "Failed to render API Studio: "+err.Error(), http.StatusInternalServerError)
			return
		}

		e.RenderLayout(w, r, "API & Tokens Studio", contentHTML)
	}))
	server.Register(studioPath+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, studioPath, http.StatusMovedPermanently)
	}))

	// POST /admin/api_generator/save -> Create or update token
	server.Register(studioPath+"/save", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					"error":   "Forbidden: Superadmin role required to manage API access tokens",
				})
				return
			}
		}

		var payload APIToken
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid token JSON payload: " + err.Error(),
			})
			return
		}

		if strings.TrimSpace(payload.Name) == "" {
			payload.Name = "Untitled API Client"
		}
		if payload.Environment == "" {
			payload.Environment = "production"
		}

		isNew := payload.ID == ""
		if isNew {
			payload.Token = GenerateSecureAPIToken(payload.Environment)
		}

		if err := e.SaveAPIToken(&payload); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to persist token: " + err.Error(),
			})
			return
		}

		actionType := "TOKEN_CREATE"
		if !isNew {
			actionType = "TOKEN_UPDATE"
		}
		e.LogAudit(r, actionType, "API Generator", fmt.Sprintf("Saved API client '%s' (Env: %s, ID: %s)", payload.Name, payload.Environment, payload.ID))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message":    "API Token saved successfully!",
			"is_new":     isNew,
			"token":      payload.Token,
			"token_mask": payload.SecretMask,
			"id":         payload.ID,
		})
	}))

	// POST /admin/api_generator/delete -> Revoke or delete token
	server.Register(studioPath+"/delete", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
					"error":   "Forbidden: Superadmin role required to revoke API access tokens",
				})
				return
			}
		}

		var payload struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.ID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Missing or invalid token ID",
			})
			return
		}

		if err := e.DeleteAPIToken(payload.ID); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Failed to delete token: " + err.Error(),
			})
			return
		}

		e.LogAudit(r, "TOKEN_REVOKE", "API Generator", fmt.Sprintf("Revoked and removed API token ID #%s", payload.ID))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "API token revoked and removed successfully",
		})
	}))

	// GET /admin/api_generator/openapi.json -> Download OpenAPI spec
	server.Register(studioPath+"/openapi.json", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		spec := e.GenerateOpenAPISpec()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\"openapi.json\"")
		_ = json.NewEncoder(w).Encode(spec)
	}))

	// POST /admin/api_generator/test-request -> In-browser live endpoint tester
	server.Register(studioPath+"/test-request", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Method  string `json:"method"`
			Path    string `json:"path"`
			Token   string `json:"token"`
			Payload string `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Invalid test request: " + err.Error(),
			})
			return
		}

		// Perform local in-process dispatch simulation
		start := time.Now()

		targetURL := req.Path
		if !strings.HasPrefix(targetURL, "/") {
			targetURL = "/" + targetURL
		}

		// If no token specified, use first active token
		if req.Token == "" {
			tokens := e.GetAPITokens()
			for _, t := range tokens {
				if t.Status == "active" {
					req.Token = t.Token
					break
				}
			}
		}

		var bodyReader = strings.NewReader(req.Payload)
		httpReq, _ := http.NewRequest(req.Method, targetURL, bodyReader)
		if req.Token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+req.Token)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		// Create response recorder
		wRec := &memoryResponseWriter{
			headers: make(http.Header),
		}

		// Dispatch via engine's server handler
		if e.server != nil && e.server.Handler() != nil {
			e.server.Handler().ServeHTTP(wRec, httpReq)
		} else {
			wRec.status = http.StatusServiceUnavailable
			wRec.body = []byte(`{"error":"Server kernel handler not active"}`)
		}

		duration := time.Since(start)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     wRec.status < 400,
			"status_code": wRec.status,
			"latency_ms":  float64(duration.Microseconds()) / 1000.0,
			"headers":     wRec.headers,
			"body":        string(wRec.body),
		})
	}))
}

// memoryResponseWriter is a lightweight ResponseWriter for internal test execution
type memoryResponseWriter struct {
	headers http.Header
	status  int
	body    []byte
}

func (m *memoryResponseWriter) Header() http.Header {
	return m.headers
}

func (m *memoryResponseWriter) Write(b []byte) (int, error) {
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *memoryResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
}
