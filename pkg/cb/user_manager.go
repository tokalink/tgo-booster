package cb

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// UserAccount represents a persistent user identity in data/users.json
type UserAccount struct {
	ID     string `json:"id"`
	Avatar string `json:"avatar"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	// Username: nama masuk opsional, unik se-platform (tanpa huruf besar, tanpa '@'). Login boleh memakai email atau username.
	Username      string   `json:"username,omitempty"`
	PasswordHash  string   `json:"password_hash"`
	Salt          string   `json:"salt"`
	RoleID        string   `json:"role_id"`
	Role          string   `json:"role"`
	Status        string   `json:"status"` // "Active", "Suspended"
	Tenants       []string `json:"tenants,omitempty"`
	DefaultTenant string   `json:"default_tenant,omitempty"`
	CreatedAt     string   `json:"created_at"`
	LastLoginAt   string   `json:"last_login_at,omitempty"`
	// PasswordAlgo: "" (legacy SHA-256+salt, otomatis ditingkatkan saat login) | "bcrypt".
	PasswordAlgo string `json:"password_algo,omitempty"`
	// SessionsValidFrom: sesi yang diterbitkan sebelum waktu ini (RFC3339) ditolak (ganti sandi, cabut sesi).
	SessionsValidFrom string `json:"sessions_valid_from,omitempty"`
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

// hashPassword membuat hash bcrypt (cost 12). Salt lama dipertahankan kosong untuk algoritma baru.
func hashPassword(password string) (hash, salt, algo string, err error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", "", "", err
	}
	return string(b), "", "bcrypt", nil
}

// checkPassword memverifikasi sandi terhadap akun (bcrypt atau SHA-256 lama). legacy=true bila perlu ditingkatkan.
func checkPassword(u *UserAccount, password string) (ok, legacy bool) {
	if u.PasswordAlgo == "bcrypt" {
		return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) == nil, false
	}
	return VerifyUserPassword(password, u.Salt, u.PasswordHash), true
}

// setPasswordLocked mengganti sandi akun (bcrypt) dan mencabut semua sesi sebelumnya. um.mu harus dipegang.
func setPasswordLocked(u *UserAccount, password string) error {
	h, salt, algo, err := hashPassword(password)
	if err != nil {
		return err
	}
	u.PasswordHash, u.Salt, u.PasswordAlgo = h, salt, algo
	u.SessionsValidFrom = time.Now().UTC().Format(time.RFC3339)
	return nil
}

var reservedUsernames = map[string]bool{
	"admin": true, "administrator": true, "root": true, "support": true, "system": true, "superadmin": true, "owner": true,
	"finance": true, "billing": true, "auditor": true, "kriva": true, "help": true, "api": true, "www": true, "mail": true,
	"null": true, "undefined": true, "me": true, "login": true, "logout": true, "signup": true, "security": true,
}

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// ValidateUsername menormalkan (huruf kecil, spasi dipangkas) dan memvalidasi nama masuk: 3–32 karakter a-z 0-9 . _ -,
// diawali huruf/angka, tanpa '@', bukan nama yang dicadangkan.
func ValidateUsername(u string) (string, error) {
	u = strings.ToLower(strings.TrimSpace(u))
	if !usernameRe.MatchString(u) {
		return "", errors.New("Username 3–32 karakter: huruf kecil, angka, titik, garis bawah, atau strip; diawali huruf/angka")
	}
	if strings.Contains(u, "..") || strings.HasSuffix(u, ".") {
		return "", errors.New("Username tidak boleh memuat titik berurutan atau diakhiri titik")
	}
	if reservedUsernames[u] {
		return "", errors.New("Username itu dicadangkan, pilih yang lain")
	}
	return u, nil
}

// dummyHash dipakai untuk menyamakan waktu respons saat akun tidak ditemukan (mencegah enumerasi lewat waktu).
var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

func burnPasswordCheck(password string) {
	dummyHashOnce.Do(func() { dummyHash, _ = bcrypt.GenerateFromPassword([]byte("kriva-dummy"), 12) })
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
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
	lastFail       map[string]time.Time // kegagalan terakhir per kunci; dipakai pembersihan
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
		lastFail:       make(map[string]time.Time),
	}
	um.loadUsers()
	return um
}

func (um *UserManager) loadUsers() {
	um.mu.Lock()
	defer um.mu.Unlock()

	data, err := um.engine.docLoad(DocUsers)
	if err != nil {
		// Galat baca (koneksi/timeout) BUKAN berarti dokumen tidak ada: jangan menyemai dan jangan menimpa.
		log.Printf("[docstore] gagal memuat %s: %v (data di memori dipertahankan, tidak ada seed)", DocUsers, err)
		return
	}
	if len(data) > 0 {
		var list []*UserAccount
		if err := json.Unmarshal(data, &list); err != nil {
			log.Printf("[docstore] %s tidak valid: %v (data di memori dipertahankan, dokumen tidak ditimpa)", DocUsers, err)
			return
		}
		if len(list) > 0 {
			um.users = list
			return
		}
	}

	// Seed akun awal. Produksi (APP_ENV=production): HANYA superadmin dengan kata sandi dari BOOSTER_ADMIN_PASSWORD
	// atau acak (dicetak sekali di log). Pengembangan: akun contoh dengan kata sandi lemah untuk demo lokal.
	prod := os.Getenv("APP_ENV") == "production"
	adminPass := os.Getenv("BOOSTER_ADMIN_PASSWORD")
	if prod && adminPass == "" {
		b := make([]byte, 12)
		_, _ = rand.Read(b)
		adminPass = hex.EncodeToString(b)
		log.Printf("[SECURITY] Kata sandi awal superadmin admin@tgo.io (ganti segera): %s", adminPass)
	}
	if adminPass == "" {
		adminPass = "admin123"
	}
	mk := func(id, avatar, name, email, pass, roleID, role string, tenants []string, created string) *UserAccount {
		h, salt, algo, _ := hashPassword(pass)
		return &UserAccount{ID: id, Avatar: avatar, Name: name, Email: email, PasswordHash: h, Salt: salt, PasswordAlgo: algo,
			RoleID: roleID, Role: role, Status: "Active", Tenants: tenants, DefaultTenant: "default", CreatedAt: created}
	}
	defaultUsers := []*UserAccount{
		mk("1", "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=100&auto=format&fit=crop&q=80", "Super Administrator", "admin@tgo.io", adminPass, "1", "Superadmin", []string{"*"}, "2026-01-10"),
	}
	if !prod {
		defaultUsers = append(defaultUsers,
			mk("2", "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=100&auto=format&fit=crop&q=80", "Budi Operations", "manager@tgo.io", "manager123", "2", "Operations Manager", []string{"default", "demo-enterprise"}, "2026-02-14"),
			mk("3", "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=100&auto=format&fit=crop&q=80", "Siti Auditor", "auditor@tgo.io", "auditor123", "3", "Read-Only Auditor", []string{"default"}, "2026-03-01"),
		)
	}

	um.users = defaultUsers
	_ = um.saveUnlocked()
}

func (um *UserManager) saveUnlocked() error {
	bytes, err := json.MarshalIndent(um.users, "", "  ")
	if err != nil {
		return err
	}
	return um.engine.docSave(DocUsers, bytes)
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

// FindByLogin mencari akun dari email (bila memuat '@') atau username.
func (um *UserManager) FindByLogin(login string) *UserAccount {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return nil
	}
	if strings.Contains(login, "@") {
		return um.FindByEmail(login)
	}
	um.mu.RLock()
	defer um.mu.RUnlock()
	for _, u := range um.users {
		if u.Username != "" && strings.ToLower(u.Username) == login {
			c := *u
			return &c
		}
	}
	return nil
}

// UsernameTaken: username sudah dipakai akun lain (exceptID dikecualikan).
func (um *UserManager) UsernameTaken(username, exceptID string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	um.mu.RLock()
	defer um.mu.RUnlock()
	for _, u := range um.users {
		if u.ID != exceptID && u.Username != "" && strings.ToLower(u.Username) == username {
			return true
		}
	}
	return false
}

// SetUsername mengatur (atau mengosongkan bila "") username akun. Unik se-platform.
func (um *UserManager) SetUsername(id, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username != "" {
		var err error
		if username, err = ValidateUsername(username); err != nil {
			return "", err
		}
	}
	um.mu.Lock()
	defer um.mu.Unlock()
	var target *UserAccount
	for _, u := range um.users {
		if u.ID == id {
			target = u
		} else if username != "" && strings.EqualFold(u.Username, username) {
			return "", errors.New("Username sudah dipakai")
		}
	}
	if target == nil {
		return "", errors.New("user not found")
	}
	target.Username = username
	return username, um.saveUnlocked()
}

// Authenticate verifies email or username and password against stored hash with rate limit checking
func (um *UserManager) Authenticate(email, password, clientIP string, maxAttempts, lockoutMin int) (*UserAccount, error) {
	emailClean := strings.ToLower(strings.TrimSpace(email))
	ipKey := emailClean + "@" + clientIP

	// 1. Lookup User lebih dulu agar kunci per-akun memakai identitas ternormalisasi (email dan username akun yang
	// sama berbagi satu jatah; variasi IP tidak menggandakannya).
	user := um.FindByLogin(emailClean)
	acctKey := "login:" + emailClean
	if user != nil {
		acctKey = "acct:" + user.ID
	}
	acctMax := maxAttempts * 4 // lebih longgar dari per-IP: cegah brute force terdistribusi tanpa mudah mengunci pemilik akun

	// 2. Check Lockout Status (per email@ip dan per akun)
	um.trackerMu.Lock()
	for _, k := range []string{ipKey, acctKey} {
		if until, locked := um.lockedUntil[k]; locked {
			if time.Now().Before(until) {
				remain := time.Until(until).Round(time.Second)
				um.trackerMu.Unlock()
				return nil, fmt.Errorf("Account locked due to too many failed attempts. Try again in %v", remain)
			}
			// Lock expired
			delete(um.lockedUntil, k)
			delete(um.failedAttempts, k)
			delete(um.lastFail, k)
		}
	}
	um.trackerMu.Unlock()

	if user == nil {
		// Samakan waktu respons dan pesan dengan kata sandi salah (tidak membocorkan keberadaan akun)
		burnPasswordCheck(password)
		um.recordFailure(ipKey, maxAttempts, lockoutMin)
		um.recordFailure(acctKey, acctMax, lockoutMin)
		return nil, errors.New("Invalid username/email or password")
	}

	// 3. Verify Password (bcrypt atau SHA-256 lama; tanpa kata sandi bawaan/pintu belakang)
	ok, legacy := checkPassword(user, password)
	if !ok {
		um.recordFailure(ipKey, maxAttempts, lockoutMin)
		um.recordFailure(acctKey, acctMax, lockoutMin)
		return nil, errors.New("Invalid username/email or password")
	}

	// 4. Status akun diperiksa SETELAH kata sandi benar, agar status tidak bisa ditebak tanpa kredensial.
	if strings.EqualFold(user.Status, "Suspended") {
		return nil, errors.New("This administrator account has been suspended. Please contact Super Administrator")
	}
	if legacy {
		// Tingkatkan hash lama ke bcrypt secara transparan (tanpa mencabut sesi yang sedang berjalan)
		um.mu.Lock()
		if h, salt, algo, err := hashPassword(password); err == nil {
			for _, u := range um.users {
				if u.ID == user.ID {
					u.PasswordHash, u.Salt, u.PasswordAlgo = h, salt, algo
				}
			}
			_ = um.saveUnlocked()
		}
		um.mu.Unlock()
	}

	// 5. Success -> Clear failure counters & update LastLoginAt
	um.trackerMu.Lock()
	for _, k := range []string{ipKey, acctKey} {
		delete(um.failedAttempts, k)
		delete(um.lockedUntil, k)
		delete(um.lastFail, k)
	}
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

	um.pruneTrackersLocked()
	um.failedAttempts[key]++
	um.lastFail[key] = time.Now()
	if um.failedAttempts[key] >= maxAttempts {
		um.lockedUntil[key] = time.Now().Add(time.Duration(lockoutMin) * time.Minute)
	}
}

// pruneTrackersLocked membuang entri lockout/kegagalan yang sudah kedaluwarsa bila peta membesar, dengan batas keras
// agar tidak tumbuh tanpa batas (mis. serangan dengan banyak email/IP acak). trackerMu harus dipegang.
func (um *UserManager) pruneTrackersLocked() {
	if len(um.failedAttempts)+len(um.lockedUntil) < 2000 {
		return
	}
	now := time.Now()
	for k, until := range um.lockedUntil {
		if now.After(until) {
			delete(um.lockedUntil, k)
			delete(um.failedAttempts, k)
			delete(um.lastFail, k)
		}
	}
	for k, t := range um.lastFail {
		if _, locked := um.lockedUntil[k]; !locked && now.Sub(t) > time.Hour {
			delete(um.failedAttempts, k)
			delete(um.lastFail, k)
		}
	}
	if len(um.failedAttempts) > 50000 { // batas keras
		um.failedAttempts = make(map[string]int)
		um.lastFail = make(map[string]time.Time)
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
			if err := setPasswordLocked(u, newPassword); err != nil {
				return err
			}
			return um.saveUnlocked()
		}
	}
	return errors.New("user not found")
}

// InvalidateSessions mencabut seluruh sesi akun yang diterbitkan sebelum sekarang.
func (um *UserManager) InvalidateSessions(id string) error {
	um.mu.Lock()
	defer um.mu.Unlock()
	for _, u := range um.users {
		if u.ID == id {
			u.SessionsValidFrom = time.Now().UTC().Format(time.RFC3339)
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

	if password == "" {
		return nil, errors.New("password is required")
	}
	passHash, salt, algo, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	newAcc := &UserAccount{
		ID:           newID,
		Avatar:       avatar,
		Name:         name,
		Email:        emailClean,
		PasswordHash: passHash,
		Salt:         salt,
		PasswordAlgo: algo,
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
		if err := setPasswordLocked(target, password); err != nil {
			return err
		}
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
			"tenants":    strings.Join(u.Tenants, ", "),
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
		"tenants":    strings.Join(u.Tenants, ", "),
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

	acc, err := udp.manager.CreateUser(name, email, password, role, roleID, avatar, status)
	if err != nil {
		return err
	}
	if raw, ok := data["tenants"]; ok {
		if list := parseTenantList(raw); len(list) > 0 {
			return udp.manager.SetTenants(acc.ID, list, list[0])
		}
	}
	return nil
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

	if err := udp.manager.UpdateUser(id, name, email, password, role, roleID, avatar, status); err != nil {
		return err
	}
	if raw, ok := data["tenants"]; ok {
		list := parseTenantList(raw)
		def := ""
		if len(list) > 0 {
			def = list[0]
		}
		return udp.manager.SetTenants(id, list, def)
	}
	return nil
}

// parseTenantList: "toko-a, toko-b" atau []string → daftar slug huruf kecil unik.
func parseTenantList(v interface{}) []string {
	var parts []string
	switch x := v.(type) {
	case string:
		parts = strings.Split(x, ",")
	case []string:
		parts = x
	case []interface{}:
		for _, p := range x {
			parts = append(parts, fmt.Sprint(p))
		}
	}
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

func (udp *UserDataProvider) Delete(ctx *Context, id string) error {
	return udp.manager.DeleteUser(id)
}

// SetTenants mengganti daftar tenant yang boleh diakses akun (dan tenant bawaan).
func (um *UserManager) SetTenants(id string, tenants []string, defaultTenant string) error {
	um.mu.Lock()
	defer um.mu.Unlock()
	for _, u := range um.users {
		if u.ID == id {
			u.Tenants = append([]string(nil), tenants...)
			u.DefaultTenant = defaultTenant
			return um.saveUnlocked()
		}
	}
	return fmt.Errorf("user %s not found", id)
}

// RemoveTenant mencabut satu tenant dari akses akun (idempoten). Bila tenant itu adalah DefaultTenant, kosongkan.
// Sesi aktif akun dicabut (SessionsValidFrom) agar cookie lama tidak memuat slug yang sudah dicabut. Akun superadmin
// ("*") tidak terpengaruh. Mengembalikan akun terbarui; slug yang tidak dimiliki tidak mengubah apa pun.
func (um *UserManager) RemoveTenant(id, slug string) (*UserAccount, error) {
	um.mu.Lock()
	defer um.mu.Unlock()
	for _, u := range um.users {
		if u.ID != id {
			continue
		}
		kept := make([]string, 0, len(u.Tenants))
		removed := false
		for _, t := range u.Tenants {
			if t != "*" && strings.EqualFold(t, slug) {
				removed = true
				continue
			}
			kept = append(kept, t)
		}
		if removed {
			u.Tenants = kept
			if strings.EqualFold(u.DefaultTenant, slug) {
				u.DefaultTenant = ""
			}
			u.SessionsValidFrom = time.Now().UTC().Format(time.RFC3339)
			if err := um.saveUnlocked(); err != nil {
				return nil, err
			}
		}
		cp := *u
		return &cp, nil
	}
	return nil, fmt.Errorf("user %s not found", id)
}

// AddTenant menambahkan satu tenant ke akses akun (idempoten) dan mengembalikan akun terbarui.
func (um *UserManager) AddTenant(id, slug string) (*UserAccount, error) {
	um.mu.Lock()
	defer um.mu.Unlock()
	for _, u := range um.users {
		if u.ID != id {
			continue
		}
		has := false
		for _, t := range u.Tenants {
			if strings.EqualFold(t, slug) {
				has = true
			}
		}
		if !has {
			u.Tenants = append(u.Tenants, slug)
		}
		if u.DefaultTenant == "" {
			u.DefaultTenant = slug
		}
		if err := um.saveUnlocked(); err != nil {
			return nil, err
		}
		cp := *u
		return &cp, nil
	}
	return nil, fmt.Errorf("user %s not found", id)
}
