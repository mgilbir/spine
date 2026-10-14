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
	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/internal/renderfonts"
	"github.com/mgilbir/spine/pptx"
)

// testFont is a font of a letter A whose advance and name are as given.
func testFont(name string, advance int, extra map[string][]byte) []byte {
	return fonttest.SFNT(fonttest.SFNTOptions{Name: name, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: advance, HasShape: true}}, Extra: extra})
}

// buildCollection writes a TrueType collection of fonts, each table stored
// once however many fonts carry it, as collection tools do. The same builder
// lives in the renderfonts tests; it is test-only, so it is duplicated here
// for the chart budget tests.
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

func TestFontCollectionBudget(t *testing.T) {
	dir := t.TempDir()
	// A 20 MiB table that two faces share: a face copied out of it is as
	// large again.
	big := map[string][]byte{"ZZZZ": make([]byte, 20<<20)}
	data := buildCollection(testFont("First", 1000, big), testFont("Second", 500, big))
	collection := filepath.Join(dir, "big.ttc")
	if err := os.WriteFile(collection, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, files, err := renderfonts.Resolver([]string{"A=" + collection + "#0", "B=" + collection + "#1", "C:bold=" + collection + "#1"}, false)
	if err != nil {
		t.Fatal(err)
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
	collectionPath := filepath.Join(dir, "pair.ttc")
	collection := buildCollection(testFont("First", 1000, nil), testFont("Second", 500, nil))
	if err := os.WriteFile(collectionPath, collection, 0o600); err != nil {
		t.Fatal(err)
	}
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
		fonts: renderfonts.Flags{"Calibri=" + collectionPath + "#1", "Arial=" + collectionPath + "#0"}}
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
