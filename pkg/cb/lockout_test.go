package cb

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuthenticate_PerAccountLockAcrossIPsAndIdentifiers(t *testing.T) {
	e := newAuthEngine(t)
	acc, err := e.UserManager.CreateUser("Budi", "budi@uji.id", "Sandi#12345", "Operations Manager", "2", "", "Active")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.UserManager.SetUsername(acc.ID, "budi"); err != nil {
		t.Fatal(err)
	}
	// maxAttempts=2 -> jatah per akun = 8. Variasi IP dan penulisan (email/username) berbagi satu jatah.
	ids := []string{"budi@uji.id", "BUDI", "budi", "Budi@Uji.id"}
	for i := 0; i < 8; i++ {
		ip := "9.9.9." + string(rune('0'+i))
		if _, err := e.UserManager.Authenticate(ids[i%len(ids)], "salah", ip, 2, 15); err == nil {
			t.Fatal("kata sandi salah harus ditolak")
		}
	}
	_, err = e.UserManager.Authenticate("budi@uji.id", "Sandi#12345", "1.1.1.1", 2, 15)
	if err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("akun harus terkunci walau IP baru dan sandi benar: %v", err)
	}
}

func TestAuthenticate_SuspendedNotRevealedWithoutPassword(t *testing.T) {
	e := newAuthEngine(t)
	acc, err := e.UserManager.CreateUser("Sus", "sus@uji.id", "Sandi#12345", "Operations Manager", "2", "", "Suspended")
	if err != nil {
		t.Fatal(err)
	}
	_ = acc
	_, err = e.UserManager.Authenticate("sus@uji.id", "salah", "1.1.1.1", 5, 15)
	if err == nil || !strings.Contains(err.Error(), "Invalid username/email or password") {
		t.Fatalf("akun suspended + sandi salah harus pesan generik: %v", err)
	}
	_, err = e.UserManager.Authenticate("sus@uji.id", "Sandi#12345", "1.1.1.1", 5, 15)
	if err == nil || !strings.Contains(err.Error(), "suspended") {
		t.Fatalf("sandi benar + suspended: %v", err)
	}
}

func TestTrackersArePruned(t *testing.T) {
	e := newAuthEngine(t)
	um := e.UserManager
	for i := 0; i < 3000; i++ {
		um.recordFailure(fmt.Sprintf("k%d@8.8.8.8", i), 5, 15)
	}
	um.trackerMu.Lock()
	n := len(um.failedAttempts)
	for k := range um.lastFail { // paksa semua catatan kedaluwarsa
		um.lastFail[k] = um.lastFail[k].Add(-2 * time.Hour)
	}
	um.trackerMu.Unlock()
	um.recordFailure("baru@8.8.8.8", 5, 15)
	um.trackerMu.Lock()
	after := len(um.failedAttempts)
	um.trackerMu.Unlock()
	if n < 3000 || after >= 100 {
		t.Errorf("peta tidak dibersihkan: sebelum %d sesudah %d", n, after)
	}
}

type errSF struct{}

func (errSF) Required(string) bool                { return false }
func (errSF) RequiredErr(string) (bool, error)    { return false, errors.New("db mati") }
func (errSF) Verify(string, string) bool          { return true }
func (errSF) ChallengeHTML(a, e, m string) string { return "" }

func TestBeginLogin_FailsClosedWhenSecondFactorLookupErrors(t *testing.T) {
	e := newAuthEngine(t)
	e.Auth.SecondFactor = errSF{}
	rec := httptest.NewRecorder()
	needs := e.Auth.BeginLogin(rec, &User{ID: "1", Name: "A", Email: "a@uji.id"})
	if !needs {
		t.Error("galat pencarian 2FA harus gagal-tertutup")
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == SessionCookieName {
			t.Error("sesi tidak boleh dibuat saat galat 2FA")
		}
	}
}
