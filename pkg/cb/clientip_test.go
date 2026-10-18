package cb

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote, xff, xrip string
		trust                   bool
		hops                    int
		want                    string
	}{
		{"tanpa trust abaikan XFF", "10.0.0.1:5555", "1.2.3.4", "", false, 0, "10.0.0.1"},
		{"trust ambil kanan, bukan kiri palsu", "10.0.0.1:5555", "1.2.3.4, 9.9.9.9", "", true, 1, "9.9.9.9"},
		{"dua hop", "10.0.0.1:5555", "1.2.3.4, 9.9.9.9, 10.0.0.2", "", true, 2, "9.9.9.9"},
		{"entri bukan IP ditolak -> RemoteAddr", "10.0.0.1:5555", "bukan-ip", "", true, 1, "10.0.0.1"},
		{"hops melebihi panjang -> RemoteAddr", "10.0.0.1:5555", "9.9.9.9", "", true, 3, "10.0.0.1"},
		{"X-Real-IP valid sebagai cadangan", "10.0.0.1:5555", "", "8.8.8.8", true, 1, "8.8.8.8"},
		{"X-Real-IP palsu ditolak", "10.0.0.1:5555", "", "x", true, 1, "10.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if c.xrip != "" {
			r.Header.Set("X-Real-IP", c.xrip)
		}
		if got := ClientIP(r, c.trust, c.hops); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
