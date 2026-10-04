package cb

import (
	"fmt"
	"strconv"
	"sync"
)

// MemoryStore is a thread-safe, in-memory implementation of DataProvider.
// It serves as an out-of-the-box data store for prototyping, tests, and MVC models.
type MemoryStore struct {
	mu     sync.RWMutex
	rows   []map[string]interface{}
	nextID int
	idKey  string
}

// NewMemoryStore initializes a new in-memory store with optional initial rows.
func NewMemoryStore(initialRows ...map[string]interface{}) *MemoryStore {
	store := &MemoryStore{
		rows:   make([]map[string]interface{}, 0, len(initialRows)),
		nextID: 1,
		idKey:  "id",
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

// FindAll returns all records matching the query context.
func (s *MemoryStore) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]map[string]interface{}, len(s.rows))
	for i, r := range s.rows {
		clone := make(map[string]interface{}, len(r))
		for k, v := range r {
			clone[k] = v
		}
		result[i] = clone
	}
	return result, nil
}

// FindByID returns a single record by its primary key identifier.
func (s *MemoryStore) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			clone := make(map[string]interface{}, len(r))
			for k, v := range r {
				clone[k] = v
			}
			return clone, nil
		}
	}
	return nil, fmt.Errorf("record with %s=%s not found", s.idKey, id)
}

// Create inserts a new record, automatically allocating an ID if none is supplied.
func (s *MemoryStore) Create(ctx *Context, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

// Update modifies an existing record identified by primary key ID.
func (s *MemoryStore) Update(ctx *Context, id string, data map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			for k, v := range data {
				if k != s.idKey { // preserve immutable primary key
					s.rows[i][k] = v
				}
			}
			return nil
		}
	}
	return fmt.Errorf("record with %s=%s not found", s.idKey, id)
}

// Delete removes a record identified by primary key ID.
func (s *MemoryStore) Delete(ctx *Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, r := range s.rows {
		if fmt.Sprint(r[s.idKey]) == id {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("record with %s=%s not found", s.idKey, id)
}
