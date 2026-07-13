package xml

import "testing"

// A source whose root element is namespace-prefixed (e.g. <x:workbook
// xmlns:x="…spreadsheetml…">) must round-trip with a prefixed open tag: the
// prefix binding has to be resolved from the preserved declarations before
// the tag name is written, or the open tag comes out unprefixed while
// children and the close tag resolve to the prefixed form — malformed XML
// (Common Crawl corpus, 16 xlsx files).
func TestStartElementWithRootAttrs_PrefixedRoot(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	b.StartElementWithRootAttrs(NSSpreadsheetML, "workbook", []RootAttr{
		{IsNS: true, Prefix: "r", Value: NSOfficeDocumentRels},
		{IsNS: true, Prefix: "x", Value: NSSpreadsheetML},
	})
	b.StartElement(NSSpreadsheetML, "sheets")
	b.EndElement(NSSpreadsheetML, "sheets")
	b.EndElement(NSSpreadsheetML, "workbook")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	want := `<x:workbook xmlns:r="` + NSOfficeDocumentRels + `" xmlns:x="` + NSSpreadsheetML + `"><x:sheets></x:sheets></x:workbook>`
	if got := b.String(); got != want {
		t.Errorf("prefixed root output:\n got %s\nwant %s", got, want)
	}
}

// A default (xmlns=) declaration for the root's namespace keeps element names
// unprefixed, even when a prefixed alias for the same URI is also declared
// (the alias serves attributes only).
func TestStartElementWithRootAttrs_DefaultDeclWins(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	b.StartElementWithRootAttrs(NSSpreadsheetML, "workbook", []RootAttr{
		{IsNS: true, Prefix: "", Value: NSSpreadsheetML},
		{IsNS: true, Prefix: "x", Value: NSSpreadsheetML},
	})
	b.EndElement(NSSpreadsheetML, "workbook")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	want := `<workbook xmlns="` + NSSpreadsheetML + `" xmlns:x="` + NSSpreadsheetML + `"></workbook>`
	if got := b.String(); got != want {
		t.Errorf("default-decl output:\n got %s\nwant %s", got, want)
	}
}

// The default declaration wins regardless of declaration order.
func TestStartElementWithRootAttrs_DefaultDeclWinsAfterPrefixed(t *testing.T) {
	b := NewSpreadsheetMLBuilder()
	b.StartElementWithRootAttrs(NSSpreadsheetML, "workbook", []RootAttr{
		{IsNS: true, Prefix: "x", Value: NSSpreadsheetML},
		{IsNS: true, Prefix: "", Value: NSSpreadsheetML},
	})
	b.EndElement(NSSpreadsheetML, "workbook")
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	want := `<workbook xmlns:x="` + NSSpreadsheetML + `" xmlns="` + NSSpreadsheetML + `"></workbook>`
	if got := b.String(); got != want {
		t.Errorf("default-decl-last output:\n got %s\nwant %s", got, want)
	}
}
