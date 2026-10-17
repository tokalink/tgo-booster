package cb

import (
	"fmt"
	"strconv"
	"sync"
)

// MemoryStore is a thread-safe, in-memory implementation of DataProvider.
// It serves as an out-of-the-box data store for prototyping, tests, and MVC models.
// It also supports native SaaS row-level multi-tenancy.
type MemoryStore struct {
	mu           sync.RWMutex
	rows         []map[string]interface{}
	nextID       int
	idKey        string
	tenantScoped bool
	tenantCol    string
}

// NewMemoryStore initializes a new in-memory store with optional initial rows.
func NewMemoryStore(initialRows ...map[string]interface{}) *MemoryStore {
	store := &MemoryStore{
		rows:      make([]map[string]interface{}, 0, len(initialRows)),
		nextID:    1,
		idKey:     "id",
		tenantCol: "tenant_id",
	}

	maxID := 0
	for _, r := range initialRows {
		clone := make(map[string]interface{}, len(r))
		for k, v := range r {
			clone[k] = v
		}
		if idVal, ok := clone[store.idKey]; ok {
			if idInt, err := strconv.Atoi(fmt.Sprint(idVal)); err == nil && idInt > maxID {
				maxID = idInt
			}
		}
		store.rows = append(store.rows, clone)
	}
	if maxID > 0 {
		store.nextID = maxID + 1
	}

	return store
}

// SetIDKey configures the primary key column name (defaults to "id").
func (s *MemoryStore) SetIDKey(key string) *MemoryStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idKey = key
	return s
}

// SetTenantScoped configures whether this store isolates records by tenant ID (defaults to "tenant_id")
func (s *MemoryStore) SetTenantScoped(scoped bool, col ...string) *MemoryStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenantScoped = scoped
	if len(col) > 0 && col[0] != "" {
		s.tenantCol = col[0]
	} else if s.tenantCol == "" {
		s.tenantCol = "tenant_id"
	}
	return s
}

// IsTenantScoped returns true if tenant scoping is active
func (s *MemoryStore) IsTenantScoped() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tenantScoped
}

// TenantCol returns the tenant column name
func (s *MemoryStore) TenantCol() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.tenantCol == "" {
		return "tenant_id"
	}
	return s.tenantCol
}

// FindAll returns all records matching the query context.
func (s *MemoryStore) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantSlug := ""
	if ctx != nil {
		tenantSlug = ctx.TenantSlug
	}

	result := make([]map[string]interface{}, 0, len(s.rows))
	for _, r := range s.rows {
		if s.tenantScoped && tenantSlug != "" && tenantSlug != "*" {
			rTenant := fmt.Sprint(r[s.tenantCol])
			if rTenant != "" && rTenant != tenantSlug {
				continue
			}
		}
		clone := make(map[string]interface{}, len(r))
		for k, v := range r {
			clone[k] = v
		}
		result = append(result, clone)
	}
	return result, nil
}

// FindByID returns a single record by its primary key identifier and tenant scope.
func (s *MemoryStore) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenantSlug := ""
	if ctx != nil {
		tenantSlug = ctx.TenantSlug
	}

	for _, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			if s.tenantScoped && tenantSlug != "" && tenantSlug != "*" {
				rTenant := fmt.Sprint(r[s.tenantCol])
				if rTenant != "" && rTenant != tenantSlug {
					continue
				}
			}
			clone := make(map[string]interface{}, len(r))
			for k, v := range r {
				clone[k] = v
			}
			return clone, nil
		}
	}
	return nil, fmt.Errorf("record with %s=%s not found", s.idKey, id)
}

// Create inserts a new record, automatically allocating an ID and tenant ID if none is supplied.
func (s *MemoryStore) Create(ctx *Context, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantSlug := ""
	if ctx != nil {
		tenantSlug = ctx.TenantSlug
	}

	if s.tenantScoped && tenantSlug != "" && tenantSlug != "*" {
		if _, ok := data[s.tenantCol]; !ok || fmt.Sprint(data[s.tenantCol]) == "" {
			data[s.tenantCol] = tenantSlug
		}
	}

	if _, ok := data[s.idKey]; !ok || fmt.Sprint(data[s.idKey]) == "" {
		data[s.idKey] = strconv.Itoa(s.nextID)
		s.nextID++
	}

	clone := make(map[string]interface{}, len(data))
	for k, v := range data {
		clone[k] = v
	}
	s.rows = append(s.rows, clone)
	return nil
}

// Update modifies an existing record identified by primary key ID and tenant scope.
func (s *MemoryStore) Update(ctx *Context, id string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantSlug := ""
	if ctx != nil {
		tenantSlug = ctx.TenantSlug
	}

	for i, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			if s.tenantScoped && tenantSlug != "" && tenantSlug != "*" {
				rTenant := fmt.Sprint(r[s.tenantCol])
				if rTenant != "" && rTenant != tenantSlug {
					return fmt.Errorf("record with %s=%s not found", s.idKey, id)
				}
			}
			for k, v := range data {
				if k != s.idKey && (!s.tenantScoped || k != s.tenantCol) { // preserve immutable PK & tenant
					s.rows[i][k] = v
				}
			}
			return nil
		}
	}
	return fmt.Errorf("record with %s=%s not found", s.idKey, id)
}

// Delete removes a record identified by primary key ID and tenant scope.
func (s *MemoryStore) Delete(ctx *Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenantSlug := ""
	if ctx != nil {
		tenantSlug = ctx.TenantSlug
	}

	for i, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			if s.tenantScoped && tenantSlug != "" && tenantSlug != "*" {
				rTenant := fmt.Sprint(r[s.tenantCol])
				if rTenant != "" && rTenant != tenantSlug {
					return fmt.Errorf("record with %s=%s not found", s.idKey, id)
				}
			}
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("record with %s=%s not found", s.idKey, id)
}
