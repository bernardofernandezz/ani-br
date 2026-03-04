package norm

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// NormalizeTitle normaliza um título para comparação e deduplicação.
// - converte para minúsculas
// - remove acentos
// - remove caracteres não alfanuméricos
func NormalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	s = strings.ToLower(s)
	// Decomposição canônica para separar acentos.
	t := norm.NFD.String(s)

	var b strings.Builder
	b.Grow(len(t))

	for _, r := range t {
		if unicode.Is(unicode.Mn, r) {
			// ignora marcas de acento.
			continue
		}
		if r > utf8.RuneSelf && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if r <= utf8.RuneSelf && !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			continue
		}
		b.WriteRune(r)
	}

	return b.String()
}

