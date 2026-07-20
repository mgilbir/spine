package opc

import "errors"

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
)
