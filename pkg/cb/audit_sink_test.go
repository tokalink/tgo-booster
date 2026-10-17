package cb

import (
	"os"
	"strconv"
	"sync"
	"testing"
)

type memAuditSink struct {
	mu   sync.Mutex
	rows []AuditLogEntry // urut tertua dulu
}

func (m *memAuditSink) Append(e AuditLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = strconv.Itoa(len(m.rows) + 1)
	m.rows = append(m.rows, e)
	return nil
}
func (m *memAuditSink) Recent(limit int) ([]AuditLogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []AuditLogEntry
	for i := len(m.rows) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.rows[i])
	}
	return out, nil
}
func (m *memAuditSink) Find(id string) (*AuditLogEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rows {
		if r.ID == id {
			c := r
			return &c, nil
		}
	}
	return nil, nil
}
func (m *memAuditSink) Count() (int, error) { return len(m.rows), nil }

func TestAuditSinkImportsOnceAndRedirectsWrites(t *testing.T) {
	t.Chdir(t.TempDir())
	e := NewEngine("Uji")
	before, _ := os.ReadFile("data/logs.json")
	n := len(e.AuditLogger.GetAllLogs())
	if n == 0 {
		t.Fatal("log awal kosong")
	}
	sink := &memAuditSink{}
	if err := e.SetAuditSink(sink); err != nil {
		t.Fatal(err)
	}
	if c, _ := sink.Count(); c != n {
		t.Fatalf("impor awal %d, ingin %d", c, n)
	}
	e.AuditLogger.Log(nil, "uji_aksi", "Uji", "deskripsi")
	if got := e.AuditLogger.GetAllLogs(); got[0].Action != "UJI_AKSI" {
		t.Errorf("entri terbaru harus di depan: %+v", got[0])
	}
	if after, _ := os.ReadFile("data/logs.json"); string(after) != string(before) {
		t.Error("logs.json berubah setelah pindah ke sink")
	}
	// Pindah ulang tidak mengimpor dua kali.
	if err := e.SetAuditSink(sink); err != nil {
		t.Fatal(err)
	}
	if c, _ := sink.Count(); c != n+1 {
		t.Errorf("impor ganda: %d", c)
	}
}
