// Package opc implements the Open Packaging Conventions (OPC) specification
// as defined in ECMA-376 Part 2.
package opc

import "errors"

var (
	// ErrInvalidPartName indicates a part name that violates OPC naming rules.
	ErrInvalidPartName = errors.New("opc: invalid part name")

	// ErrPartNotFound indicates the requested part does not exist in the package.
	ErrPartNotFound = errors.New("opc: part not found")

	// ErrDuplicatePart indicates an attempt to create a part that already exists.
	ErrDuplicatePart = errors.New("opc: duplicate part")

	// ErrInvalidRelationship indicates a malformed relationship.
	ErrInvalidRelationship = errors.New("opc: invalid relationship")

	// ErrInvalidContentType indicates a malformed content type.
	ErrInvalidContentType = errors.New("opc: invalid content type")

	// ErrPackageClosed indicates an operation on a closed package.
	ErrPackageClosed = errors.New("opc: package is closed")

	// ErrCorruptedPackage indicates the package structure is invalid.
	ErrCorruptedPackage = errors.New("opc: corrupted package")
)
