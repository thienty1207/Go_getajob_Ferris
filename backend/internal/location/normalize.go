package location

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const maxLocationTextRunes = 512

// Normalize creates the stable lookup key used by crawler aliases and client
// location resolution. It intentionally does not map a value to a city; that
// ownership belongs to the canonical location registry.
func Normalize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || utf8.RuneCountInString(value) > maxLocationTextRunes {
		return ""
	}

	decomposed := norm.NFD.String(strings.NewReplacer("Đ", "D", "đ", "d").Replace(strings.ToLower(value)))
	var builder strings.Builder
	previousSpace := true
	for _, character := range decomposed {
		if unicode.Is(unicode.Mn, character) {
			continue
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			previousSpace = false
			continue
		}
		if !previousSpace {
			builder.WriteByte(' ')
			previousSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}
