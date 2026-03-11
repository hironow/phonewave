package platform

import "strings"

// SanitizeUTF8 replaces invalid UTF-8 bytes with the Unicode replacement character.
// Use this before passing strings from external sources (file system events,
// error messages) to OTel span attributes, which require valid UTF-8.
func SanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}
