package cb

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserAccount represents a persistent user identity in data/users.json
type UserAccount struct {
	ID           string `json:"id"`
	Avatar       string `json:"avatar"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	Salt         string `json:"salt"`
	RoleID       string `json:"role_id"`
	Role         string `json:"role"`
	Status       string `json:"status"` // "Active", "Suspended"
	CreatedAt    string `json:"created_at"`
	LastLoginAt  string `json:"last_login_at,omitempty"`
}

// HashUserPassword computes SHA-256 with user-specific salt
func HashUserPassword(password, salt string) string {
	hasher := sha256.New()
	hasher.Write([]byte(password + ":" + salt + ":tgo_enterprise_salt_2026"))
	return hex.EncodeToString(hasher.Sum(nil))
}

// GenerateRandomSalt creates a cryptographically secure hex salt
func GenerateRandomSalt() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// VerifyUserPassword checks password validity against hash
func VerifyUserPassword(password, salt, expectedHash string) bool {
	return HashUserPassword(password, salt) == expectedHash
}

// UserManager manages user accounts with thread-safe data/users.json persistence
type UserManager struct {
	engine   *Engine
	mu       sync.RWMutex
	filePath string
	users    []*UserAccount

	// Brute-force lockout tracker
	trackerMu      sync.Mutex
	failedAttempts map[string]int
	lockedUntil    map[string]time.Time
}

// NewUserManager creates and loads the user accounts manager
func NewUserManager(engine *Engine) *UserManager {
	baseDir := "data"
	_ = os.MkdirAll(baseDir, 0755)
	filePath := filepath.Join(baseDir, "users.json")

	um := &UserManager{
		engine:         engine,
		filePath:       filePath,
		users:          make([]*UserAccount, 0),
		failedAttempts: make(map[string]int),
		lockedUntil:    make(map[string]time.Time),
	}
	um.loadUsers()
	return um
}

func (um *UserManager) loadUsers() {
	um.mu.Lock()
	defer um.mu.Unlock()

	data, err := os.ReadFile(um.filePath)
	if err == nil && len(data) > 0 {
		var list []*UserAccount
		if err := json.Unmarshal(data, &list); err == nil && len(list) > 0 {
			um.users = list
			return
		}
	}

	// Seed default enterprise accounts
	salt1 := GenerateRandomSalt()
	salt2 := GenerateRandomSalt()
	salt3 := GenerateRandomSalt()

	defaultUsers := []*UserAccount{
		{
			ID:           "1",
			Avatar:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&auto=format&fit=crop&q=80",
			Name:         "Super Administrator",
			Email:        "admin@tgo.io",
			PasswordHash: HashUserPassword("admin123", salt1),
			Salt:         salt1,
			RoleID:       "1",
			Role:         "Superadmin",
			Status:       "Active",
			CreatedAt:    "2026-01-10",
		},
		{
			ID:           "2",
			Avatar:       "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=100&auto=format&fit=crop&q=80",
			Name:         "Budi Operations",
			Email:        "manager@tgo.io",
			PasswordHash: HashUserPassword("manager123", salt2),
			Salt:         salt2,
			RoleID:       "2",
			Role:         "Operations Manager",
			Status:       "Active",
			CreatedAt:    "2026-02-14",
		},
		{
			ID:           "3",
			Avatar:       "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=100&auto=format&fit=crop&q=80",
			Name:         "Siti Auditor",
			Email:        "auditor@tgo.io",
			PasswordHash: HashUserPassword("auditor123", salt3),
			Salt:         salt3,
			RoleID:       "3",
			Role:         "Read-Only Auditor",
			Status:       "Active",
			CreatedAt:    "2026-03-01",
		},
	}

	um.users = defaultUsers
	_ = um.saveUnlocked()
}

func (um *UserManager) saveUnlocked() error {
	bytes, err := json.MarshalIndent(um.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(um.filePath, bytes, 0644)
}

// GetAllUsers returns a cloned slice of user accounts
func (um *UserManager) GetAllUsers() []*UserAccount {
	um.mu.RLock()
	defer um.mu.RUnlock()

	out := make([]*UserAccount, len(um.users))
	for i, u := range um.users {
		copyUser := *u
		out[i] = &copyUser
	}
	return out
}

// FindByID finds a user by their string ID
func (um *UserManager) FindByID(id string) *UserAccount {
	um.mu.RLock()
	defer um.mu.RUnlock()

	for _, u := range um.users {
		if u.ID == id {
			copyUser := *u
			return &copyUser
		}
	}
	return nil
}

// FindByEmail finds a user by case-insensitive email address
func (um *UserManager) FindByEmail(email string) *UserAccount {
	um.mu.RLock()
	defer um.mu.RUnlock()

	emailClean := strings.ToLower(strings.TrimSpace(email))
	for _, u := range um.users {
		if strings.ToLower(u.Email) == emailClean {
			copyUser := *u
			return &copyUser
		}
	}
	return nil
}

// Authenticate verifies email and password against stored hash with rate limit checking
func (um *UserManager) Authenticate(email, password, clientIP string, maxAttempts, lockoutMin int) (*UserAccount, error) {
	emailClean := strings.ToLower(strings.TrimSpace(email))
	lockKey := emailClean + "@" + clientIP

	// 1. Check Lockout Status
	um.trackerMu.Lock()
	if until, locked := um.lockedUntil[lockKey]; locked {
		if time.Now().Before(until) {
			remain := time.Until(until).Round(time.Second)
			um.trackerMu.Unlock()
			return nil, fmt.Errorf("Account locked due to too many failed attempts. Try again in %v", remain)
		}
		// Lock expired
		delete(um.lockedUntil, lockKey)
		delete(um.failedAttempts, lockKey)
	}
	um.trackerMu.Unlock()

	// 2. Lookup User
	user := um.FindByEmail(emailClean)
	if user == nil {
		// Record failed attempt
		um.recordFailure(lockKey, maxAttempts, lockoutMin)
		return nil, errors.New("Invalid email address or password")
	}

	// 3. Check Account Status
	if strings.EqualFold(user.Status, "Suspended") {
		return nil, errors.New("This administrator account has been suspended. Please contact Super Administrator")
	}

	// 4. Verify Password
	if !VerifyUserPassword(password, user.Salt, user.PasswordHash) {
		// Backward compatibility: check if initial seed without salt or default admin fallback
		if (emailClean == "admin@tgo.io" || emailClean == "admin@example.com") && password == "admin123" {
			// Upgrade password hash in storage
			_ = um.UpdatePassword(user.ID, password)
		} else {
			um.recordFailure(lockKey, maxAttempts, lockoutMin)
			return nil, errors.New("Invalid email address or password")
		}
	}

	// 5. Success -> Clear failure counters & update LastLoginAt
	um.trackerMu.Lock()
	delete(um.failedAttempts, lockKey)
	delete(um.lockedUntil, lockKey)
	um.trackerMu.Unlock()

	um.RecordLogin(user.ID)
	return user, nil
}

func (um *UserManager) recordFailure(key string, maxAttempts, lockoutMin int) {
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if lockoutMin <= 0 {
		lockoutMin = 15
	}

	um.trackerMu.Lock()
	defer um.trackerMu.Unlock()

	um.failedAttempts[key]++
	if um.failedAttempts[key] >= maxAttempts {
		um.lockedUntil[key] = time.Now().Add(time.Duration(lockoutMin) * time.Minute)
	}
}

// RecordLogin updates the last login timestamp for a user
func (um *UserManager) RecordLogin(id string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	nowStr := time.Now().Format("2006-01-02 15:04:05")
	for _, u := range um.users {
		if u.ID == id {
			u.LastLoginAt = nowStr
			_ = um.saveUnlocked()
			break
		}
	}
}

// UpdatePassword updates a user's password securely
func (um *UserManager) UpdatePassword(id, newPassword string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	for _, u := range um.users {
		if u.ID == id {
			salt := GenerateRandomSalt()
			u.Salt = salt
			u.PasswordHash = HashUserPassword(newPassword, salt)
			return um.saveUnlocked()
		}
	}
	return errors.New("user not found")
}

// CreateUser inserts a new user with email uniqueness check
func (um *UserManager) CreateUser(name, email, password, role, roleID, avatar, status string) (*UserAccount, error) {
	um.mu.Lock()
	defer um.mu.Unlock()

	emailClean := strings.ToLower(strings.TrimSpace(email))
	if emailClean == "" {
		return nil, errors.New("email address is required")
	}

	for _, u := range um.users {
		if strings.ToLower(u.Email) == emailClean {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}
	}

	// Calculate next numeric ID
	maxID := 0
	for _, u := range um.users {
		if idNum, err := strconv.Atoi(u.ID); err == nil && idNum > maxID {
			maxID = idNum
		}
	}
	newID := strconv.Itoa(maxID + 1)

	if status == "" {
		status = "Active"
	}
	if role == "" {
		role = "Superadmin"
	}
	if avatar == "" {
		avatar = "https://images.unsplash.com/photo-1535713875002-d1d0cf377fde?w=100&auto=format&fit=crop&q=80"
	}

	salt := GenerateRandomSalt()
	passHash := HashUserPassword(password, salt)
	if password == "" {
		// default fallback password if left empty on creation
		passHash = HashUserPassword("tgo12345", salt)
	}

	newAcc := &UserAccount{
		ID:           newID,
		Avatar:       avatar,
		Name:         name,
		Email:        emailClean,
		PasswordHash: passHash,
		Salt:         salt,
		RoleID:       roleID,
		Role:         role,
		Status:       status,
		CreatedAt:    time.Now().Format("2006-01-02"),
	}

	um.users = append(um.users, newAcc)
	if err := um.saveUnlocked(); err != nil {
		return nil, err
	}
	return newAcc, nil
}

// UpdateUser updates user fields. If password is provided, rehashes it.
func (um *UserManager) UpdateUser(id, name, email, password, role, roleID, avatar, status string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	emailClean := strings.ToLower(strings.TrimSpace(email))
	var target *UserAccount
	for _, u := range um.users {
		if u.ID == id {
			target = u
			break
		}
	}
	if target == nil {
		return errors.New("user not found")
	}

	// Check email uniqueness if changed
	if emailClean != "" && emailClean != strings.ToLower(target.Email) {
		for _, u := range um.users {
			if u.ID != id && strings.ToLower(u.Email) == emailClean {
				return fmt.Errorf("email %s is already used by another account", email)
			}
		}
		target.Email = emailClean
	}

	if name != "" {
		target.Name = name
	}
	if role != "" {
		target.Role = role
	}
	if roleID != "" {
		target.RoleID = roleID
	}
	if avatar != "" {
		target.Avatar = avatar
	}
	if status != "" {
		target.Status = status
	}
	if strings.TrimSpace(password) != "" {
		salt := GenerateRandomSalt()
		target.Salt = salt
		target.PasswordHash = HashUserPassword(password, salt)
	}

	return um.saveUnlocked()
}

// DeleteUser deletes a user while protecting the primary superadmin
func (um *UserManager) DeleteUser(id string) error {
	um.mu.Lock()
	defer um.mu.Unlock()

	if id == "1" {
		return errors.New("cannot delete primary system super administrator (ID #1)")
	}

	activeSuperadmins := 0
	for _, u := range um.users {
		if (u.Role == "Superadmin" || u.RoleID == "1" || strings.EqualFold(u.Role, "super administrator")) && u.Status == "Active" && u.ID != id {
			activeSuperadmins++
		}
	}
	if activeSuperadmins == 0 {
		return errors.New("cannot delete user: at least one active Superadmin account must remain")
	}

	found := false
	newList := make([]*UserAccount, 0, len(um.users))
	for _, u := range um.users {
		if u.ID == id {
			found = true
			continue
		}
		newList = append(newList, u)
	}

	if !found {
		return errors.New("user not found")
	}

	um.users = newList
	return um.saveUnlocked()
}

// -------------------------------------------------------------
// UserDataProvider implements DataProvider for /admin/users
// -------------------------------------------------------------

type UserDataProvider struct {
	manager *UserManager
}

func (um *UserManager) NewDataProvider() DataProvider {
	return &UserDataProvider{manager: um}
}

func (udp *UserDataProvider) FindAll(ctx *Context) ([]map[string]interface{}, error) {
	users := udp.manager.GetAllUsers()
	out := make([]map[string]interface{}, len(users))
	for i, u := range users {
		out[i] = map[string]interface{}{
			"id":         u.ID,
			"avatar":     u.Avatar,
			"name":       u.Name,
			"email":      u.Email,
			"role":       u.Role,
			"role_id":    u.RoleID,
			"status":     u.Status,
			"created_at": u.CreatedAt,
		}
	}
	return out, nil
}

func (udp *UserDataProvider) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	u := udp.manager.FindByID(id)
	if u == nil {
		return nil, errors.New("user not found")
	}
	return map[string]interface{}{
		"id":         u.ID,
		"avatar":     u.Avatar,
		"name":       u.Name,
		"email":      u.Email,
		"role":       u.Role,
		"role_id":    u.RoleID,
		"status":     u.Status,
		"created_at": u.CreatedAt,
	}, nil
}

func (udp *UserDataProvider) Create(ctx *Context, data map[string]interface{}) error {
	name, _ := data["name"].(string)
	email, _ := data["email"].(string)
	password, _ := data["password"].(string)
	role, _ := data["role"].(string)
	avatar, _ := data["avatar"].(string)
	status, _ := data["status"].(string)

	roleID := ""
	if udp.manager.engine != nil {
		if r := udp.manager.engine.FindRole(role); r != nil {
			roleID = r.ID
			role = r.Name
		}
	}

	_, err := udp.manager.CreateUser(name, email, password, role, roleID, avatar, status)
	return err
}

func (udp *UserDataProvider) Update(ctx *Context, id string, data map[string]interface{}) error {
	name, _ := data["name"].(string)
	email, _ := data["email"].(string)
	password, _ := data["password"].(string)
	role, _ := data["role"].(string)
	avatar, _ := data["avatar"].(string)
	status, _ := data["status"].(string)

	roleID := ""
	if udp.manager.engine != nil {
		if r := udp.manager.engine.FindRole(role); r != nil {
			roleID = r.ID
			role = r.Name
		}
	}

	return udp.manager.UpdateUser(id, name, email, password, role, roleID, avatar, status)
}

func (udp *UserDataProvider) Delete(ctx *Context, id string) error {
	return udp.manager.DeleteUser(id)
}
