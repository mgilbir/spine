// Package spectest provides a test harness for validating XML examples
// extracted from ISO/IEC 29500-1:2012 against the spine library's Go types.
package spectest

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
)

// Example represents a single XML example extracted from the ISO spec.
type Example struct {
	ID             string  `json:"id"`
	Section        string  `json:"section"`
	ElementName    string  `json:"element_name"`
	Description    string  `json:"description"`
	Page           int     `json:"page"`
	Document       string  `json:"document"`
	RootElement    *string `json:"root_element"`
	NSPrefix       string  `json:"ns_prefix"`
	Classification string  `json:"classification"`
	XML            *string `json:"xml"`
	XMLStripped    *string `json:"xml_stripped"`
}

// ExampleFile represents a JSON file of extracted examples.
type ExampleFile struct {
	Documents []string  `json:"documents"`
	Format    string    `json:"format"`
	Examples  []Example `json:"examples"`
}

// LoadExamples reads a JSON test data file and returns the parsed examples.
func LoadExamples(t *testing.T, path string) []Example {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read examples file %s: %v", path, err)
	}

	var ef ExampleFile
	if err := json.Unmarshal(data, &ef); err != nil {
		t.Fatalf("Failed to parse examples file %s: %v", path, err)
	}

	return ef.Examples
}

// LogBreadcrumb logs the ISO spec reference for a test example.
func LogBreadcrumb(t *testing.T, ex Example) {
	t.Helper()
	t.Logf("%s, section %s, page %d: %s (%s)",
		ex.Document, ex.Section, ex.Page, ex.ElementName, ex.Description)
}

// WML namespace constants.
const (
	NsWML          = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	NsRelationship = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	NsDML          = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NsPML          = "http://schemas.openxmlformats.org/presentationml/2006/main"
	NsSML          = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	NsWPDrawing    = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	NsMath         = "http://schemas.openxmlformats.org/officeDocument/2006/math"
	NsVML          = "urn:schemas-microsoft-com:vml"
	NsOffice       = "urn:schemas-microsoft-com:office:office"
	NsXMLNS        = "http://www.w3.org/XML/1998/namespace"
	NsMC           = "http://schemas.openxmlformats.org/markup-compatibility/2006"
)

// WrapWML wraps an XML fragment with WML namespace declarations.
func WrapWML(xmlStr string) string {
	return `<w:wrapper` +
		` xmlns:w="` + NsWML + `"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:a="` + NsDML + `"` +
		` xmlns:wp="` + NsWPDrawing + `"` +
		` xmlns:m="` + NsMath + `"` +
		` xmlns:v="` + NsVML + `"` +
		` xmlns:o="` + NsOffice + `"` +
		` xmlns:mc="` + NsMC + `"` +
		` xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml"` +
		` xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"` +
		` xmlns:sl="http://schemas.openxmlformats.org/schemaLibrary/2006/main"` +
		`>` + xmlStr + `</w:wrapper>`
}

// WrapSML wraps an XML fragment with SML namespace declarations.
func WrapSML(xmlStr string) string {
	return `<wrapper` +
		` xmlns="` + NsSML + `"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:mc="` + NsMC + `"` +
		`>` + xmlStr + `</wrapper>`
}

// WrapPML wraps an XML fragment with PML namespace declarations.
func WrapPML(xmlStr string) string {
	return `<p:wrapper` +
		` xmlns:p="` + NsPML + `"` +
		` xmlns:a="` + NsDML + `"` +
		` xmlns:r="` + NsRelationship + `"` +
		` xmlns:mc="` + NsMC + `"` +
		`>` + xmlStr + `</p:wrapper>`
}

// UnmarshalFragment wraps an XML fragment, skips the wrapper element, and
// decodes the first child element into the target.
func UnmarshalFragment(wrapped []byte, target interface{}) error {
	dec := xml.NewDecoder(bytes.NewReader(wrapped))

	// Skip tokens until we find the wrapper start element
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("reading wrapper: %w", err)
		}
		if _, ok := tok.(xml.StartElement); ok {
			// Found wrapper start element, now decode first child into target
			return dec.Decode(target)
		}
	}
}

// TestUnmarshalExamples runs unmarshal tests for a set of examples using the given
// type registry and wrapper function.
func TestUnmarshalExamples(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string) {
	t.Helper()
	TestUnmarshalExamplesWithSkips(t, examples, typeMap, wrapFn, nil)
}

// TestUnmarshalExamplesWithSkips runs unmarshal tests with an optional out-of-scope skip map.
// Elements in outOfScope are skipped with a descriptive reason instead of "No Go type mapped".
func TestUnmarshalExamplesWithSkips(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string, outOfScope map[string]string) {
	t.Helper()

	for _, ex := range examples {
		if ex.Classification != "clean" && ex.Classification != "ellipsis_strippable" {
			continue
		}

		xmlStr := ex.XML
		if ex.Classification == "ellipsis_strippable" {
			xmlStr = ex.XMLStripped
		}
		if xmlStr == nil {
			continue
		}

		rootElem := ""
		if ex.RootElement != nil {
			rootElem = *ex.RootElement
		}

		t.Run(ex.ID, func(t *testing.T) {
			LogBreadcrumb(t, ex)

			if reason, skip := outOfScope[rootElem]; skip {
				t.Skipf("Out of scope: %s (%s)", rootElem, reason)
				return
			}

			typ, ok := typeMap[rootElem]
			if !ok {
				t.Skipf("No Go type mapped for element %q", rootElem)
				return
			}

			wrapped := wrapFn(*xmlStr)
			target := reflect.New(typ).Interface()

			if err := UnmarshalFragment([]byte(wrapped), target); err != nil {
				t.Errorf("Unmarshal failed: %v\nXML: %.200s", err, *xmlStr)
			}
		})
	}
}

// MarshalFunc marshals a Go value for round-trip testing. The rootElem parameter
// is the local name of the root element (e.g., "p", "document").
// Implementations can use Builder-based marshaling for types with custom MarshalToBuilder.
type MarshalFunc func(v interface{}, rootElem string) ([]byte, error)

// TestRoundTripExamples runs unmarshal-marshal-unmarshal round-trip tests
// using encoding/xml.Marshal.
func TestRoundTripExamples(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string) {
	t.Helper()
	testRoundTripExamples(t, examples, typeMap, wrapFn, nil, nil)
}

// TestRoundTripExamplesWithMarshal runs unmarshal-marshal-unmarshal round-trip tests
// using a custom marshal function (e.g., Builder-based marshaling for types with xml:"-" fields).
func TestRoundTripExamplesWithMarshal(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string, marshalFn MarshalFunc) {
	t.Helper()
	testRoundTripExamples(t, examples, typeMap, wrapFn, marshalFn, nil)
}

// TestRoundTripExamplesWithSkips runs round-trip tests with an out-of-scope skip map.
func TestRoundTripExamplesWithSkips(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string, marshalFn MarshalFunc, outOfScope map[string]string) {
	t.Helper()
	testRoundTripExamples(t, examples, typeMap, wrapFn, marshalFn, outOfScope)
}

func testRoundTripExamples(t *testing.T, examples []Example, typeMap map[string]reflect.Type, wrapFn func(string) string, marshalFn MarshalFunc, outOfScope map[string]string) {
	t.Helper()

	for _, ex := range examples {
		// Only round-trip clean examples (ellipsis-stripped are incomplete)
		if ex.Classification != "clean" {
			continue
		}
		if ex.XML == nil {
			continue
		}

		rootElem := ""
		if ex.RootElement != nil {
			rootElem = *ex.RootElement
		}

		t.Run(ex.ID, func(t *testing.T) {
			LogBreadcrumb(t, ex)

			if reason, skip := outOfScope[rootElem]; skip {
				t.Skipf("Out of scope: %s (%s)", rootElem, reason)
				return
			}

			typ, ok := typeMap[rootElem]
			if !ok {
				t.Skipf("No Go type mapped for element %q", rootElem)
				return
			}

			wrapped := wrapFn(*ex.XML)

			// First unmarshal
			target1 := reflect.New(typ).Interface()
			if err := UnmarshalFragment([]byte(wrapped), target1); err != nil {
				t.Skipf("Unmarshal failed (covered by unmarshal test): %v", err)
				return
			}

			// Marshal
			var marshaled []byte
			var err error
			if marshalFn != nil {
				marshaled, err = marshalFn(target1, rootElem)
			} else {
				marshaled, err = xml.Marshal(target1)
			}
			if err != nil {
				t.Errorf("Marshal failed: %v", err)
				return
			}

			// Re-wrap for second unmarshal (marshaled output may need namespace context)
			rewrapped := wrapFn(string(marshaled))

			// Second unmarshal
			target2 := reflect.New(typ).Interface()
			if err := UnmarshalFragment([]byte(rewrapped), target2); err != nil {
				t.Errorf("Second unmarshal failed: %v\nMarshaled: %.200s", err, string(marshaled))
				return
			}

			// Compare: clear round-trip preservation fields before comparing,
			// as they capture original file formatting (namespace URIs, attribute
			// order) that may differ between Strict/Transitional conformance
			// classes and are not part of the semantic content.
			rv1 := reflect.ValueOf(target1).Elem()
			rv2 := reflect.ValueOf(target2).Elem()
			clearRoundTripFields(rv1)
			clearRoundTripFields(rv2)
			if !reflect.DeepEqual(rv1.Interface(), rv2.Interface()) {
				t.Errorf("Round-trip mismatch:\n  Original XML: %.200s\n  Marshaled: %.200s", *ex.XML, string(marshaled))
			}
		})
	}
}

// clearRoundTripFields zeroes out fields that exist solely for byte-identical
// round-trip preservation (e.g., OriginalNSDecls, OriginalRootAttrs, XMLName).
// These fields capture the original file's namespace representation and attribute
// ordering, which may differ between Strict and Transitional conformance classes.
func clearRoundTripFields(v reflect.Value) {
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		switch f.Name {
		case "XMLName", "OriginalNSDecls", "OriginalRootAttrs":
			v.Field(i).Set(reflect.Zero(f.Type))
		}
	}
}

// TestWellFormedExamples tests that all clean examples are well-formed XML
// by parsing them with the standard library. This doesn't require type mapping.
func TestWellFormedExamples(t *testing.T, examples []Example, wrapFn func(string) string) {
	t.Helper()

	for _, ex := range examples {
		if ex.Classification != "clean" && ex.Classification != "ellipsis_strippable" {
			continue
		}

		xmlStr := ex.XML
		if ex.Classification == "ellipsis_strippable" {
			xmlStr = ex.XMLStripped
		}
		if xmlStr == nil {
			continue
		}

		t.Run(ex.ID, func(t *testing.T) {
			LogBreadcrumb(t, ex)

			wrapped := wrapFn(*xmlStr)

			decoder := xml.NewDecoder(bytes.NewReader([]byte(wrapped)))
			for {
				_, err := decoder.Token()
				if err != nil {
					if err == io.EOF {
						break
					}
					t.Errorf("XML parse error: %v\nXML: %.200s", err, *xmlStr)
					return
				}
			}
		})
	}
}
