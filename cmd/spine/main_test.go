package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// Fixture paths and the shape ids the fixtures get after their first save.
var (
	xlsxPath, pptxPath, encPath, encXlsxPath string

	titleShapeID    uint32 // slide 1's title placeholder id
	bodyShapeID     uint32 // slide 1's body placeholder id
	textBoxShapeID  uint32 // slide 2's text box id
	slide2HasNoText = ""   // placeholder for the "no title on slide 2" case
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "spine-cli-test")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	xlsxPath, pptxPath, encPath, encXlsxPath, err = buildFixtures(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// buildFixtures generates the xlsx and pptx fixtures with the library itself
// (no binary testdata) and returns their paths plus an encrypted copy.
func buildFixtures(dir string) (xlsxPath, pptxPath, encPath, encXlsxPath string, err error) {
	xlsxPath = filepath.Join(dir, "book.xlsx")
	pptxPath = filepath.Join(dir, "deck.pptx")
	encPath = filepath.Join(dir, "deck-enc.pptx")
	encXlsxPath = filepath.Join(dir, "book-enc.xlsx")

	wb := xlsx.Create()
	wb.Properties.Title = "Q3 report"
	wb.Properties.Creator = "Ada"
	data, err := wb.AddSheet("Data")
	if err != nil {
		return "", "", "", "", err
	}
	summary, err := wb.AddSheet("Summary")
	if err != nil {
		return "", "", "", "", err
	}
	var c *xlsx.Cell
	c, _ = data.Cell("A1")
	c.SetString("Revenue")
	c, _ = data.Cell("B1")
	c.SetFormula("=SUM(B2:B9)")
	c, _ = data.Cell("C1")
	c.SetFloat(42)
	c, _ = data.Cell("A2")
	c.SetBool(true)
	c, _ = data.Cell("B2")
	c.SetTime(time.Date(2025, 10, 14, 0, 0, 0, 0, time.UTC))
	if err := c.SetStyle(xlsx.CellStyle{NumberFormatID: xlsx.NumberFormatDate}); err != nil {
		return "", "", "", "", err
	}
	c, _ = summary.Cell("A1")
	c.SetString("Total")
	if err := summary.MergeCells("A4", "B5"); err != nil {
		return "", "", "", "", err
	}
	if err := wb.Save(xlsxPath); err != nil {
		return "", "", "", "", err
	}
	if err := wb.SaveEncrypted(encXlsxPath, "s3cret"); err != nil {
		return "", "", "", "", err
	}

	p := pptx.CreateWidescreen()
	p.Properties.Title = "Deck"
	p.Properties.Creator = "Ada"
	s1 := p.AddSlide()
	s1.SetName("Title Slide")
	title := pptx.NewPlaceholderShape(pptx.PlaceholderTitle)
	title.SetText("Hello")
	if err := s1.AddShape(title); err != nil {
		return "", "", "", "", err
	}
	body := pptx.NewPlaceholderShape(pptx.PlaceholderBody)
	body.SetText("Body line\nSecond line")
	if err := s1.AddShape(body); err != nil {
		return "", "", "", "", err
	}
	s2 := p.AddSlide()
	s2.SetName("Bullets")
	tb := s2.AddTextBox()
	tb.TextFrame().SetText("A text box")
	if err := p.Save(pptxPath); err != nil {
		return "", "", "", "", err
	}
	if err := p.SaveEncrypted(encPath, "s3cret"); err != nil {
		return "", "", "", "", err
	}

	// Reopen to learn the saved shape ids the selectors address.
	loaded, err := pptx.Open(pptxPath)
	if err != nil {
		return "", "", "", "", err
	}
	defer loaded.Close()
	for _, sh := range loaded.Slides()[0].Shapes() {
		if ph, ok := sh.(*pptx.PlaceholderShape); ok {
			if ph.IsTitle() {
				titleShapeID = sh.ID()
			} else if ph.IsBody() {
				bodyShapeID = sh.ID()
			}
		}
	}
	for _, sh := range loaded.Slides()[1].Shapes() {
		if _, ok := sh.(*pptx.TextBox); ok {
			textBoxShapeID = sh.ID()
		}
	}
	return xlsxPath, pptxPath, encPath, encXlsxPath, nil
}

// copyFixture copies a fixture so tests that mutate it stay independent.
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return dst
}

// runCLI runs one command line and returns stdout, stderr and the exit code.
func runCLI(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return out.String(), errb.String(), code
}

// runJSON runs a command expecting success and returns its stdout.
func runJSON(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := runCLI(append([]string{"--json"}, args...)...)
	if code != 0 {
		t.Fatalf("command %v failed (exit %d): %s", args, code, errOut)
	}
	return out
}

func TestInfoXLSXGolden(t *testing.T) {
	want := `{
  "kind": "xlsx",
  "sheet_count": 2,
  "sheets": [
    {
      "index": 0,
      "name": "Data",
      "active": true
    },
    {
      "index": 1,
      "name": "Summary",
      "active": false
    }
  ],
  "title": "Q3 report",
  "creator": "Ada"
}
`
	if got := runJSON(t, "info", xlsxPath); got != want {
		t.Errorf("info xlsx output\n got: %s\nwant: %s", got, want)
	}
}

func TestInfoPPTXGolden(t *testing.T) {
	want := `{
  "kind": "pptx",
  "slide_count": 2,
  "slides": [
    {
      "index": 1,
      "name": "Title Slide",
      "shape_count": 2
    },
    {
      "index": 2,
      "name": "Bullets",
      "shape_count": 1
    }
  ],
  "title": "Deck",
  "creator": "Ada"
}
`
	if got := runJSON(t, "info", pptxPath); got != want {
		t.Errorf("info pptx output\n got: %s\nwant: %s", got, want)
	}
}

func TestInfoEncrypted(t *testing.T) {
	out, errOut, code := runCLI("--json", "info", encPath)
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Errorf("encrypted without password: exit %d, stderr %s", code, errOut)
	}
	out, errOut, code = runCLI("--json", "info", encPath, "--password", "wrong")
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Errorf("wrong password: exit %d, stderr %s", code, errOut)
	}
	out, _, code = runCLI("--json", "info", encPath, "--password", "s3cret")
	if code != 0 || !strings.Contains(out, `"kind": "pptx"`) {
		t.Errorf("encrypted with password: exit %d, stdout %s", code, out)
	}
}

func TestInfoNotFoundsAndUnsupported(t *testing.T) {
	out, errOut, code := runCLI("--json", "info", filepath.Join(t.TempDir(), "missing.xlsx"))
	if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) {
		t.Errorf("missing file: exit %d, stderr %s", code, errOut)
	}
	plain := filepath.Join(t.TempDir(), "plain.txt")
	os.WriteFile(plain, []byte("hello"), 0o600)
	out, errOut, code = runCLI("--json", "info", plain)
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Errorf("plain text file: exit %d, stderr %s", code, errOut)
	}
	_ = out
}

func TestLsGolden(t *testing.T) {
	sheets := runJSON(t, "ls", xlsxPath, "sheets")
	if !strings.Contains(sheets, `"name": "Data"`) || !strings.Contains(sheets, `"active": true`) {
		t.Errorf("ls sheets: %s", sheets)
	}
	slides := runJSON(t, "ls", pptxPath, "slides")
	if !strings.Contains(slides, `"name": "Title Slide"`) || !strings.Contains(slides, `"text_preview": "Hello Body line Second line"`) {
		t.Errorf("ls slides: %s", slides)
	}
	shapes := runJSON(t, "ls", pptxPath, "slide/1")
	want := fmt.Sprintf(`[
  {
    "id": %d,
    "name": "title",
    "type": "placeholder",
    "placeholder": {
      "type": "title",
      "idx": 0
    },
    "text_preview": "Hello"
  },
  {
    "id": %d,
    "name": "body",
    "type": "placeholder",
    "placeholder": {
      "type": "body",
      "idx": 0
    },
    "text_preview": "Body line Second line"
  }
]
`, titleShapeID, bodyShapeID)
	if shapes != want {
		t.Errorf("ls slide/1\n got: %s\nwant: %s", shapes, want)
	}
}

func TestLsErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"ls", xlsxPath, "slides"}, "unsupported"},
		{[]string{"ls", pptxPath, "sheets"}, "unsupported"},
		{[]string{"ls", pptxPath, "bogus"}, "invalid_selector"},
		{[]string{"ls", pptxPath, "slide/x"}, "invalid_selector"},
		{[]string{"ls", pptxPath, "slide/9"}, "not_found"},
	} {
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"`+tc.code+`"`) {
			t.Errorf("%v: exit %d, stderr %s, want code %s", tc.args, code, errOut, tc.code)
		}
	}
}

func TestText(t *testing.T) {
	out, _, code := runCLI("text", pptxPath)
	if code != 0 || out != "Hello\nBody line\nSecond line\n\nA text box\n" {
		t.Errorf("text pptx: exit %d, stdout %q", code, out)
	}
	out, _, code = runCLI("text", pptxPath, "--slide", "2")
	if code != 0 || out != "A text box\n" {
		t.Errorf("text --slide 2: exit %d, stdout %q", code, out)
	}
	out, _, code = runCLI("--json", "text", pptxPath, "--slide", "2")
	if code != 0 || !strings.Contains(out, `"text": "A text box"`) {
		t.Errorf("text --json: exit %d, stdout %q", code, out)
	}
	out, _, code = runCLI("text", xlsxPath, "--sheet", "Summary")
	if code != 0 || out != "Total\n" {
		t.Errorf("text --sheet Summary: exit %d, stdout %q", code, out)
	}
	_, errOut, code := runCLI("--json", "text", xlsxPath, "--sheet", "Missing")
	if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) {
		t.Errorf("text missing sheet: exit %d, stderr %s", code, errOut)
	}
	_, errOut, code = runCLI("--json", "text", pptxPath, "--slide", "9")
	if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) {
		t.Errorf("text missing slide: exit %d, stderr %s", code, errOut)
	}
}

func TestCellsGolden(t *testing.T) {
	want := `{
  "sheet": "Data",
  "range": "A1:C2",
  "cells": [
    {
      "ref": "A1",
      "type": "string",
      "value": "Revenue"
    },
    {
      "ref": "B1",
      "type": "formula",
      "value": null,
      "formula": "=SUM(B2:B9)"
    },
    {
      "ref": "C1",
      "type": "number",
      "value": 42
    },
    {
      "ref": "A2",
      "type": "boolean",
      "value": true
    },
    {
      "ref": "B2",
      "type": "date",
      "value": "2025-10-14T00:00:00Z"
    }
  ]
}
`
	if got := runJSON(t, "cells", xlsxPath, "Data", "A1:C2"); got != want {
		t.Errorf("cells output\n got: %s\nwant: %s", got, want)
	}
	// A reversed range reads the same cells.
	got := runJSON(t, "cells", xlsxPath, "Data", "C2:A1")
	if !strings.Contains(got, `"range": "A1:C2"`) {
		t.Errorf("reversed range: %s", got)
	}
	// Single cell.
	got = runJSON(t, "cells", xlsxPath, "Data", "B2")
	if !strings.Contains(got, `"range": "B2"`) || !strings.Contains(got, `"type": "date"`) {
		t.Errorf("single cell: %s", got)
	}
	// Human format: tab-separated lines.
	out, _, code := runCLI("cells", xlsxPath, "Data", "A1:A2")
	if code != 0 || out != "A1\tstring\tRevenue\nA2\tboolean\ttrue\n" {
		t.Errorf("cells human: exit %d, stdout %q", code, out)
	}
}

func TestCellsErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"cells", pptxPath, "x", "A1"}, "unsupported"},
		{[]string{"cells", xlsxPath, "Missing", "A1"}, "not_found"},
		{[]string{"cells", xlsxPath, "Data", "A1:"}, "invalid_range"},
		{[]string{"cells", xlsxPath, "Data", "notacell"}, "invalid_range"},
	} {
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"`+tc.code+`"`) {
			t.Errorf("%v: exit %d, stderr %s, want code %s", tc.args, code, errOut, tc.code)
		}
	}
	// An empty range emits an empty array, not null.
	if got := runJSON(t, "cells", xlsxPath, "Data", "Z100:Z100"); !strings.Contains(got, `"cells": []`) {
		t.Errorf("empty range: %s", got)
	}
}

func TestSetXlsxTypes(t *testing.T) {
	for _, tc := range []struct {
		flag, value, typ, valueJSON string
	}{
		{"", "3.5", "number", "3.5"},
		{"", "true", "boolean", "true"},
		{"", "hello", "string", `"hello"`},
		{"", "=SUM(A1:A2)", "formula", "null"},
		{"number", "7", "number", "7"},
		{"string", "42", "string", `"42"`},
		{"boolean", "false", "boolean", "false"},
		{"formula", "=A1+1", "formula", "null"},
		{"date", "2025-10-14T00:00:00Z", "date", `"2025-10-14T00:00:00Z"`},
	} {
		path := copyFixture(t, xlsxPath)
		args := []string{"set", path, "Data!G1", tc.value}
		if tc.flag != "" {
			args = append(args, "--type", tc.flag)
		}
		out := runJSON(t, args...)
		want := fmt.Sprintf("{\n  \"selector\": \"Data!G1\",\n  \"type\": %q,\n  \"value\": %s\n}\n", tc.typ, tc.valueJSON)
		if out != want {
			t.Errorf("set %v: got %s, want %s", args, out, want)
		}
		// The value round-trips through the read path.
		if got := runJSON(t, "cells", path, "Data", "G1"); !strings.Contains(got, tc.valueJSON) {
			t.Errorf("round trip of %v: %s", tc.value, got)
		}
	}
}

func TestSetXlsxErrors(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		code     string
		contains string
	}{
		{[]string{"set", xlsxPath, "B2", "x"}, "invalid_selector", ""},
		{[]string{"set", xlsxPath, "'Q3'!A1", "x"}, "not_found", ""},
		{[]string{"set", xlsxPath, "Dat!A1", "x"}, "not_found", "closest"},
		{[]string{"set", xlsxPath, "Missing!A1", "x"}, "not_found", ""},
		{[]string{"set", xlsxPath, "Data!ZZZZ1", "x"}, "invalid_range", ""},
		{[]string{"set", xlsxPath, "Data!A1", "yes", "--type", "boolean"}, "invalid_value", ""},
		{[]string{"set", xlsxPath, "Data!A1", "x", "--type", "number"}, "invalid_value", ""},
		{[]string{"set", xlsxPath, "Data!A1", "A1+1", "--type", "formula"}, "invalid_value", ""},
		{[]string{"set", xlsxPath, "Data!A1", "yesterday", "--type", "date"}, "invalid_value", ""},
		{[]string{"set", xlsxPath, "Summary!A5", "x"}, "invalid_value", "A5 is part of merged range A4:B5; write to A4"},
	} {
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"`+tc.code+`"`) {
			t.Errorf("%v: exit %d, stderr %s, want code %s", tc.args, code, errOut, tc.code)
		}
		if tc.contains != "" && !strings.Contains(errOut, tc.contains) {
			t.Errorf("%v: stderr %s misses %q", tc.args, errOut, tc.contains)
		}
	}
}

func TestSetMergedTopLeftAllowed(t *testing.T) {
	path := copyFixture(t, xlsxPath)
	out := runJSON(t, "set", path, "Summary!A4", "Merged!")
	if !strings.Contains(out, `"value": "Merged!"`) {
		t.Errorf("set merged top-left: %s", out)
	}
}

func TestSetPptxText(t *testing.T) {
	path := copyFixture(t, pptxPath)
	out := runJSON(t, "set", path, fmt.Sprintf("slide/1/title"), "Changed")
	want := fmt.Sprintf("{\n  \"selector\": \"slide/1/title\",\n  \"shape_id\": %d,\n  \"text\": \"Changed\"\n}\n", titleShapeID)
	if out != want {
		t.Errorf("set title\n got: %s\nwant: %s", out, want)
	}
	if got := runJSON(t, "text", path, "--slide", "1"); !strings.Contains(got, "Changed") {
		t.Errorf("text after set: %s", got)
	}
	out = runJSON(t, "set", path, "slide/1/body", "One\nTwo")
	if !strings.Contains(out, `"text": "One\nTwo"`) {
		t.Errorf("set body: %s", out)
	}
	out = runJSON(t, "set", path, fmt.Sprintf("slide/2/shape/%d", textBoxShapeID), "Box")
	if !strings.Contains(out, `"shape_id": `+fmt.Sprint(textBoxShapeID)) || !strings.Contains(out, `"text": "Box"`) {
		t.Errorf("set shape: %s", out)
	}
}

func TestSetPptxErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code string
	}{
		{[]string{"set", pptxPath, "slide/9/title", "x"}, "not_found"},
		{[]string{"set", pptxPath, "slide/2/title", "x"}, "not_found"},
		{[]string{"set", pptxPath, "slide/2/body", "x"}, "not_found"},
		{[]string{"set", pptxPath, "slide/1/shape/0", "x"}, "not_found"},
		{[]string{"set", pptxPath, "slide/1/shape/999", "x"}, "not_found"},
		{[]string{"set", pptxPath, "slide/x/title", "x"}, "invalid_selector"},
		{[]string{"set", pptxPath, "slide/1/chair", "x"}, "invalid_selector"},
		{[]string{"set", xlsxPath, "slide/1/title", "x"}, "invalid_selector"},
	} {
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"`+tc.code+`"`) {
			t.Errorf("%v: exit %d, stderr %s, want code %s", tc.args, code, errOut, tc.code)
		}
	}
	// An unknown --type is a command line error, exit 2.
	if _, _, code := runCLI("set", xlsxPath, "Data!A1", "x", "--type", "nope"); code != 2 {
		t.Errorf("unknown --type: exit %d", code)
	}
	// --type is not accepted for pptx text: flag parse error, exit 2.
	_, errOut, code := runCLI("--json", "set", pptxPath, "slide/1/title", "x", "--type", "string")
	if code != 2 {
		t.Errorf("set pptx --type: exit %d, stderr %s", code, errOut)
	}
}

func TestSetOutLeavesInputUntouched(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.xlsx")
	data, _ := os.ReadFile(xlsxPath)
	os.WriteFile(src, data, 0o600)
	dst := filepath.Join(dir, "out.xlsx")
	runJSON(t, "set", src, "--out", dst, "Data!H1", "spilled")
	if got := runJSON(t, "cells", src, "Data", "H1:H1"); strings.Contains(got, "spilled") {
		t.Errorf("input changed: %s", got)
	}
	if got := runJSON(t, "cells", dst, "Data", "H1:H1"); !strings.Contains(got, "spilled") {
		t.Errorf("output missing write: %s", got)
	}
}

func TestSetEncryptedRoundTrip(t *testing.T) {
	path := copyFixture(t, encPath)
	runJSON(t, "set", path, "--password", "s3cret", "slide/1/title", "Still encrypted")
	// The output is still encrypted: the wrong password fails, the right one reads.
	_, errOut, code := runCLI("--json", "info", path, "--password", "wrong")
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Errorf("re-encrypted file opened with wrong password: exit %d, stderr %s", code, errOut)
	}
	out, _, code := runCLI("--json", "info", path, "--password", "s3cret")
	if code != 0 {
		t.Errorf("re-encrypted file: exit %d, stderr", code)
	}
	if !strings.Contains(out, "pptx") {
		t.Errorf("info on re-encrypted file: %s", out)
	}
}

func TestSetRoundTripAndInfo(t *testing.T) {
	// A set must not corrupt the file: info and the read commands still work.
	path := copyFixture(t, xlsxPath)
	runJSON(t, "set", path, "Data!I9", "kept")
	runJSON(t, "info", path)
	if got := runJSON(t, "cells", path, "Data", "I9:I9"); !strings.Contains(got, `"value": "kept"`) {
		t.Errorf("round trip: %s", got)
	}
	path = copyFixture(t, pptxPath)
	runJSON(t, "set", path, "slide/1/body", "Rewritten")
	runJSON(t, "info", path)
	if got := runJSON(t, "text", path, "--slide", "1"); !strings.Contains(got, "Rewritten") {
		t.Errorf("round trip: %s", got)
	}
}

func TestAtomicity(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.xlsx")
	data, _ := os.ReadFile(xlsxPath)
	os.WriteFile(src, data, 0o600)
	before, _ := os.ReadFile(src)
	locked := filepath.Join(dir, "locked")
	os.Mkdir(locked, 0o500)
	_, errOut, code := runCLI("--json", "set", src, "--out", filepath.Join(locked, "out.xlsx"), "Data!J1", "x")
	if code != 1 || !strings.Contains(errOut, `"code":"io"`) {
		t.Errorf("unwritable directory: exit %d, stderr %s", code, errOut)
	}
	after, _ := os.ReadFile(src)
	if !bytes.Equal(before, after) {
		t.Error("input file changed by a failed save")
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".spine-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
	// Saving over a directory and into a missing parent are io failures too.
	_, errOut, code = runCLI("--json", "set", src, "--out", dir, "Data!J1", "x")
	if code != 1 || !strings.Contains(errOut, `"code":"io"`) {
		t.Errorf("save onto a directory: exit %d, stderr %s", code, errOut)
	}
	_, errOut, code = runCLI("--json", "set", src, "--out", filepath.Join(dir, "nope", "out.xlsx"), "Data!J1", "x")
	if code != 1 || !strings.Contains(errOut, `"code":"io"`) {
		t.Errorf("save into a missing parent: exit %d, stderr %s", code, errOut)
	}
}

func TestDeterminism(t *testing.T) {
	for _, args := range [][]string{
		{"cells", xlsxPath, "Data", "A1:C2"},
		{"ls", pptxPath, "slide/1"},
		{"ls", pptxPath, "slides"},
		{"info", xlsxPath},
	} {
		a := runJSON(t, append([]string{"--json"}, args...)...)
		b := runJSON(t, append([]string{"--json"}, args...)...)
		if a != b {
			t.Errorf("%v is not deterministic:\n%s\n%s", args, a, b)
		}
	}
}

func TestExitCodesAndHelp(t *testing.T) {
	out, _, code := runCLI()
	if code != 0 || !strings.Contains(out, "spine info") {
		t.Errorf("no arguments: exit %d", code)
	}
	out, _, code = runCLI("help")
	if code != 0 || !strings.Contains(out, "spine cells") {
		t.Errorf("help: exit %d", code)
	}
	_, _, code = runCLI("bogus")
	if code != 2 {
		t.Errorf("unknown command: exit %d", code)
	}
	_, _, code = runCLI("info")
	if code != 2 {
		t.Errorf("info without a file: exit %d", code)
	}
	_, _, code = runCLI("cells", xlsxPath, "Data", "A1", "--bogus")
	if code != 2 {
		t.Errorf("unknown flag: exit %d", code)
	}
	out, _, code = runCLI("info", "--help")
	if code != 0 {
		t.Errorf("--help: exit %d", code)
	}
}

func TestJSONErrorEnvelope(t *testing.T) {
	_, errOut, code := runCLI("--json", "info", "missing.xlsx")
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	want := `{"error":"file not found: missing.xlsx","code":"not_found"}` + "\n"
	if errOut != want {
		t.Errorf("error envelope\n got: %q\nwant: %q", errOut, want)
	}
}

func TestInternalErrorCode(t *testing.T) {
	// guard is the production panic path; a recovered panic is reported as
	// internal with exit 1.
	var errb bytes.Buffer
	code := guard(true, &errb, func() int { panic("boom") })
	if code != 1 {
		t.Errorf("guard exit %d", code)
	}
	if !strings.Contains(errb.String(), `"code":"internal"`) || !strings.Contains(errb.String(), "boom") {
		t.Errorf("guard stderr: %s", errb.String())
	}
}

func TestTextPreviewTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	got := textPreview(long)
	if len([]rune(got)) != 81 || !strings.HasSuffix(got, "…") {
		t.Errorf("preview of 100 runes: %d runes, suffix %q", len([]rune(got)), got[len(got)-3:])
	}
	if got := textPreview("x\ny"); got != "x y" {
		t.Errorf("newlines: %q", got)
	}
}

func TestEncryptedPasswordOnEveryCommand(t *testing.T) {
	// Every command opens an encrypted file with --password; a wrong password
	// is unsupported everywhere.
	type encCase struct {
		args     []string
		contains string
	}
	cases := []encCase{
		{[]string{"info", encXlsxPath}, `"kind": "xlsx"`},
		{[]string{"ls", encXlsxPath, "sheets"}, `"name": "Data"`},
		{[]string{"text", encXlsxPath}, "Revenue"},
		{[]string{"cells", encXlsxPath, "Data", "A1"}, `"value": "Revenue"`},
		{[]string{"info", encPath}, `"kind": "pptx"`},
		{[]string{"ls", encPath, "slides"}, `"name": "Title Slide"`},
		{[]string{"text", encPath}, "A text box"},
	}
	for _, tc := range cases {
		// Without a password the open itself fails.
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
			t.Errorf("%v without password: exit %d, stderr %s", tc.args, code, errOut)
		}
		// A wrong password is unsupported too.
		wrongArgs := append(append([]string{"--json"}, tc.args...), "--password", "wrong")
		_, errOut, code = runCLI(wrongArgs...)
		if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
			t.Errorf("%v with wrong password: exit %d, stderr %s", tc.args, code, errOut)
		}
		// The right password succeeds.
		rightArgs := append(append([]string{"--json"}, tc.args...), "--password", "s3cret")
		out, _, code := runCLI(rightArgs...)
		if code != 0 || !strings.Contains(out, tc.contains) {
			t.Errorf("%v with password: exit %d, stdout %s", tc.args, code, out)
		}
	}
	// set on an encrypted workbook round-trips and re-encrypts.
	path := copyFixture(t, encXlsxPath)
	runJSON(t, "set", path, "--password", "s3cret", "Data!F1", "secret")
	_, errOut, code := runCLI("--json", "cells", path, "Data", "F1:F1", "--password", "wrong")
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Errorf("re-encrypted xlsx with wrong password: exit %d, stderr %s", code, errOut)
	}
	if got := runJSON(t, "cells", path, "--password", "s3cret", "Data", "F1:F1"); !strings.Contains(got, `"value": "secret"`) {
		t.Errorf("encrypted set round trip: %s", got)
	}
}

func TestSheetNameExactMatch(t *testing.T) {
	// "data" must not match the sheet "Data": not_found, with the closest
	// name mentioned diagnostically.
	for _, tc := range []struct {
		args []string
	}{
		{[]string{"cells", xlsxPath, "data", "A1"}},
		{[]string{"cells", xlsxPath, "DATA", "A1"}},
		{[]string{"text", xlsxPath, "--sheet", "data"}},
		{[]string{"set", xlsxPath, "data!A1", "x"}},
		{[]string{"set", xlsxPath, "DATA!A1", "x"}},
	} {
		_, errOut, code := runCLI(append([]string{"--json"}, tc.args...)...)
		if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) {
			t.Errorf("%v: exit %d, stderr %s, want not_found", tc.args, code, errOut)
		}
		if !strings.Contains(errOut, "closest is") {
			t.Errorf("%v: stderr %s misses the closest-name hint \"Data\"", tc.args, errOut)
		}
	}
	// The exact name still works everywhere.
	runJSON(t, "cells", xlsxPath, "Data", "A1")
	runJSON(t, "text", xlsxPath, "--sheet", "Data")
	runJSON(t, "set", copyFixture(t, xlsxPath), "Data!F1", "ok")
}

func TestSavePreservesFileMode(t *testing.T) {
	for _, mode := range []os.FileMode{0o644, 0o600} {
		path := copyFixture(t, xlsxPath)
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		runJSON(t, "set", path, "Data!F1", "mode")
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != mode {
			t.Errorf("mode 0%o became 0%o after set", mode, got)
		}
	}
	// A newly created file (no destination mode to preserve) gets 0644.
	dst := filepath.Join(t.TempDir(), "new.xlsx")
	runJSON(t, "set", xlsxPath, "--out", dst, "Data!F1", "new")
	if fi, err := os.Stat(dst); err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("new file mode: %v, err %v", fi, err)
	}
}

func TestTextSlideFlagErrors(t *testing.T) {
	for _, n := range []string{"0", "-1", "-99"} {
		_, errOut, code := runCLI("text", pptxPath, "--slide", n)
		if code != 2 {
			t.Errorf("text --slide %s: exit %d, stderr %s, want flag error exit 2", n, code, errOut)
		}
	}
	// slide 1 and above still work.
	if _, _, code := runCLI("text", pptxPath, "--slide", "1"); code != 0 {
		t.Errorf("text --slide 1: exit %d", code)
	}
}

func TestCellsPptxMessage(t *testing.T) {
	_, errOut, code := runCLI("--json", "cells", pptxPath, "x", "A1")
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) {
		t.Fatalf("cells on pptx: exit %d, stderr %s", code, errOut)
	}
	if !strings.Contains(errOut, "cells applies only to xlsx files") {
		t.Errorf("cells on pptx stderr: %s", errOut)
	}
}

// ---- negative values and the "--" terminator ----

func TestSetNegativeNumbersAfterDashDash(t *testing.T) {
	path := copyFixture(t, xlsxPath)
	// Inference: "-5" and "-0.5" are numbers when they follow "--".
	out := runJSON(t, "set", path, "Data!G1", "--", "-5")
	if out != "{\n  \"selector\": \"Data!G1\",\n  \"type\": \"number\",\n  \"value\": -5\n}\n" {
		t.Errorf("set -5: %s", out)
	}
	out = runJSON(t, "set", path, "Data!G2", "--", "-0.5")
	if !strings.Contains(out, `"value": -0.5`) {
		t.Errorf("set -0.5: %s", out)
	}
	// Forced type, with flags before the terminator.
	out = runJSON(t, "set", path, "Data!G3", "--type", "number", "--", "-5")
	if !strings.Contains(out, `"value": -5`) {
		t.Errorf("set --type number -- -5: %s", out)
	}
	// The values round-trip as numbers.
	cells := runJSON(t, "cells", path, "Data", "G1:G3")
	for _, want := range []string{`"value": -5`, `"value": -0.5`} {
		if !strings.Contains(cells, want) {
			t.Errorf("round trip missing %s in:\n%s", want, cells)
		}
	}
	// Without the terminator, "-5" is still treated as a value (negative
	// numbers need no "--"): tokens like "-5" and "-0.5" are positional
	// because no flag name in this CLI begins with a digit.
	out = runJSON(t, "set", path, "Data!G5", "-5")
	if !strings.Contains(out, `"value": -5`) {
		t.Errorf("set -5 without --: %s", out)
	}
	// Everything after "--" is positional, even flag-looking tokens: this is
	// a three-argument set with the string value "-5" typed as string.
	out = runJSON(t, "set", path, "Data!G4", "--type", "string", "--", "-5")
	if !strings.Contains(out, `"type": "string"`) || !strings.Contains(out, `"value": "-5"`) {
		t.Errorf("set --type string -- -5: %s", out)
	}
}

// ---- password has no effect on plaintext files ----

func TestPasswordOnPlaintextStaysPlaintext(t *testing.T) {
	path := copyFixture(t, xlsxPath)
	// Editing a plaintext file with --password (even a bogus one) must not
	// encrypt the output: the file still opens without any password.
	runJSON(t, "set", path, "--password", "bogus", "Data!F1", "plain")
	if got := runJSON(t, "cells", path, "Data", "F1"); !strings.Contains(got, `"value": "plain"`) {
		t.Errorf("plaintext set with --password not readable without it: %s", got)
	}
	if info := runJSON(t, "info", path); !strings.Contains(info, `"kind": "xlsx"`) {
		t.Errorf("info on the plaintext file failed: %s", info)
	}
	// A wrong password on a genuinely encrypted file is unsupported with the
	// exact "wrong password" message.
	_, errOut, code := runCLI("--json", "info", encPath, "--password", "wrong")
	if code != 1 || !strings.Contains(errOut, `"code":"unsupported"`) || !strings.Contains(errOut, "wrong password") {
		t.Errorf("wrong password on encrypted file: exit %d, stderr %s", code, errOut)
	}
	// The encrypted round trip still re-encrypts (covered in detail by
	// TestSetEncryptedRoundTrip); here: the right password works.
	out, _, code := runCLI("--json", "info", encPath, "--password", "s3cret")
	if code != 0 || !strings.Contains(out, `"kind": "pptx"`) {
		t.Errorf("encrypted file with right password: exit %d", code)
	}
}

// ---- ls slide/<N> text_preview truncation ----

func TestLsSlideShapePreviewTruncation(t *testing.T) {
	p := pptx.CreateWidescreen()
	s := p.AddSlide()
	tb := s.AddTextBox()
	long := strings.Repeat("ab\ncd", 20) // 100 runes, multi-line
	tb.TextFrame().SetText(long)
	path := filepath.Join(t.TempDir(), "long.pptx")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	var entries []shapeEntry
	if err := json.Unmarshal([]byte(runJSON(t, "ls", path, "slide/1")), &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("shape entries: %d", len(entries))
	}
	got := entries[0].TextPreview
	runes := []rune(got)
	if len(runes) != 81 || !strings.HasSuffix(got, "…") {
		t.Errorf("preview of 100 runes = %d runes, want 80 + ellipsis: %q", len(runes), got)
	}
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("preview keeps newlines: %q", got)
	}
}

// ---- info on an empty pptx ----

func TestInfoEmptyPPTXSlidesArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.pptx")
	runJSON(t, "new", "pptx", path)
	out := runJSON(t, "info", path)
	if !strings.Contains(out, `"slides": []`) {
		t.Errorf("empty pptx info does not emit an empty slides array: %s", out)
	}
}

// ---- doubled '' in a quoted sheet name ----

func TestDoubledQuoteSheetNameNotFound(t *testing.T) {
	// 'a''b'!A1 contains a doubled quote inside the quoted name; such names
	// select nothing and are not_found, not a selector parse error.
	_, errOut, code := runCLI("--json", "set", xlsxPath, "'a''b'!A1", "x")
	if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) || !strings.Contains(errOut, "sheet not found") {
		t.Errorf("set 'a''b'!A1: exit %d, stderr %s", code, errOut)
	}
	// The same through the operations vocabulary.
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, `{"version":1,"ops":[{"op":"set_cell","selector":"'a''b'!A1","value":1}]}`)
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("edit exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "not_found" {
		t.Errorf("edit 'a''b'!A1: code %q, want not_found (error %q)", e.Code, e.Error)
	}
	// A normal quoted name still works.
	if got := runJSON(t, "set", xlsxPath, "'Data'!F2", "ok"); !strings.Contains(got, `"value": "ok"`) {
		t.Errorf("quoted sheet name broke: %s", got)
	}
}

// ---- non-finite numbers ----

func TestSetRejectsNonFiniteNumbers(t *testing.T) {
	// Values beginning with "-" need the "--" terminator (standard flag
	// semantics); the rest are plain positionals.
	for _, tc := range []struct{ dashdash, value string }{
		{"--", "-Inf"},
		{"--", "-inf"},
		{"", "NaN"},
		{"", "nan"},
		{"", "Inf"},
		{"", "+Inf"},
		{"", "inf"},
	} {
		args := []string{"--json", "set", xlsxPath, "Data!H1"}
		if tc.dashdash != "" {
			args = append(args, tc.dashdash)
		}
		args = append(args, tc.value)
		_, errOut, code := runCLI(args...)
		if code != 1 || !strings.Contains(errOut, `"code":"invalid_value"`) || !strings.Contains(errOut, "is not a finite number") {
			t.Errorf("set %q: exit %d, stderr %s", tc.value, code, errOut)
		}
		typed := []string{"--json", "set", xlsxPath, "Data!H1", "--type", "number"}
		if tc.dashdash != "" {
			typed = append(typed, tc.dashdash)
		}
		typed = append(typed, tc.value)
		_, errOut, code = runCLI(typed...)
		if code != 1 || !strings.Contains(errOut, "is not a finite number") {
			t.Errorf("set --type number %q: exit %d, stderr %s", tc.value, code, errOut)
		}
	}
	// set_cell rejects them too (they can only arrive as strings, since JSON
	// cannot encode NaN/Inf).
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, opsDoc(t, op("set_cell", "selector", "Data!H1", "value", "NaN")))
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("edit exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" || !strings.Contains(e.Error, "is not a finite number") {
		t.Errorf("set_cell NaN: code %q, error %q", e.Code, e.Error)
	}
}

// ---- --password is rejected where no document is opened ----

func TestPasswordFlagRejectedWithoutDocument(t *testing.T) {
	dir := t.TempDir()
	if _, _, code := runCLI("new", "xlsx", filepath.Join(dir, "n.xlsx"), "--password", "x"); code != 2 {
		t.Errorf("new --password: exit %d, want 2", code)
	}
	if _, _, code := runCLI("ops-schema", "--password", "x"); code != 2 {
		t.Errorf("ops-schema --password: exit %d, want 2", code)
	}
}

// ---- explicitly empty --sheet ----

func TestTextEmptySheetFlag(t *testing.T) {
	// --sheet "" is distinct from the flag being absent: not_found.
	_, errOut, code := runCLI("--json", "text", xlsxPath, "--sheet", "")
	if code != 1 || !strings.Contains(errOut, `"code":"not_found"`) {
		t.Fatalf("text --sheet \"\": exit %d, stderr %s", code, errOut)
	}
	if !strings.Contains(errOut, `sheet not found: \"\"`) {
		t.Errorf("text --sheet \"\" message: %s", errOut)
	}
	// The flag absent still reads the whole workbook.
	if out, _, code := runCLI("text", xlsxPath); code != 0 || !strings.Contains(out, "Revenue") {
		t.Errorf("text without --sheet: exit %d", code)
	}
	// --sheet "" still conflicts with --slide.
	if _, _, code := runCLI("text", xlsxPath, "--sheet", "", "--slide", "1"); code != 2 {
		t.Errorf("--sheet \"\" with --slide: exit %d, want 2", code)
	}
}

// ---- set_properties with JSON null fields ----

func TestEditSetPropertiesNullField(t *testing.T) {
	f := copyFixture(t, xlsxPath)
	ops := writeOpsFile(t, `{"version":1,"ops":[{"op":"set_properties","title":null}]}`)
	_, errOut, code := runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	e := parseErrJSON(t, errOut)
	if e.Code != "invalid_value" || !strings.Contains(e.Error, `field "title" must be a string`) {
		t.Errorf("null title: code %q, error %q", e.Code, e.Error)
	}
	// add_sheet's optional name rejects null the same way.
	ops = writeOpsFile(t, `{"version":1,"ops":[{"op":"add_sheet","name":null}]}`)
	_, errOut, code = runCLI("--json", "edit", f, ops)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if e := parseErrJSON(t, errOut); e.Code != "invalid_value" || !strings.Contains(e.Error, `field "name" must be a string`) {
		t.Errorf("null name: code %q, error %q", e.Code, e.Error)
	}
}

// ---- cells range size cap ----

func TestCellsRangeTooLarge(t *testing.T) {
	_, errOut, code := runCLI("--json", "cells", xlsxPath, "Data", "A1:GR60000")
	if code != 1 || !strings.Contains(errOut, `"code":"invalid_range"`) {
		t.Fatalf("200x60000 range: exit %d, stderr %s", code, errOut)
	}
	if !strings.Contains(errOut, "range A1:GR60000 covers 12,000,000 cells; the limit is 10,000,000") {
		t.Errorf("range cap message: %s", errOut)
	}
	// The full worksheet grid is far over the cap too.
	_, errOut, code = runCLI("--json", "cells", xlsxPath, "Data", "A1:XFD1048576")
	if code != 1 || !strings.Contains(errOut, "covers 17,179,869,184 cells; the limit is 10,000,000") {
		t.Errorf("full grid message: exit %d, stderr %s", code, errOut)
	}
}
