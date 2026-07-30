package xlsx

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Sheet.Protection copies sixteen near-identical optional booleans out of
// <sheetProtection> into sixteen near-identical accessors, applying two
// different defaults depending on the operation. Two failure modes hide
// perfectly in that shape: an accessor returning its neighbour's field
// (formatColumns reading formatRows), and an operation wired to the wrong
// default so an *omitted* attribute reports backwards — which is the common
// case, since Excel omits the attribute precisely when the operation is locked.
//
// The matrix below pins both. Every case writes all sixteen attributes
// explicitly at their default value and flips exactly one, then asserts the
// whole sixteen-value vector: a cross-wiring moves the flip to another
// accessor and both are named. The all-absent case pins the defaults
// separately, and protectionAccessorsAreComplete derives the accessor list from
// the type so a seventeenth operation cannot join untested.

// protectionOp names one protection operation: the sheetProtection attribute,
// the accessor, and the value the accessor reports when the attribute is
// omitted entirely.
type protectionOp struct {
	attr          string
	method        string
	defaultLocked bool
	get           func(*SheetProtection) bool
}

func protectionOps() []protectionOp {
	return []protectionOp{
		// Format/insert/delete/sort/autoFilter/pivotTables default to LOCKED:
		// Excel omits the attribute when locked and writes "0" to unlock.
		{"formatCells", "FormatCells", true, (*SheetProtection).FormatCells},
		{"formatColumns", "FormatColumns", true, (*SheetProtection).FormatColumns},
		{"formatRows", "FormatRows", true, (*SheetProtection).FormatRows},
		{"insertColumns", "InsertColumns", true, (*SheetProtection).InsertColumns},
		{"insertRows", "InsertRows", true, (*SheetProtection).InsertRows},
		{"insertHyperlinks", "InsertHyperlinks", true, (*SheetProtection).InsertHyperlinks},
		{"deleteColumns", "DeleteColumns", true, (*SheetProtection).DeleteColumns},
		{"deleteRows", "DeleteRows", true, (*SheetProtection).DeleteRows},
		{"sort", "Sort", true, (*SheetProtection).Sort},
		{"autoFilter", "AutoFilter", true, (*SheetProtection).AutoFilter},
		{"pivotTables", "PivotTables", true, (*SheetProtection).PivotTables},
		// Objects/scenarios and the two selection flags default to UNLOCKED.
		{"objects", "Objects", false, (*SheetProtection).Objects},
		{"scenarios", "Scenarios", false, (*SheetProtection).Scenarios},
		{"selectLockedCells", "SelectLockedCells", false, (*SheetProtection).SelectLockedCells},
		{"selectUnlockedCells", "SelectUnlockedCells", false, (*SheetProtection).SelectUnlockedCells},
	}
}

// protectionFromAttrs builds a sheet whose <sheetProtection> carries exactly
// the given attributes and returns its parsed protection view.
func protectionFromAttrs(t *testing.T, attrs string) *SheetProtection {
	t.Helper()
	ws := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/><sheetProtection sheet="1"` + attrs + `/></worksheet>`
	var model oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(ws), &model); err != nil {
		t.Fatalf("unmarshal worksheet: %v", err)
	}
	s := &Sheet{wsModel: &model, wsParsed: true}
	p := s.Protection()
	if p == nil {
		t.Fatal("Protection() = nil for a sheet carrying <sheetProtection>")
	}
	return p
}

// boolAttrValue is the attribute text for a locked/unlocked state.
func boolAttrValue(locked bool) string {
	if locked {
		return "1"
	}
	return "0"
}

// A <sheetProtection> with no operation attributes reports each operation at
// its OOXML default: locked for the edit operations, unlocked for
// objects/scenarios and cell selection. Getting a default backwards inverts the
// meaning of a workbook that never wrote the attribute at all.
func TestSheetProtectionDefaultsWhenAttributesAbsent(t *testing.T) {
	p := protectionFromAttrs(t, "")
	if !p.Enabled() {
		t.Error("Enabled() = false for <sheetProtection sheet=\"1\">")
	}
	if p.HasPassword() {
		t.Error("HasPassword() = true with no password or hashValue")
	}
	for _, op := range protectionOps() {
		if got := op.get(p); got != op.defaultLocked {
			t.Errorf("%s() with %s absent = %v, want %v", op.method, op.attr, got, op.defaultLocked)
		}
	}
}

// Flipping one attribute moves exactly one accessor. Every other attribute is
// present at its default value, so an accessor reading a neighbour's field
// reports the flip on the wrong operation and both show up by name.
func TestSheetProtectionAccessorsAreIndependentlyWired(t *testing.T) {
	ops := protectionOps()
	for _, target := range ops {
		t.Run(target.method, func(t *testing.T) {
			var b strings.Builder
			for _, op := range ops {
				v := op.defaultLocked
				if op.attr == target.attr {
					v = !v
				}
				b.WriteString(` ` + op.attr + `="` + boolAttrValue(v) + `"`)
			}
			p := protectionFromAttrs(t, b.String())
			for _, op := range ops {
				want := op.defaultLocked
				if op.attr == target.attr {
					want = !want
				}
				if got := op.get(p); got != want {
					t.Errorf("with only %s flipped: %s() = %v, want %v",
						target.attr, op.method, got, want)
				}
			}
		})
	}
}

// A password guard is reported for both the legacy 16-bit hash and the modern
// hashValue, and never exposes the password itself.
func TestSheetProtectionHasPasswordSources(t *testing.T) {
	if p := protectionFromAttrs(t, ` password="83AF"`); !p.HasPassword() {
		t.Error("HasPassword() = false for a legacy password hash")
	}
	if p := protectionFromAttrs(t, ` hashValue="abc=" saltValue="def=" spinCount="100000"`); !p.HasPassword() {
		t.Error("HasPassword() = false for an agile hashValue")
	}
}

// Enabled reports the sheet attribute, not merely the element's presence: a
// <sheetProtection sheet="0"> is a stored-but-off protection block.
func TestSheetProtectionEnabledFollowsSheetAttribute(t *testing.T) {
	ws := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/><sheetProtection sheet="0" objects="1"/></worksheet>`
	var model oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(ws), &model); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	s := &Sheet{wsModel: &model, wsParsed: true}
	p := s.Protection()
	if p == nil {
		t.Fatal("Protection() = nil for a sheet carrying <sheetProtection>")
	}
	if p.Enabled() {
		t.Error("Enabled() = true for sheet=\"0\"")
	}
	if !p.Objects() {
		t.Error("Objects() = false for objects=\"1\"")
	}
}

// A sheet with no <sheetProtection> has no protection view at all, which is how
// callers distinguish "unprotected" from "protected with everything allowed".
func TestSheetProtectionNilWithoutElement(t *testing.T) {
	ws := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData/></worksheet>`
	var model oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(ws), &model); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	s := &Sheet{wsModel: &model, wsParsed: true}
	if p := s.Protection(); p != nil {
		t.Errorf("Protection() = %+v for a sheet with no <sheetProtection>, want nil", p)
	}
}

// Every bool accessor on SheetProtection must appear in the matrix. Deriving
// the list from the type means a new operation accessor fails here until it is
// pinned, instead of joining the untested set.
func TestSheetProtectionAccessorMatrixIsComplete(t *testing.T) {
	// Enabled and HasPassword are state accessors, not operations; they have
	// their own tests above.
	nonOperation := map[string]bool{"Enabled": true, "HasPassword": true}

	covered := map[string]bool{}
	for _, op := range protectionOps() {
		covered[op.method] = true
	}

	typ := reflect.TypeOf(&SheetProtection{})
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.Bool {
			continue
		}
		if nonOperation[m.Name] || covered[m.Name] {
			continue
		}
		t.Errorf("(*SheetProtection).%s is not pinned by the protection matrix", m.Name)
	}
}
