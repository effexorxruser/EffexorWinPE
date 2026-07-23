package i18n

import (
	"unicode/utf16"
)

// EncodeUTF16 returns a null-terminated UTF-16 code unit slice for Win32 APIs.
func EncodeUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s + "\x00"))
}

// DecodeUTF16 decodes a null-terminated UTF-16 slice to a Go string.
func DecodeUTF16(u []uint16) string {
	if len(u) == 0 {
		return ""
	}
	if u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}
