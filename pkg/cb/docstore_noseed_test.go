package cb

import (
	"errors"
	"strings"
	"testing"
)

// failDocStore membungkus memDocStore dan dapat dipaksa galat saat Load, atau mengembalikan JSON rusak.
type failDocStore struct {
	memDocStore
	failLoad bool
	corrupt  bool
	saves    int
}

func (f *failDocStore) Load(n string) ([]byte, error) {
	f.mu.Lock()
	fl, co := f.failLoad, f.corrupt
	f.mu.Unlock()
	if fl {
		return nil, errors.New("koneksi putus")
	}
	if co {
		return []byte("{bukan json"), nil
	}
	return f.memDocStore.Load(n)
}

func (f *failDocStore) Save(n string, d []byte) error {
	f.mu.Lock()
	f.saves++
	f.mu.Unlock()
	return f.memDocStore.Save(n, d)
}

// Galat Load atau JSON rusak saat muat ulang tidak boleh menyemai/menimpa dokumen asli; memori tetap utuh.
func TestReloadDoesNotSeedOverRealDataOnErrorOrCorruption(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	fs := &failDocStore{}
	if err := e.SetDocStore(fs); err != nil {
		t.Fatal(err)
	}
	if _, err := e.UserManager.CreateUser("Asli", "asli@uji.id", "Sandi#12345", "Superadmin", "1", "", "Active"); err != nil {
		t.Fatal(err)
	}
	before, _ := fs.memDocStore.Load(DocUsers)
	tenantsBefore, _ := fs.memDocStore.Load(DocTenants)

	for _, mode := range []string{"galat", "rusak"} {
		fs.mu.Lock()
		fs.failLoad, fs.corrupt, fs.saves = mode == "galat", mode == "rusak", 0
		fs.mu.Unlock()
		for _, n := range DocNames {
			e.reloadDoc(n)
		}
		fs.mu.Lock()
		saves := fs.saves
		fs.failLoad, fs.corrupt = false, false
		fs.mu.Unlock()
		if saves != 0 {
			t.Errorf("%s: reload memicu %d penulisan; tidak boleh menyimpan apa pun", mode, saves)
		}
		after, _ := fs.memDocStore.Load(DocUsers)
		if string(after) != string(before) {
			t.Errorf("%s: dokumen users tertimpa", mode)
		}
		tAfter, _ := fs.memDocStore.Load(DocTenants)
		if string(tAfter) != string(tenantsBefore) {
			t.Errorf("%s: dokumen tenants tertimpa", mode)
		}
		if e.UserManager.FindByEmail("asli@uji.id") == nil {
			t.Errorf("%s: pengguna asli hilang dari memori", mode)
		}
		if e.UserManager.FindByEmail("manager@tgo.io") != nil && !hasEmail(before, "manager@tgo.io") {
			t.Errorf("%s: akun seed muncul di memori", mode)
		}
	}
}

func hasEmail(doc []byte, email string) bool { return strings.Contains(string(doc), email) }

// Dokumen benar-benar absen: seed dibuat tepat sekali.
func TestAbsentDocumentIsSeededOnce(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	fs := &failDocStore{}
	if err := e.SetDocStore(fs); err != nil {
		t.Fatal(err)
	}
	d, _ := fs.memDocStore.Load(DocTenants)
	if len(d) == 0 {
		t.Fatal("tenants harus disemai bila dokumen belum ada")
	}
}

// saveVerStore menaikkan versi dokumen oleh "instance lain" tepat setelah Save sendiri; SaveVersioned mengembalikan
// versi tulisan sendiri sehingga tulisan instance lain tetap terdeteksi saat SyncDocsOnce.
type saveVerStore struct {
	verDocStore
	other int
}

func (s *saveVerStore) SaveVersioned(n string, d []byte) (int64, error) {
	if err := s.verDocStore.Save(n, d); err != nil {
		return 0, err
	}
	s.mu.Lock()
	own := s.ver[n]
	s.ver[n]++ // tulisan instance lain menyelip sebelum Versions() dipanggil
	s.other++
	s.mu.Unlock()
	return own, nil
}

func TestSaveVersionedDoesNotSwallowOtherInstanceWrite(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	st := &saveVerStore{}
	if err := e.SetDocStore(st); err != nil {
		t.Fatal(err)
	}
	if _, err := e.UserManager.CreateUser("A", "a@uji.id", "Sandi#12345", "Superadmin", "1", "", "Active"); err != nil {
		t.Fatal(err)
	}
	changed, err := e.SyncDocsOnce()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range changed {
		if n == DocUsers {
			found = true
		}
	}
	if !found {
		t.Errorf("tulisan instance lain pada users harus terdeteksi; changed=%v", changed)
	}
}
