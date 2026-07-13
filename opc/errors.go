// Package opc implements the Open Packaging Conventions (OPC) specification
// as defined in ECMA-376 Part 2.
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
)
