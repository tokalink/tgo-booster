package cb

import (
	"os"
	"strings"
	"sync"
	"testing"
)

type memDocStore struct {
	mu   sync.Mutex
	docs map[string][]byte
}

func (m *memDocStore) Load(n string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.docs[n], nil
}
func (m *memDocStore) Save(n string, d []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.docs == nil {
		m.docs = map[string][]byte{}
	}
	m.docs[n] = append([]byte(nil), d...)
	return nil
}

// SetDocStore menyalin dokumen berkas yang ada ke penyimpanan baru sekali, lalu semua simpan berikutnya
// masuk ke penyimpanan baru (bukan berkas), dan dokumen yang sudah ada di penyimpanan baru menang atas berkas.
func TestSetDocStoreImportsOnceAndRedirectsWrites(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	usersFile, _ := os.ReadFile("data/users.json")
	if len(usersFile) == 0 {
		t.Fatal("berkas users.json awal tidak ada")
	}
	mem := &memDocStore{}
	if err := e.SetDocStore(mem); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{DocUsers, DocPrivileges, DocMenus, DocPages, DocTenants, DocEmailTemplates, DocSettings} {
		if d, _ := mem.Load(n); len(d) == 0 {
			t.Errorf("dokumen %s tidak diimpor ke penyimpanan baru", n)
		}
	}
	// Tulis setelah pindah: masuk ke mem, berkas tidak berubah.
	if _, err := e.UserManager.CreateUser("Baru", "baru@uji.id", "Sandi#12345", "Superadmin", "1", "", "Active"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile("data/users.json")
	if string(after) != string(usersFile) {
		t.Error("users.json berubah setelah pindah ke DocStore")
	}
	d, _ := mem.Load(DocUsers)
	if !strings.Contains(string(d), "baru@uji.id") {
		t.Error("pengguna baru tidak tersimpan di DocStore")
	}
	// Instance baru memakai penyimpanan yang sama tanpa berkas: pengguna ikut termuat.
	t.Chdir(t.TempDir())
	e2 := NewEngine("Uji")
	if err := e2.SetDocStore(mem); err != nil {
		t.Fatal(err)
	}
	if e2.UserManager.FindByEmail("baru@uji.id") == nil {
		t.Error("instance kedua tidak memuat pengguna dari DocStore")
	}
}

type verDocStore struct {
	memDocStore
	ver map[string]int64
}

func (v *verDocStore) Save(n string, d []byte) error {
	if err := v.memDocStore.Save(n, d); err != nil {
		return err
	}
	v.mu.Lock()
	if v.ver == nil {
		v.ver = map[string]int64{}
	}
	v.ver[n]++
	v.mu.Unlock()
	return nil
}
func (v *verDocStore) Versions() (map[string]int64, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := map[string]int64{}
	for k, x := range v.ver {
		out[k] = x
	}
	return out, nil
}

// Dua instance berbagi satu penyimpanan: perubahan di A terlihat di B setelah SyncDocsOnce, dan tulisan B sendiri
// tidak memicu muat ulang di B.
func TestSyncDocsOnceReloadsChangesFromOtherInstance(t *testing.T) {
	store := &verDocStore{}
	t.Chdir(t.TempDir())
	a := NewEngine("A")
	if err := a.SetDocStore(store); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	b := NewEngine("B")
	if err := b.SetDocStore(store); err != nil {
		t.Fatal(err)
	}
	if ch, _ := b.SyncDocsOnce(); len(ch) != 0 {
		t.Fatalf("tanpa perubahan tidak boleh memuat ulang: %v", ch)
	}
	if _, err := a.UserManager.CreateUser("Dari A", "dari-a@uji.id", "Sandi#12345", "Superadmin", "1", "", "Active"); err != nil {
		t.Fatal(err)
	}
	if b.UserManager.FindByEmail("dari-a@uji.id") != nil {
		t.Fatal("B belum boleh tahu sebelum sinkron")
	}
	ch, err := b.SyncDocsOnce()
	if err != nil || len(ch) != 1 || ch[0] != DocUsers {
		t.Fatalf("sinkron: %v %v", ch, err)
	}
	if b.UserManager.FindByEmail("dari-a@uji.id") == nil {
		t.Error("B tidak memuat pengguna baru dari A")
	}
	// tulisan B sendiri tidak memicu muat ulang
	if _, err := b.UserManager.CreateUser("Dari B", "dari-b@uji.id", "Sandi#12345", "Superadmin", "1", "", "Active"); err != nil {
		t.Fatal(err)
	}
	if ch, _ := b.SyncDocsOnce(); len(ch) != 0 {
		t.Errorf("tulisan sendiri memicu muat ulang: %v", ch)
	}
}
