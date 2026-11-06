package opc

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidPartName indicates a part name that violates OPC naming rules.
	ErrInvalidPartName = errors.New("opc: invalid part name")

	// ErrDuplicatePart indicates an attempt to create a part that already exists.
	ErrDuplicatePart = errors.New("opc: duplicate part")

	// ErrInvalidContentType indicates a malformed content type.
	ErrInvalidContentType = errors.New("opc: invalid content type")

	// ErrPackageClosed indicates an operation on a closed package.
	ErrPackageClosed = errors.New("opc: package is closed")

	// ErrCorruptedPackage indicates the package structure is invalid.
	ErrCorruptedPackage = errors.New("opc: corrupted package")

	// ErrStrictOOXML indicates a valid but ISO-Strict (ISO/IEC 29500 Strict)
	// OOXML package. Its parts use the purl.oclc.org/ooxml namespaces rather
	// than the transitional schemas.openxmlformats.org ones, which spine does
	// not yet read. It is a distinct, actionable signal that the file is a
	// genuine Office document in an unsupported dialect — not a corrupt or
	// non-Office file.
	ErrStrictOOXML = errors.New("opc: ISO-Strict OOXML packages are not supported")

	// ErrLegacyBinary indicates a compound (CFB) file that is not an encrypted
	// OOXML package: in practice a document in the binary format Office used
	// before 2007 (.doc, .xls or .ppt), which spine does not read. The message
	// names the application when the file's main stream identifies it. An
	// encrypted OOXML package is a compound file too, and reports ErrEncrypted
	// instead; no password opens a legacy file, so ErrLegacyBinary is returned
	// whether or not one was given.
	ErrLegacyBinary = errors.New("opc: legacy binary Office document, not an OOXML package")

	// ErrDecompressionLimit indicates a part that could not be read because it
	// declares, or decompresses to, more bytes than the per-part or
	// package-total decompression limit allows (see WithMaxDecompressedPartSize
	// and WithMaxDecompressedPackageSize). The part is present in the package
	// but unreadable under the configured limits; an open that meets one fails
	// rather than treating the part as absent, and the error wraps this
	// sentinel so a caller can tell the case apart and retry with a higher
	// limit.
	ErrDecompressionLimit = errors.New("opc: decompression limit exceeded")
)

// limitError is a decompression-limit violation. It keeps the specific,
// actionable message (which part, which limit, which option lifts it) as its
// text while matching ErrDecompressionLimit under errors.Is.
type limitError struct{ msg string }

func (e *limitError) Error() string { return e.msg }

func (e *limitError) Is(target error) bool { return target == ErrDecompressionLimit }

// limitErrorf formats a decompression-limit violation.
func limitErrorf(format string, args ...any) error {
	return &limitError{msg: fmt.Sprintf(format, args...)}
}
