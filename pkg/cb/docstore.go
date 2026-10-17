package cb

import (
	"os"
	"path/filepath"
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
	return e.docs.Save(name, data)
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
