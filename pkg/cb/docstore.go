package cb

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DocStore adalah penyimpanan dokumen konfigurasi Booster (pengguna, peran, menu, halaman, tenant, template email,
// token API, pengaturan, definisi modul). Bawaannya berkas JSON di folder data/ (cocok untuk pengembangan);
// aplikasi produksi memasang implementasi basis data lewat Engine.SetDocStore agar konfigurasi tidak bergantung
// pada berkas lokal dan dapat dipakai bersama oleh beberapa instance.
type DocStore interface {
	// Load mengembalikan isi dokumen; (nil, nil) bila dokumen belum ada.
	Load(name string) ([]byte, error)
	// Save menulis dokumen (membuat bila belum ada).
	Save(name string, data []byte) error
}

// Nama dokumen yang dikelola Booster.
const (
	DocUsers          = "users"
	DocPrivileges     = "privileges"
	DocMenus          = "menus"
	DocPages          = "pages"
	DocTenants        = "tenants"
	DocEmailTemplates = "email_templates"
	DocAPITokens      = "api_tokens"
	DocSettings       = "settings"
	DocModules        = "modules"
)

// DocNames: seluruh dokumen yang dipindahkan oleh SetDocStore.
var DocNames = []string{DocUsers, DocPrivileges, DocMenus, DocPages, DocTenants, DocEmailTemplates, DocAPITokens, DocSettings, DocModules}

// FileDocStore menyimpan dokumen sebagai data/<nama>.json (folder dicari seperti perilaku lama).
type FileDocStore struct{}

func (FileDocStore) path(name string) string {
	for _, dir := range []string{"data", "starter/data", "../starter/data"} {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return filepath.Join(dir, name+".json")
		}
	}
	return filepath.Join("data", name+".json")
}

func (f FileDocStore) Load(name string) ([]byte, error) {
	b, err := os.ReadFile(f.path(name))
	if os.IsNotExist(err) {
		return nil, nil
	}
	return b, err
}

func (f FileDocStore) Save(name string, data []byte) error {
	p := f.path(name)
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	return os.WriteFile(p, data, 0644)
}

func (e *Engine) docLoad(name string) ([]byte, error) {
	if e == nil || e.docs == nil {
		return FileDocStore{}.Load(name)
	}
	return e.docs.Load(name)
}

func (e *Engine) docSave(name string, data []byte) error {
	if e == nil || e.docs == nil {
		return FileDocStore{}.Save(name, data)
	}
	// Penyimpanan yang dapat mengembalikan versi hasil tulisan sendiri (SaveVersioned) dipakai agar versi tulisan
	// instance lain yang masuk di antara Save dan Versions tidak ikut tercatat "sudah dilihat".
	if sv, ok := e.docs.(interface {
		SaveVersioned(name string, data []byte) (int64, error)
	}); ok {
		v, err := sv.SaveVersioned(name, data)
		if err != nil {
			return err
		}
		e.docSync.mu.Lock()
		if e.docSync.seen == nil {
			e.docSync.seen = map[string]int64{}
		}
		e.docSync.seen[name] = v
		e.docSync.mu.Unlock()
		return nil
	}
	if err := e.docs.Save(name, data); err != nil {
		return err
	}
	e.noteOwnWrite(name)
	return nil
}

// VersionedDocStore: DocStore yang dapat melaporkan versi tiap dokumen, agar instance lain mengetahui adanya
// perubahan (lihat Engine.StartDocSync).
type VersionedDocStore interface {
	DocStore
	// Versions mengembalikan versi terkini semua dokumen (nama -> versi).
	Versions() (map[string]int64, error)
}

type docSyncState struct {
	mu   sync.Mutex
	seen map[string]int64
}

func (e *Engine) noteOwnWrite(name string) {
	vs, ok := e.docs.(VersionedDocStore)
	if !ok {
		return
	}
	cur, err := vs.Versions()
	if err != nil {
		return
	}
	e.docSync.mu.Lock()
	if e.docSync.seen == nil {
		e.docSync.seen = map[string]int64{}
	}
	e.docSync.seen[name] = cur[name]
	e.docSync.mu.Unlock()
}

// reloadDoc memuat ulang manajer pemilik dokumen dari penyimpanan. Definisi modul tidak dimuat ulang (butuh restart).
func (e *Engine) reloadDoc(name string) {
	switch name {
	case DocUsers:
		if e.UserManager != nil {
			e.UserManager.loadUsers()
		}
	case DocPrivileges:
		e.LoadDynamicRoles()
	case DocMenus:
		e.LoadDynamicMenus()
	case DocPages:
		e.LoadDynamicPages()
	case DocTenants:
		if e.TenantManager != nil {
			e.TenantManager.load()
		}
	case DocEmailTemplates:
		if e.EmailTemplates != nil {
			e.EmailTemplates.loadTemplates()
		}
	case DocAPITokens:
		e.LoadDynamicTokens()
	case DocSettings:
		e.LoadDynamicSettings()
	}
}

// SyncDocsOnce membandingkan versi dokumen di penyimpanan dengan yang terakhir diketahui instance ini dan memuat
// ulang dokumen yang diubah instance lain. Mengembalikan nama dokumen yang dimuat ulang.
func (e *Engine) SyncDocsOnce() ([]string, error) {
	vs, ok := e.docs.(VersionedDocStore)
	if !ok {
		return nil, nil
	}
	cur, err := vs.Versions()
	if err != nil {
		return nil, err
	}
	e.docSync.mu.Lock()
	if e.docSync.seen == nil {
		e.docSync.seen = map[string]int64{}
	}
	var changed []string
	for _, n := range DocNames {
		v, present := cur[n]
		if !present {
			continue
		}
		if old, known := e.docSync.seen[n]; known && old != v {
			changed = append(changed, n)
		}
		e.docSync.seen[n] = v
	}
	e.docSync.mu.Unlock()
	for _, n := range changed {
		e.reloadDoc(n)
	}
	return changed, nil
}

// StartDocSync memeriksa perubahan dokumen oleh instance lain tiap interval sampai ctx selesai. Hanya berguna bila
// beberapa instance berbagi satu penyimpanan; satu instance tidak perlu menyalakannya.
func (e *Engine) StartDocSync(ctx context.Context, interval time.Duration) {
	if interval < time.Second {
		interval = time.Second
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if ch, err := e.SyncDocsOnce(); err != nil {
					log.Printf("doc sync: %v", err)
				} else if len(ch) > 0 {
					log.Printf("doc sync: dimuat ulang dari penyimpanan bersama: %v", ch)
				}
			}
		}
	}()
}

// SetDocStore memindahkan penyimpanan konfigurasi ke ds. Dokumen yang belum ada di ds disalin sekali dari
// penyimpanan sebelumnya (mis. berkas JSON lama); setelah itu penyimpanan lama tidak dipakai lagi. Seluruh
// manajer dimuat ulang dari ds.
func (e *Engine) SetDocStore(ds DocStore) error {
	if ds == nil {
		return nil
	}
	old := e.docs
	if old == nil {
		old = FileDocStore{}
	}
	for _, n := range DocNames {
		cur, err := ds.Load(n)
		if err != nil {
			return err
		}
		if len(cur) > 0 {
			continue
		}
		prev, err := old.Load(n)
		if err != nil {
			return err
		}
		if len(prev) > 0 {
			if err := ds.Save(n, prev); err != nil {
				return err
			}
		}
	}
	e.docs = ds
	if vs, ok := ds.(VersionedDocStore); ok {
		if cur, err := vs.Versions(); err == nil {
			e.docSync.mu.Lock()
			e.docSync.seen = cur
			e.docSync.mu.Unlock()
		}
	}
	if e.UserManager != nil {
		e.UserManager.loadUsers()
	}
	if e.EmailTemplates != nil {
		e.EmailTemplates.loadTemplates()
	}
	if e.TenantManager != nil {
		e.TenantManager.load()
	}
	e.LoadDynamicRoles()
	e.LoadDynamicMenus()
	e.LoadDynamicPages()
	e.LoadDynamicSettings()
	e.LoadDynamicTokens()
	return nil
}
