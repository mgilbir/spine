// Package opc implements the Open Packaging Conventions (OPC) specification
// as defined in ECMA-376 Part 2.
package opc

import (
	"errors"
	"fmt"
)

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

// PartError represents an error associated with a specific part.
type PartError struct {
	PartName string
	Err      error
}

func (e *PartError) Error() string {
	return fmt.Sprintf("opc: part %q: %v", e.PartName, e.Err)
}

func (e *PartError) Unwrap() error {
	return e.Err
}

// RelationshipError represents an error associated with a relationship.
type RelationshipError struct {
	RelationshipID string
	Err            error
}

func (e *RelationshipError) Error() string {
	return fmt.Sprintf("opc: relationship %q: %v", e.RelationshipID, e.Err)
}

func (e *RelationshipError) Unwrap() error {
	return e.Err
}
