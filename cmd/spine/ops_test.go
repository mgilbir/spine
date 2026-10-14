package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// opsDoc is a small builder for operations documents in tests.
func opsDoc(t *testing.T, ops ...map[string]any) string {
	t.Helper()
	doc := map[string]any{"version": 1, "ops": ops}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func op(name string, fields ...any) map[string]any {
	m := map[string]any{"op": name}
	for i := 0; i+1 < len(fields); i += 2 {
		m[fields[i].(string)] = fields[i+1]
	}
	return m
}

// writeOpsFile writes an operations document to a temporary file.
func writeOpsFile(t *testing.T, doc string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "ops.json")
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// errJSON is the --json error envelope; op_index/op are present only for
// per-operation failures.
type errJSON struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	OpIndex *int   `json:"op_index"`
	Op      string `json:"op"`
}

func parseErrJSON(t *testing.T, stderr string) errJSON {
	t.Helper()
	var e errJSON
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &e); err != nil {
		t.Fatalf("stderr is not a JSON error envelope: %q", stderr)
	}
	return e
}

func md5OfFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", md5.Sum(b))
}

// tempOpsDir gives a per-test scratch directory.
func tempOpsDir(t *testing.T) string { return t.TempDir() }

// ---- set_cell ----

func TestEditSetCellAccept(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t,
		op("set_cell", "selector", "Data!A3", "value", "plain"),
		op("set_cell", "selector", "Data!B3", "value", 42),
		op("set_cell", "selector", "Data!C3", "value", true),
		op("set_cell", "selector", "Data!A4", "value", "123", "type", "string"),
		op("set_cell", "selector", "Data!B4", "value", "7", "type", "number"),
		op("set_cell", "selector", "Data!C4", "value", "=SUM(A3:A4)", "type", "formula"),
		op("set_cell", "selector", "Data!A5", "value", "2025-10-14T00:00:00Z", "type", "date"),
	))
	runJSON(t, "edit", f, ops)
	cells := runJSON(t, "cells", f, "Data", "A3:C5")
	for _, want := range []string{
		`"ref": "A3"`, `"value": "plain"`,
		`"ref": "B3"`, `"value": 42`,
		`"ref": "C3"`, `"value": true`,
		`"ref": "A4"`, `"value": "123"`,
		`"ref": "B4"`, `"value": 7`,
		`"ref": "C4"`, `"formula": "=SUM(A3:A4)"`,
		`"ref": "A5"`, `"value": "2025-10-14T00:00:00Z"`,
	} {
		if !strings.Contains(cells, want) {
			t.Errorf("cells after set_cell ops missing %s in:\n%s", want, cells)
		}
	}
	// The saved file must still open cleanly.
	runJSON(t, "info", f)
}

func TestEditSetCellRejects(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	cases := []struct {
		name string
		op   map[string]any
		code string
	}{
		{"missing selector", op("set_cell", "value", 1), "invalid_value"},
		{"missing value", op("set_cell", "selector", "Data!A1"), "invalid_value"},
		{"null value", op("set_cell", "selector", "Data!A1", "value", nil), "invalid_value"},
		{"object value", op("set_cell", "selector", "Data!A1", "value", map[string]any{"x": 1}), "invalid_value"},
		{"unknown type", op("set_cell", "selector", "Data!A1", "value", 1, "type", "float"), "invalid_value"},
		{"unknown sheet", op("set_cell", "selector", "Nope!A1", "value", 1), "not_found"},
		{"bad selector", op("set_cell", "selector", "Data", "value", 1), "invalid_selector"},
		{"bad cell ref", op("set_cell", "selector", "Data!1A", "value", 1), "invalid_range"},
		{"merged non-top-left", op("set_cell", "selector", "Summary!B4", "value", 1), "invalid_value"},
		{"number for boolean type", op("set_cell", "selector", "Data!A1", "value", 1, "type", "boolean"), "invalid_value"},
		{"formula without =", op("set_cell", "selector", "Data!A1", "value", "SUM(A1)", "type", "formula"), "invalid_value"},
		{"bad date", op("set_cell", "selector", "Data!A1", "value", "yesterday", "type", "date"), "invalid_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := writeOpsFile(t, opsDoc(t, tc.op))
			_, errOut, code := runCLI("--json", "edit", f, ops)
			if code != 1 {
				t.Fatalf("exit = %d, want 1 (stderr %s)", code, errOut)
			}
			e := parseErrJSON(t, errOut)
			if e.Code != tc.code {
				t.Errorf("code = %q, want %q (error %q)", e.Code, tc.code, e.Error)
			}
			if e.OpIndex == nil || *e.OpIndex != 0 {
				t.Errorf("op_index = %v, want 0", e.OpIndex)
			}
			if e.Op != "set_cell" {
				t.Errorf("op = %q, want set_cell", e.Op)
			}
		})
	}
}

// ---- clear_cell ----

func TestEditClearCell(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t, op("clear_cell", "selector", "Data!A1")))
	runJSON(t, "edit", f, ops)
	cells := runJSON(t, "cells", f, "Data", "A1:A1")
	if strings.Contains(cells, `"ref": "A1"`) {
		t.Errorf("A1 still present after clear_cell: %s", cells)
	}
	// A cleared cell with a missing selector is a reject.
	ops = writeOpsFile(t, opsDoc(t, op("clear_cell")))
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 || !strings.Contains(errOut, `"code":"invalid_value"`) {
		t.Errorf("clear_cell without selector: exit %d, stderr %s", code, errOut)
	}
}

// ---- set_text ----

func TestEditTextOps(t *testing.T) {
	f := copyFixture(t, pptxPath)
	ops := writeOpsFile(t, opsDoc(t,
		op("set_text", "selector", "slide/1/title", "text", "New title"),
		op("set_text", "selector", fmt.Sprintf("slide/2/shape/%d", textBoxShapeID), "text", "Line one\nLine two"),
	))
	runJSON(t, "edit", f, ops)
	text := runJSON(t, "text", f, "--slide", "2")
	if !strings.Contains(text, "Line one\\nLine two") {
		t.Errorf("slide 2 text after set_text: %s", text)
	}
	title := runJSON(t, "text", f, "--slide", "1")
	if !strings.Contains(title, "New title") {
		t.Errorf("slide 1 text after set_text: %s", title)
	}
}

func TestEditTextRejects(t *testing.T) {
	f := copyFixture(t, pptxPath)
	cases := []struct {
		name string
		op   map[string]any
		code string
	}{
		{"missing selector", op("set_text", "text", "x"), "invalid_value"},
		{"missing text", op("set_text", "selector", "slide/1/title"), "invalid_value"},
		{"non-string text", op("set_text", "selector", "slide/1/title", "text", 3), "invalid_value"},
		{"absent placeholder", op("set_text", "selector", "slide/2/title", "text", "x"), "not_found"},
		{"unknown shape", op("set_text", "selector", "slide/1/shape/99", "text", "x"), "not_found"},
		{"bare slide selector", op("set_text", "selector", "slide/1", "text", "x"), "invalid_selector"},
		{"out of range slide", op("set_text", "selector", "slide/9/title", "text", "x"), "not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := writeOpsFile(t, opsDoc(t, tc.op))
			_, errOut, code := runCLI("--json", "edit", f, ops)
			if code != 1 {
				t.Fatalf("exit = %d, want 1 (stderr %s)", code, errOut)
			}
			if e := parseErrJSON(t, errOut); e.Code != tc.code {
				t.Errorf("code = %q, want %q (error %q)", e.Code, tc.code, e.Error)
			}
		})
	}
}

// ---- add_sheet / remove_sheet ----

func TestEditAddSheet(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	// The workbook has Data and Summary, so the first unused Sheet<N> is
	// Sheet1.
	ops := writeOpsFile(t, opsDoc(t, op("add_sheet")))
	runJSON(t, "edit", f, ops)
	if sheets := runJSON(t, "ls", f, "sheets"); !strings.Contains(sheets, `"name": "Sheet1"`) {
		t.Errorf("default sheet name not chosen: %s", sheets)
	}
	// A named sheet, then a cell written into it by a later op.
	ops = writeOpsFile(t, opsDoc(t,
		op("add_sheet", "name", "Extra"),
		op("set_cell", "selector", "'Extra'!A1", "value", "later sees earlier"),
	))
	runJSON(t, "edit", f, ops)
	if cells := runJSON(t, "cells", f, "Extra", "A1"); !strings.Contains(cells, "later sees earlier") {
		t.Errorf("later op did not see earlier effect: %s", cells)
	}
}

func TestEditAddSheetRejects(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	cases := []struct {
		name string
		op   map[string]any
		code string
	}{
		{"taken name", op("add_sheet", "name", "Data"), "invalid_value"},
		{"taken case-insensitive", op("add_sheet", "name", "dATA"), "invalid_value"},
		{"forbidden character", op("add_sheet", "name", "a/b"), "invalid_value"},
		{"too long", op("add_sheet", "name", strings.Repeat("x", 32)), "invalid_value"},
		{"non-string name", op("add_sheet", "name", 7), "invalid_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := writeOpsFile(t, opsDoc(t, tc.op))
			_, errOut, code := runCLI("--json", "edit", f, ops)
			if code != 1 {
				t.Fatalf("exit = %d, want 1 (stderr %s)", code, errOut)
			}
			if e := parseErrJSON(t, errOut); e.Code != tc.code {
				t.Errorf("code = %q, want %q (error %q)", e.Code, tc.code, e.Error)
			}
		})
	}
}

func TestEditRemoveSheet(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t, op("remove_sheet", "name", "Summary")))
	runJSON(t, "edit", f, ops)
	if sheets := runJSON(t, "ls", f, "sheets"); strings.Contains(sheets, "Summary") {
		t.Errorf("Summary still present: %s", sheets)
	}
	// Unknown sheet: not_found.
	ops = writeOpsFile(t, opsDoc(t, op("remove_sheet", "name", "Nope")))
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "not_found" {
		t.Errorf("code = %q, want not_found", e.Code)
	}
	// The last remaining sheet cannot be removed.
	one := filepath.Join(tempOpsDir(t), "one.xlsx")
	runJSON(t, "new", "xlsx", one)
	ops = writeOpsFile(t, opsDoc(t, op("remove_sheet", "name", "Sheet1")))
	_, errOut, code = runCLI("--json", "edit", one, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" {
		t.Errorf("code = %q, want invalid_value (error %q)", e.Code, e.Error)
	}
}

// ---- add_slide / remove_slide ----

func TestEditAddSlide(t *testing.T) {
	f := copyFixture(t, pptxPath)
	ops := writeOpsFile(t, opsDoc(t, op("add_slide")))
	runJSON(t, "edit", f, ops)
	if slides := runJSON(t, "ls", f, "slides"); !strings.Contains(slides, `"index": 3`) {
		t.Errorf("slide not appended: %s", slides)
	}
}

func TestEditAddSlideLayout(t *testing.T) {
	one := filepath.Join(tempOpsDir(t), "one.pptx")
	runJSON(t, "new", "pptx", one)

	// Layouts accepted by the default widescreen master: title,
	// title_and_content, section_header, two_content, title_only, blank.
	for _, layout := range []string{"title", "title_and_content", "blank"} {
		ops := writeOpsFile(t, opsDoc(t, op("add_slide", "layout", layout)))
		out := runJSON(t, "--json", "edit", one, ops)
		var resp struct {
			Applied int `json:"applied"`
		}
		if err := json.Unmarshal([]byte(out), &resp); err != nil || resp.Applied != 1 {
			t.Fatalf("layout %q: edit output = %s (%v)", layout, out, err)
		}
	}

	// The slide added from the title layout has a title that set_text can
	// fill immediately.
	ops := writeOpsFile(t, opsDoc(t, op("add_slide", "layout", "title"), op("set_text", "selector", "slide/4/title", "text", "Hello")))
	runJSON(t, "edit", one, ops)
	if slides := runJSON(t, "ls", one, "slides"); !strings.Contains(slides, "\"text_preview\": \"Hello\"") {
		t.Errorf("layout slide title not set: %s", slides)
	}

	// Invalid enum value.
	bad := writeOpsFile(t, opsDoc(t, op("add_slide", "layout", "fancy")))
	_, errOut, code := runCLI("--json", "edit", one, bad)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e := parseErrJSON(t, errOut)
	if e.Code != "invalid_value" || !strings.Contains(e.Error, "fancy") {
		t.Errorf("invalid layout: code %q, error %q", e.Code, e.Error)
	}

	// A layout the default master does not have: not_found naming the type.
	missing := writeOpsFile(t, opsDoc(t, op("add_slide", "layout", "comparison")))
	_, errOut, code = runCLI("--json", "edit", one, missing)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e = parseErrJSON(t, errOut)
	if e.Code != "not_found" || !strings.Contains(e.Error, "twoTxTwoObj") {
		t.Errorf("missing layout: code %q, error %q", e.Code, e.Error)
	}
}

func TestEditRemoveSlide(t *testing.T) {
	f := copyFixture(t, pptxPath)
	ops := writeOpsFile(t, opsDoc(t, op("remove_slide", "index", 2)))
	runJSON(t, "edit", f, ops)
	if slides := runJSON(t, "ls", f, "slides"); strings.Contains(slides, `"index": 2`) {
		t.Errorf("slide 2 still present: %s", slides)
	}
	// Out of range: not_found (on a fresh two-slide fixture).
	g := copyFixture(t, pptxPath)
	ops = writeOpsFile(t, opsDoc(t, op("remove_slide", "index", 9)))
	_, errOut, code := runCLI("--json", "edit", g, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "not_found" {
		t.Errorf("code = %q, want not_found", e.Code)
	}
	// Non-integer index: invalid_value.
	opsRaw := `{"version":1,"ops":[{"op":"remove_slide","index":1.5}]}`
	ops = writeOpsFile(t, opsRaw)
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" {
		t.Errorf("code = %q, want invalid_value", e.Code)
	}
	// The last remaining slide cannot be removed.
	// Last remaining slide (invalid_value) vs out of range (not_found):
	// resolveSlide runs first, so a single-slide deck with a large index
	// reports not_found.
	one := filepath.Join(tempOpsDir(t), "one.pptx")
	runJSON(t, "new", "pptx", one)
	ops = writeOpsFile(t, opsDoc(t, op("remove_slide", "index", 5)))
	_, errOut, code = runCLI("--json", "edit", one, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "not_found" {
		t.Errorf("code = %q, want not_found (error %q)", e.Code, e.Error)
	}
	runJSON(t, "edit", one, writeOpsFile(t, opsDoc(t, op("add_slide"))))
	ops = writeOpsFile(t, opsDoc(t, op("remove_slide", "index", 1)))
	_, errOut, code = runCLI("--json", "edit", one, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" {
		t.Errorf("code = %q, want invalid_value (error %q)", e.Code, e.Error)
	}
}

// ---- set_properties ----

func TestEditSetProperties(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t, op("set_properties", "title", "New title", "subject", "Subj", "keywords", "a, b")))
	runJSON(t, "edit", f, ops)
	if info := runJSON(t, "info", f); !strings.Contains(info, `"title": "New title"`) {
		t.Errorf("properties not applied: %s", info)
	}
	// Empty string clears.
	ops = writeOpsFile(t, opsDoc(t, op("set_properties", "title", "")))
	runJSON(t, "edit", f, ops)
	if info := runJSON(t, "info", f); strings.Contains(info, `"title": "New title"`) {
		t.Errorf("empty string did not clear the title: %s", info)
	}

	g := copyFixture(t, pptxPath)
	ops = writeOpsFile(t, opsDoc(t, op("set_properties", "creator", "Grace")))
	runJSON(t, "edit", g, ops)
	if info := runJSON(t, "info", g); !strings.Contains(info, `"creator": "Grace"`) {
		t.Errorf("properties not applied to pptx: %s", info)
	}
	// Non-string field: invalid_value.
	ops = writeOpsFile(t, opsDoc(t, op("set_properties", "title", 5)))
	_, errOut, code := runCLI("--json", "edit", g, ops)
	if code != 1 || !strings.Contains(errOut, `"code":"invalid_value"`) {
		t.Errorf("non-string property: exit %d, stderr %s", code, errOut)
	}
}

// ---- envelope and dispatch errors ----

func TestEditEnvelopeErrors(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	cases := []struct {
		name string
		doc  string
		code string
	}{
		{"wrong version", `{"version":2,"ops":[{"op":"add_slide"}]}`, "invalid_value"},
		{"missing version", `{"ops":[{"op":"add_slide"}]}`, "invalid_value"},
		{"missing ops", `{"version":1}`, "invalid_value"},
		{"empty ops", `{"version":1,"ops":[]}`, "invalid_value"},
		{"ops not array", `{"version":1,"ops":{"op":"add_slide"}}`, "invalid_value"},
		{"op not object", `{"version":1,"ops":[7]}`, "invalid_value"},
		{"op without op field", `{"version":1,"ops":[{"selector":"Data!A1"}]}`, "invalid_value"},
		{"op non-string", `{"version":1,"ops":[{"op":7}]}`, "invalid_value"},
		{"not json", `nonsense`, "invalid_value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ops := writeOpsFile(t, tc.doc)
			_, errOut, code := runCLI("--json", "edit", f, ops)
			if code != 1 {
				t.Fatalf("exit = %d, want 1 (stderr %s)", code, errOut)
			}
			e := parseErrJSON(t, errOut)
			if e.Code != tc.code {
				t.Errorf("code = %q, want %q", e.Code, tc.code)
			}
			// Per-operation failures carry an op_index; envelope errors do not.
			perOp := tc.name == "op not object" || tc.name == "op without op field" || tc.name == "op non-string"
			if !perOp && e.OpIndex != nil {
				t.Errorf("envelope error should have no op_index, got %d", *e.OpIndex)
			}
			if perOp && (e.OpIndex == nil || *e.OpIndex != 0) {
				t.Errorf("op_index = %v, want 0", e.OpIndex)
			}
		})
	}
}

func TestEditUnknownOpAndField(t *testing.T) {
	f := copyFixture(t, xlsxPath)

	ops := writeOpsFile(t, opsDoc(t, op("foo")))
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e := parseErrJSON(t, errOut)
	if e.Code != "invalid_value" {
		t.Errorf("code = %q, want invalid_value", e.Code)
	}
	want := `unknown operation "foo"; run ` + "`spine ops-schema`" + ` for the list`
	if e.Error != want {
		t.Errorf("message = %q, want %q", e.Error, want)
	}

	// A typo'd field name must be named in the error.
	ops = writeOpsFile(t, `{"version":1,"ops":[{"op":"set_cell","selector":"Data!B1","valu":42}]}`)
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e = parseErrJSON(t, errOut)
	if !strings.Contains(e.Error, `"valu"`) {
		t.Errorf("unknown-field error does not name the field: %q", e.Error)
	}
	if e.OpIndex == nil || *e.OpIndex != 0 || e.Op != "set_cell" {
		t.Errorf("op context = %v/%q, want 0/set_cell", e.OpIndex, e.Op)
	}

	// With two unknown fields the reported one is deterministic: the
	// lexicographically first.
	ops = writeOpsFile(t, opsDoc(t, op("set_cell", "selector", "Data!B1", "value", 1, "valu", 1, "text", "x")))
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e = parseErrJSON(t, errOut)
	if !strings.Contains(e.Error, `"text"`) || strings.Contains(e.Error, `"valu"`) {
		t.Errorf("unknown-field error = %q, want the lexicographically-first field \"text\"", e.Error)
	}

	// A field belonging to another operation is also rejected.
	ops = writeOpsFile(t, opsDoc(t, op("set_cell", "selector", "Data!B1", "value", 1, "text", "x")))
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 || !strings.Contains(errOut, `does not accept field`) || !strings.Contains(errOut, "text") {
		t.Errorf("cross-op field accepted: exit %d, stderr %s", code, errOut)
	}

	// Ops applying to the wrong kind are unsupported.
	g := copyFixture(t, pptxPath)
	ops = writeOpsFile(t, opsDoc(t, op("set_cell", "selector", "Data!A1", "value", 1)))
	_, errOut, code = runCLI("--json", "edit", g, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "unsupported" {
		t.Errorf("code = %q, want unsupported", e.Code)
	}
}

// ---- batch behavior ----

func TestEditSuccessOutputGolden(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t,
		op("set_cell", "selector", "Data!B1", "value", 42),
		op("set_cell", "selector", "Data!B2", "value", "x"),
	))
	out := runJSON(t, "edit", f, ops)
	if out != "{\n  \"applied\": 2\n}\n" {
		t.Errorf("edit output = %q, want {\"applied\": 2}", out)
	}
}

func TestEditFirstFailureAborts(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	// The first op succeeds in memory; the second fails; nothing is saved.
	ops := writeOpsFile(t, opsDoc(t,
		op("set_cell", "selector", "Data!A1", "value", "changed"),
		op("remove_sheet", "name", "Nope"),
	))
	before := md5OfFile(t, f)
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e := parseErrJSON(t, errOut)
	if e.OpIndex == nil || *e.OpIndex != 1 {
		t.Errorf("op_index = %v, want 1", e.OpIndex)
	}
	if e.Op != "remove_sheet" {
		t.Errorf("op = %q, want remove_sheet", e.Op)
	}
	if after := md5OfFile(t, f); after != before {
		t.Error("a failing batch modified the file")
	}
	entries, err := os.ReadDir(filepath.Dir(f))
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if strings.HasPrefix(en.Name(), ".spine-") && strings.HasSuffix(en.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", en.Name())
		}
	}
	// The in-memory change from op 0 must not have leaked to disk.
	cells := runJSON(t, "cells", f, "Data", "A1")
	if strings.Contains(cells, "changed") {
		t.Error("first op's effect leaked to disk despite the failure")
	}
}

func TestEditViaStdin(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	doc := opsDoc(t, op("set_cell", "selector", "Data!B1", "value", 42))
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(doc); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	saved := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = saved }()
	runJSON(t, "edit", f, "-")
	if cells := runJSON(t, "cells", f, "Data", "B1"); !strings.Contains(cells, `"value": 42`) {
		t.Errorf("stdin edit not applied: %s", cells)
	}
}

func TestEditOutFlag(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	dst := filepath.Join(tempOpsDir(t), "out.xlsx")
	ops := writeOpsFile(t, opsDoc(t, op("set_cell", "selector", "Data!B1", "value", 42)))
	runJSON(t, "edit", f, ops, "--out", dst)
	if cells := runJSON(t, "cells", dst, "Data", "B1"); !strings.Contains(cells, `"value": 42`) {
		t.Errorf("--out output missing the edit: %s", cells)
	}
	if cells := runJSON(t, "cells", f, "Data", "B1"); strings.Contains(cells, `"value": 42`) {
		t.Error("--out modified the input file")
	}
}

// ---- ops-schema ----

func TestOpsSchemaParsesAndRoundTrips(t *testing.T) {
	out := runJSON(t, "ops-schema")
	var doc struct {
		Version    int `json:"version"`
		Operations []struct {
			Op        string `json:"op"`
			AppliesTo []any  `json:"applies_to"`
			Fields    map[string]struct {
				Type        string   `json:"type"`
				Required    bool     `json:"required"`
				Enum        []string `json:"enum"`
				Description string   `json:"description"`
			} `json:"fields"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("ops-schema output is not JSON: %v", err)
	}
	if doc.Version != 1 {
		t.Errorf("schema version = %d, want 1", doc.Version)
	}
	wantOps := map[string]bool{}
	for _, o := range opRegistry {
		wantOps[o.Name] = false
	}
	seen := map[string]bool{}
	for _, o := range doc.Operations {
		seen[o.Op] = true
		if len(o.AppliesTo) == 0 {
			t.Errorf("op %q has empty applies_to", o.Op)
		}
	}
	for name := range wantOps {
		if !seen[name] {
			t.Errorf("ops-schema is missing operation %q", name)
		}
	}
	for name := range seen {
		if _, ok := wantOps[name]; !ok {
			t.Errorf("ops-schema lists unknown operation %q", name)
		}
	}
	// Destructiveness warnings are in the descriptions.
	out_ := out
	if !strings.Contains(out_, "#REF!") {
		t.Error("remove_sheet schema lacks the #REF! cascade warning")
	}
}

func TestOpsSchemaSetCellGolden(t *testing.T) {
	// The spec's set_cell schema example, reproduced from the generated
	// output.
	out := runJSON(t, "ops-schema")
	var doc struct {
		Operations []struct {
			Op     string `json:"op"`
			Fields map[string]struct {
				Type     string   `json:"type"`
				Required bool     `json:"required"`
				Enum     []string `json:"enum"`
			} `json:"fields"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	var setCell *struct {
		Op     string `json:"op"`
		Fields map[string]struct {
			Type     string   `json:"type"`
			Required bool     `json:"required"`
			Enum     []string `json:"enum"`
		} `json:"fields"`
	}
	for i := range doc.Operations {
		if doc.Operations[i].Op == "set_cell" {
			setCell = &doc.Operations[i]
		}
	}
	if setCell == nil {
		t.Fatal("set_cell missing from schema")
	}
	sel := setCell.Fields["selector"]
	if sel.Type != "string" || !sel.Required {
		t.Errorf("selector field = %+v", sel)
	}
	val := setCell.Fields["value"]
	if val.Type != "string|number|boolean" || !val.Required {
		t.Errorf("value field = %+v", val)
	}
	typ := setCell.Fields["type"]
	if typ.Required || typ.Type != "string" {
		t.Errorf("type field = %+v", typ)
	}
	wantEnum := []string{"string", "number", "boolean", "formula", "date"}
	if fmt.Sprint(typ.Enum) != fmt.Sprint(wantEnum) {
		t.Errorf("type enum = %v, want %v", typ.Enum, wantEnum)
	}
}

// TestOpsSchemaRoundTrip iterates the generated schema, builds one minimal
// batch per operation, and asserts the op is accepted (no "unknown
// operation"); destructive ops are paired with what they need.
func TestOpsSchemaRoundTrip(t *testing.T) {
	out := runJSON(t, "ops-schema")
	var doc struct {
		Operations []struct {
			Op        string `json:"op"`
			AppliesTo []any  `json:"applies_to"`
		} `json:"operations"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	kindOf := func(opName string) string {
		for _, o := range doc.Operations {
			if o.Op == opName {
				if len(o.AppliesTo) > 0 && o.AppliesTo[0] == "pptx" {
					return "pptx"
				}
				return "xlsx"
			}
		}
		return "xlsx"
	}
	minimal := map[string][]map[string]any{
		"set_cell":       {op("set_cell", "selector", "Sheet1!A1", "value", 1)},
		"clear_cell":     {op("clear_cell", "selector", "Sheet1!A1")},
		"set_text":       {op("add_slide"), op("set_text", "selector", "slide/2/title", "text", "x")},
		"add_sheet":      {op("add_sheet")},
		"remove_sheet":   {op("add_sheet", "name", "Temp"), op("remove_sheet", "name", "Temp")},
		"add_slide":      {op("add_slide")},
		"remove_slide":   {op("add_slide"), op("add_slide"), op("remove_slide", "index", 2)},
		"set_properties": {op("set_properties", "title", "x")},
	}
	for _, o := range doc.Operations {
		batch, ok := minimal[o.Op]
		if !ok {
			t.Errorf("schema lists operation %q with no minimal test batch", o.Op)
			continue
		}
		kind := kindOf(o.Op)
		f := filepath.Join(tempOpsDir(t), "rt-"+o.Op+"."+kind)
		runJSON(t, "new", kind, f)
		ops := writeOpsFile(t, opsDoc(t, batch...))
		_, errOut, code := runCLI("--json", "edit", f, ops)
		if strings.Contains(errOut, "unknown operation") {
			t.Errorf("op %q from the schema was not accepted: %s", o.Op, errOut)
		}
		if o.Op != "set_text" && code != 0 {
			t.Errorf("minimal batch for %q failed: %s", o.Op, errOut)
		}
	}
}

// ---- spine new ----

func TestNewCreatesFiles(t *testing.T) {
	dir := tempOpsDir(t)
	x := filepath.Join(dir, "n.xlsx")
	out := runJSON(t, "new", "xlsx", x, "--title", "T", "--creator", "C")
	if out != "{\n  \"kind\": \"xlsx\",\n  \"path\": \""+x+"\"\n}\n" {
		t.Errorf("new output = %q", out)
	}
	info := runJSON(t, "info", x) // the created file must pass info
	if !strings.Contains(info, `"kind": "xlsx"`) || !strings.Contains(info, `"title": "T"`) || !strings.Contains(info, `"creator": "C"`) {
		t.Errorf("created xlsx does not pass info: %s", info)
	}

	p := filepath.Join(dir, "n.pptx")
	out = runJSON(t, "new", "pptx", p)
	if out != "{\n  \"kind\": \"pptx\",\n  \"path\": \""+p+"\"\n}\n" {
		t.Errorf("new output = %q", out)
	}
	if info := runJSON(t, "info", p); !strings.Contains(info, `"kind": "pptx"`) {
		t.Errorf("created pptx does not pass info: %s", info)
	}
}

func TestNewRejects(t *testing.T) {
	dir := tempOpsDir(t)
	x := filepath.Join(dir, "n.xlsx")
	runJSON(t, "new", "xlsx", x)
	_, errOut, code := runCLI("--json", "new", "xlsx", x)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e := parseErrJSON(t, errOut)
	if e.Code != "io" || !strings.Contains(e.Error, "file already exists: "+x) {
		t.Errorf("overwrite error = %+v", e)
	}
	// The original file is untouched.
	if info := runJSON(t, "info", x); !strings.Contains(info, `"kind": "xlsx"`) {
		t.Errorf("existing file damaged: %s", info)
	}
	_, errOut, code = runCLI("--json", "new", "docx", filepath.Join(dir, "n.docx"))
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" {
		t.Errorf("kind error = %+v", e)
	}
}

// ---- spec document example ----

func TestSpecOpsDocumentExample(t *testing.T) {
	// The spec's operations document shape: version plus an ops array whose
	// elements carry an "op" field. The example mixes kinds, so its cells op
	// runs against a copy of the xlsx fixture and add_slide is verified to be
	// rejected as unsupported (kind mismatch), not as a document error.
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, `{"version":1,"ops":[{"op":"set_cell","selector":"Data!B1","value":42}]}`)
	out := runJSON(t, "edit", f, ops)
	if out != "{\n  \"applied\": 1\n}\n" {
		t.Errorf("spec example edit output = %q", out)
	}
	if cells := runJSON(t, "cells", f, "Data", "B1"); !strings.Contains(cells, `"value": 42`) {
		t.Errorf("spec example value not stored: %s", cells)
	}
}
