// This file implements Office's legacy password-obfuscation helper for the
// non-cryptographic "protection" UI features (worksheet protection,
// workbook-structure protection, document edit enforcement). See doc.go for the
// package overview and how these differ from the real encryption in agile.go /
// standard.go / rc4.go.

package crypto

import "unicode/utf16"

// LegacyPasswordHash computes Office's legacy 16-bit password hash
// (ECMA-376 §18.3.1.75 / [MS-OFFCRYPTO] §2.3.7.1). Excel writes it into the
// sheetProtection/workbookProtection password attribute; older Word builds use
// the same algorithm for document protection.
//
// The algorithm is defined over the password's *characters*, so it is computed
// here over UTF-16 code units — the form Office holds passwords in — and mixes
// in the character count, not a byte count. Iterating the UTF-8 bytes of a Go
// string instead would hash "café" as five units of length five and produce a
// value Excel never writes for that password.
//
// It is a simple, documented obfuscation and NOT a cryptographic hash: the
// 16-bit space is trivially brute-forced and collisions are common. Do not
// rely on it to protect confidential data.
func LegacyPasswordHash(password string) uint16 {
	if password == "" {
		return 0
	}
	chars := utf16.Encode([]rune(password))
	var hash uint16
	for i := len(chars) - 1; i >= 0; i-- {
		hash = ((hash >> 14) & 0x01) | ((hash << 1) & 0x7fff)
		hash ^= chars[i]
	}
	hash = ((hash >> 14) & 0x01) | ((hash << 1) & 0x7fff)
	hash ^= uint16(len(chars) & 0xffff)
	hash ^= 0xCE4B
	return hash
}
