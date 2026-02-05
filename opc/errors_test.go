package opc

import (
	"errors"
	"testing"
)

func TestPartError(t *testing.T) {
	baseErr := errors.New("base error")
	err := &PartError{
		PartName: "/test/document.xml",
		Err:      baseErr,
	}

	// Test Error() method
	errStr := err.Error()
	if errStr != `opc: part "/test/document.xml": base error` {
		t.Errorf("PartError.Error() = %q, unexpected format", errStr)
	}

	// Test Unwrap()
	if !errors.Is(err, baseErr) {
		t.Error("PartError should unwrap to base error")
	}
}

func TestRelationshipError(t *testing.T) {
	baseErr := errors.New("base error")
	err := &RelationshipError{
		RelationshipID: "rId1",
		Err:            baseErr,
	}

	// Test Error() method
	errStr := err.Error()
	if errStr != `opc: relationship "rId1": base error` {
		t.Errorf("RelationshipError.Error() = %q, unexpected format", errStr)
	}

	// Test Unwrap()
	if !errors.Is(err, baseErr) {
		t.Error("RelationshipError should unwrap to base error")
	}
}

func TestErrorVariables(t *testing.T) {
	// Verify error variables are distinct
	errs := []error{
		ErrInvalidPartName,
		ErrPartNotFound,
		ErrDuplicatePart,
		ErrInvalidRelationship,
		ErrInvalidContentType,
		ErrPackageClosed,
		ErrCorruptedPackage,
	}

	for i, err1 := range errs {
		if err1 == nil {
			t.Errorf("Error variable at index %d is nil", i)
		}
		for j, err2 := range errs {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("Errors at index %d and %d should be distinct", i, j)
			}
		}
	}
}

func TestPartError_Unwrap(t *testing.T) {
	innerErr := &PartError{
		PartName: "/inner.xml",
		Err:      ErrInvalidPartName,
	}
	outerErr := &PartError{
		PartName: "/outer.xml",
		Err:      innerErr,
	}

	// Should be able to unwrap to inner error
	if !errors.Is(outerErr, ErrInvalidPartName) {
		t.Error("Nested PartError should unwrap to ErrInvalidPartName")
	}
}
