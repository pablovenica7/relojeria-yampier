package domain

import (
	"regexp"
	"strings"
)

var (
	spacesRe = regexp.MustCompile(`\s+`)
	// Validación razonable de email: no pretende cubrir el RFC completo, solo
	// rechazar valores claramente inválidos (y sin espacios ni saltos de línea,
	// lo que además impide inyectar headers en emails).
	emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

// CollapseSpaces recorta y deja un solo espacio entre palabras.
func CollapseSpaces(s string) string {
	return spacesRe.ReplaceAllString(strings.TrimSpace(s), " ")
}

// DigitsOnly deja solo los dígitos ASCII.
func DigitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ValidEmail indica si el email tiene un formato razonable.
func ValidEmail(s string) bool { return emailRe.MatchString(s) }

// NormalizeEmail recorta y pasa a minúsculas.
func NormalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
