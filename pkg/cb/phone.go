package cb

import (
	"errors"
	"strings"
)

// NormalizePhone menormalkan nomor WhatsApp dasar di Booster:
// Membuang spasi, tanda hubung, titik, kurung. Mengizinkan awalan '+';
// sisanya harus digit 8–15 karakter. Nomor kosong dibiarkan kosong (opsional).
// Selain itu mengembalikan galat "Nomor WhatsApp tidak valid".
func NormalizePhone(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	hasPlus := false
	if strings.HasPrefix(s, "+") {
		hasPlus = true
		s = s[1:]
	}

	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '-', '.', '(', ')', '–', '—':
			continue
		default:
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			} else {
				return "", errors.New("Nomor WhatsApp tidak valid")
			}
		}
	}

	digits := b.String()
	if len(digits) < 8 || len(digits) > 15 {
		return "", errors.New("Nomor WhatsApp tidak valid")
	}

	if hasPlus {
		return "+" + digits, nil
	}
	return digits, nil
}
