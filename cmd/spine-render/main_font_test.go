package main

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/forme/font"
	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// testFont is a font of a letter A whose advance and name are as given.
func testFont(name string, advance int, extra map[string][]byte) []byte {
	return fonttest.SFNT(fonttest.SFNTOptions{Name: name, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: advance, HasShape: true}}, Extra: extra})
}

// buildCollection writes a TrueType collection of fonts, each table stored once
// however many fonts carry it, as collection tools do.
func buildCollection(fonts ...[]byte) []byte {
	type entry struct {
		tag  string
		data []byte
	}
	dirs := make([][]entry, len(fonts))
	for i, data := range fonts {
		for tag, t := range font.SFNTTables(data) {
			dirs[i] = append(dirs[i], entry{tag, t})
		}
		sort.Slice(dirs[i], func(a, b int) bool { return dirs[i][a].tag < dirs[i][b].tag })
	}
	at := 12 + 4*len(fonts)
	dirAt := make([]int, len(fonts))
	for i := range fonts {
		dirAt[i] = at
		at += 12 + 16*len(dirs[i])
	}
	var blobs [][]byte
	offsets := map[string]int{}
	offsetOf := func(t []byte) int {
		if off, ok := offsets[string(t)]; ok {
			return off
		}
		at = (at + 3) &^ 3
		offsets[string(t)] = at
		blobs = append(blobs, t)
		at += len(t)
		return offsets[string(t)]
	}
	out := make([]byte, 12+4*len(fonts))
	copy(out, "ttcf")
	binary.BigEndian.PutUint32(out[4:], 0x00010000)
	binary.BigEndian.PutUint32(out[8:], uint32(len(fonts)))
	var records []byte
	for i, d := range dirs {
		binary.BigEndian.PutUint32(out[12+4*i:], uint32(dirAt[i]))
		head := make([]byte, 12)
		binary.BigEndian.PutUint32(head, 0x00010000)
		binary.BigEndian.PutUint16(head[4:], uint16(len(d)))
		records = append(records, head...)
		for _, e := range d {
			rec := make([]byte, 16)
			copy(rec, e.tag)
			binary.BigEndian.PutUint32(rec[8:], uint32(offsetOf(e.data)))
			binary.BigEndian.PutUint32(rec[12:], uint32(len(e.data)))
			records = append(records, rec...)
		}
	}
	out = append(out, records...)
	for _, b := range blobs {
		for len(out) < offsets[string(b)] {
			out = append(out, 0)
		}
		out = append(out, b...)
	}
	return out
}

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func fontFor(t *testing.T, mappings ...string) (render.FontResolver, []fontFile, error) {
	t.Helper()
	return resolver(config{fonts: fontFlags(mappings)})
}

func resolve(t *testing.T, fonts render.FontResolver, family string) *shape.Face {
	t.Helper()
	face, err := fonts(context.Background(), render.FontRequest{Family: family})
	if err != nil {
		t.Fatal(err)
	}
	return face
}

func TestFontCollectionFaces(t *testing.T) {
	dir := t.TempDir()
	collection := writeFile(t, filepath.Join(dir, "pair.ttc"), buildCollection(testFont("First", 1000, nil), testFont("Second", 500, nil)))
	single := writeFile(t, filepath.Join(dir, "single.ttf"), testFont("Single", 700, nil))

	// FILE#INDEX picks the face, by its index from zero.
	fonts, files, err := fontFor(t, "Wide="+collection+"#0", "Narrow="+collection+"#1", "Plain="+single)
	if err != nil {
		t.Fatal(err)
	}
	for family, name := range map[string]string{"Wide": "First", "Narrow": "Second", "Plain": "Single"} {
		if got := resolve(t, fonts, family).Name(); got != name {
			t.Fatalf("%s resolved to %q, want %q", family, got, name)
		}
	}
	// Styles pick faces of one family from one collection.
	fonts, _, err = resolver(config{fonts: fontFlags{"Fam=" + collection + "#0", "Fam:bold=" + collection + "#1"}})
	if err != nil {
		t.Fatal(err)
	}
	bold, err := fonts(context.Background(), render.FontRequest{Family: "Fam", Bold: true})
	if err != nil || bold.Name() != "Second" || resolve(t, fonts, "Fam").Name() != "First" {
		t.Fatalf("styles: %v", err)
	}
	// A plain font is its own face zero.
	if _, _, err = fontFor(t, "Plain="+single+"#0"); err != nil {
		t.Fatal(err)
	}
	// Charts are given fonts of their own: a face of a collection is copied
	// out, and a font file is given as it is.
	if files[0].face == nil || files[2].face != nil || files[2].program() == nil {
		t.Fatalf("files: %+v", files)
	}
	if got, err := shape.Load(files[1].program()); err != nil || got.Name() != "Second" {
		t.Fatalf("the face copied out of its collection: %v", err)
	}

	for _, tc := range []struct{ name, mapping, want string }{
		// Which face of several is never a guess.
		{"several faces", "F=" + collection, "holds 2 faces; choose one with FILE#INDEX: #0 "},
		{"face past the end", "F=" + collection + "#2", "2 fonts"},
		{"face of a single font", "F=" + single + "#1", "one face"},
		{"huge index", "F=" + collection + "#99999999999999999999", "invalid font face index"},
		{"no file", "F=#1", "invalid font face index"},
		{"missing file", "F=" + filepath.Join(dir, "none.ttc") + "#0", "no such file"},
	} {
		if _, _, err := fontFor(t, tc.mapping); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	// The listing names the faces.
	_, _, err = fontFor(t, "F="+collection)
	for _, want := range []string{"#0 ", "(First)", "#1 ", "(Second)"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("listing: %v lacks %q", err, want)
		}
	}
	// A duplicate family still fails, and a font that is not one says so.
	if _, _, err = fontFor(t, "F="+collection+"#0", "F="+collection+"#1"); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate: %v", err)
	}
	junk := writeFile(t, filepath.Join(dir, "junk.ttc"), []byte("ttcf not a font"))
	if _, _, err = fontFor(t, "F="+junk+"#0"); err == nil || !strings.Contains(err.Error(), `font "F"`) {
		t.Fatalf("junk: %v", err)
	}
}

// The index is the digits after the file name's last '#', and nothing else is.
func TestFontIndexSyntax(t *testing.T) {
	for _, tc := range []struct {
		location, path string
		index          int
		indexed        bool
	}{
		{"a.ttc", "a.ttc", 0, false},
		{"a.ttc#0", "a.ttc", 0, true},
		{"a.ttc#12", "a.ttc", 12, true},
		{"a#b.ttf", "a#b.ttf", 0, false},
		{"a#.ttf#", "a#.ttf#", 0, false},
		{"a#2.ttf", "a#2.ttf", 0, false},
		{"dir/a#2#0", "dir/a#2", 0, true},
		{"a#3#4", "a#3", 4, true},
		{"a#-1", "a#-1", 0, false},
	} {
		path, index, indexed, err := splitFontIndex(tc.location)
		if err != nil || path != tc.path || index != tc.index || indexed != tc.indexed {
			t.Fatalf("%q: %q %d %v %v, want %q %d %v", tc.location, path, index, indexed, err, tc.path, tc.index, tc.indexed)
		}
	}
	// A directory or file named with a '#' and digits loads with its index.
	dir := filepath.Join(t.TempDir(), "fonts#7")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := writeFile(t, filepath.Join(dir, "a#12"), testFont("Odd", 600, nil))
	if _, _, err := fontFor(t, "F="+file); err == nil {
		t.Fatal("'#12' was taken for the file's name")
	}
	if _, _, err := fontFor(t, "F="+file+"#0"); err != nil {
		t.Fatal(err)
	}
}

// A font collection is read and counted once, however many of its faces are
// mapped, and two files with the same bytes are two files.
func TestFontCollectionBudget(t *testing.T) {
	dir := t.TempDir()
	// A 20 MiB table that two faces share: the file is 20 MiB, and a face
	// copied out of it is as large again.
	big := map[string][]byte{"ZZZZ": make([]byte, 20<<20)}
	data := buildCollection(testFont("First", 1000, big), testFont("Second", 500, big))
	collection := writeFile(t, filepath.Join(dir, "big.ttc"), data)
	copied := writeFile(t, filepath.Join(dir, "copy.ttc"), data)
	if len(data) > 32<<20 || len(data) < 20<<20 {
		t.Fatalf("collection of %d bytes", len(data))
	}
	_, files, err := fontFor(t, "A="+collection+"#0", "B="+collection+"#1", "C:bold="+filepath.Join(dir, ".", "big.ttc")+"#1")
	if err != nil {
		t.Fatalf("one file mapped three times: %v", err)
	}
	if len(files) != 3 || &files[0].data[0] != &files[1].data[0] {
		t.Fatal("the collection was read more than once")
	}
	if _, _, err = fontFor(t, "A="+collection+"#0", "B="+copied+"#1"); err == nil || !strings.Contains(err.Error(), "32 MiB") {
		t.Fatalf("two files: %v", err)
	}
	// The faces copied out for charts are bounded as well: each is 20 MiB.
	if _, closeCharts, err := chartRenderer(files, time.Minute); err == nil {
		_ = closeCharts()
		t.Fatal("copies of a 20 MiB collection's faces, three of them, were not bounded")
	} else if !strings.Contains(err.Error(), "copied out of font collections") {
		t.Fatalf("charts: %v", err)
	}
}

// A chart is drawn with a face of a collection, which aster is given as a font
// of its own.
func TestChartsUseFontCollectionFaces(t *testing.T) {
	dir := t.TempDir()
	collection := writeFile(t, filepath.Join(dir, "pair.ttc"), buildCollection(testFont("First", 1000, nil), testFont("Second", 500, nil)))
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	c := chart.NewColumn()
	c.SetCategories([]string{"A", "B"})
	c.AddSeries("S", []float64{3, 5}).SetColor("FF0000")
	if err = p.AddSlideFromLayout(layout).AddChart(c, int64(dml.Pixels(100)), int64(dml.Pixels(100)), int64(dml.Pixels(400)), int64(dml.Pixels(300))); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	var warnings strings.Builder
	cfg := config{input: input, out: filepath.Join(dir, "out"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, warn: &warnings, charts: true,
		fonts: fontFlags{"Calibri=" + collection + "#1", "Arial=" + collection + "#0"}}
	if err = run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(cfg.out, "slide-0001.png")); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(warnings.String(), "chart") {
		t.Fatalf("warnings: %q", warnings.String())
	}
}
