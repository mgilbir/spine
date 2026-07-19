// Package crypto provides the legacy password-obfuscation helpers that Office
// documents use for their non-cryptographic "protection" features (worksheet
// protection, workbook-structure protection, document edit enforcement).
//
// These are deliberately weak, fully documented obfuscation schemes, not
// encryption. The values they produce guard nothing: any tool can clear the
// corresponding protection element without knowing the password. They exist
// only so that files this library writes interoperate with the same UI guards
// that Word and Excel present.
package crypto

// LegacyPasswordHash computes Office's legacy 16-bit password hash
// (ECMA-376 §18.3.1.75 / [MS-OFFCRYPTO] §2.3.7.1). Excel writes it into the
// sheetProtection/workbookProtection password attribute; older Word builds use
// the same algorithm for document protection.
//
// It is a simple, documented obfuscation and NOT a cryptographic hash: the
// 16-bit space is trivially brute-forced and collisions are common. Do not
// rely on it to protect confidential data.
func LegacyPasswordHash(password string) uint16 {
	if password == "" {
		return 0
	}
	var hash uint16
	for i := len(password) - 1; i >= 0; i-- {
		hash = ((hash >> 14) & 0x01) | ((hash << 1) & 0x7fff)
		hash ^= uint16(password[i])
	}
	hash = ((hash >> 14) & 0x01) | ((hash << 1) & 0x7fff)
	hash ^= uint16(len(password))
	hash ^= 0xCE4B
	return hash
}
