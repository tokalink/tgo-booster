package cb

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

var (
	constraintMu       sync.RWMutex
	constraintMessages = map[string]string{}

	reConstraint = regexp.MustCompile(`violates (check|unique|foreign key|not-null) constraint "([^"]+)"`)
	reNotNullCol = regexp.MustCompile(`null value in column "([^"]+)"`)
	reInvalidIn  = regexp.MustCompile(`invalid input syntax for type (\w+)`)
)

// RegisterConstraintMessages mendaftarkan pesan ramah (nama constraint/index database -> kalimat untuk pengguna).
func RegisterConstraintMessages(m map[string]string) {
	constraintMu.Lock()
	defer constraintMu.Unlock()
	for k, v := range m {
		constraintMessages[k] = v
	}
}

// FriendlyDBError menerjemahkan galat database (tanpa bergantung pada driver) menjadi kalimat yang bisa dipahami pengguna.
// Mengembalikan "" bila galat bukan pelanggaran aturan data yang dikenali.
func FriendlyDBError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if m := reConstraint.FindStringSubmatch(s); m != nil {
		constraintMu.RLock()
		msg, ok := constraintMessages[m[2]]
		constraintMu.RUnlock()
		if ok {
			return msg
		}
		switch m[1] {
		case "unique":
			return "Data yang sama sudah ada (nilai harus unik)."
		case "foreign key":
			return "Data ini masih dipakai oleh data lain, atau merujuk data yang tidak ada."
		case "not-null":
			return "Ada isian wajib yang masih kosong."
		default:
			return "Isian tidak memenuhi aturan data (" + m[2] + ")."
		}
	}
	if m := reNotNullCol.FindStringSubmatch(s); m != nil {
		return fmt.Sprintf("Isian %q wajib diisi.", m[1])
	}
	if m := reInvalidIn.FindStringSubmatch(s); m != nil {
		return "Format isian tidak valid (diharapkan " + strings.ToLower(m[1]) + ")."
	}
	return ""
}

// writeFormError menampilkan galat kirim-form sebagai halaman layout dengan tombol "Kembali" (history.back)
// sehingga isian yang sudah diketik tidak hilang. Permintaan JSON mendapat teks biasa.
func (c *Controller) writeFormError(w http.ResponseWriter, r *http.Request, status int, title, msg string) {
	if c.Engine == nil || strings.Contains(r.Header.Get("Accept"), "application/json") {
		http.Error(w, msg, status)
		return
	}
	body := `<div class="panel" style="max-width:560px;margin:24px auto;">
  <header><h2>` + html.EscapeString(title) + `</h2></header>
  <div class="body" style="padding:16px 20px;">
    <p style="margin:0 0 16px;color:var(--neg-fg);">` + html.EscapeString(msg) + `</p>
    <a href="javascript:history.back()" class="btn btn-primary btn-sm">Kembali dan perbaiki isian</a>
  </div>
</div>`
	w.WriteHeader(status)
	c.Engine.RenderLayout(w, r, title, template.HTML(body))
}
