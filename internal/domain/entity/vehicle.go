package entity

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizePlate returns a plate the way it's printed: uppercase, with no spaces, dashes or
// dots ("ab 123-cd" -> "AB123CD") -- same rule auth-service applies to saved vehicles.
func NormalizePlate(plate string) string {
	var builder strings.Builder
	for _, char := range plate {
		if unicode.IsSpace(char) || char == '-' || char == '.' {
			continue
		}
		builder.WriteRune(unicode.ToUpper(char))
	}
	return builder.String()
}

// CapitalizeWords uppercases the first letter of every word and leaves the rest as typed --
// "honda xr 250" -> "Honda Xr 250", while "BMW" stays "BMW". Extra spaces collapse.
func CapitalizeWords(text string) string {
	words := strings.Fields(text)
	for i, word := range words {
		first, size := utf8.DecodeRuneInString(word)
		words[i] = string(unicode.ToUpper(first)) + word[size:]
	}
	return strings.Join(words, " ")
}
