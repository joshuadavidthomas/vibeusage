package strutil

import "strings"

// TitleCase capitalizes the first letter of a string.
// For multi-word strings, it capitalizes the first letter of each word.
// This replaces the deprecated strings.Title for our ASCII-only provider names.
func TitleCase(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
