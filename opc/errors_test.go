package opc

import (
	"errors"
	"testing"
)

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
