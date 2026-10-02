package xlsx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/render"
)

func renderTestSheet(t testing.TB) (*Workbook, *Sheet, render.Options) {
	t.Helper()
	w := Create()
	s, err := w.AddSheet("Preview")
	if err != nil {
		t.Fatal(err)
	}
	for c := 1; c <= 2; c++ {
		if err = s.SetColWidth(c, 8.7109375); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SetCellValue("A1", "AA"); err != nil {
		t.Fatal(err)
	}
	if err = s.SetCellValue("A2", 12); err != nil {
		t.Fatal(err)
	}
	glyphs := []fonttest.Glyph{{Rune: 'A', Advance: 1000, HasShape: true, Ink: [4]int{0, 0, 1000, 800}}}
	for c := '0'; c <= '9'; c++ {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 500, HasShape: true, Ink: [4]int{0, 0, 400, 800}})
	}
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
		if r.Family != "Calibri" || r.Bold || r.Italic {
			t.Fatalf("font: %+v", r)
		}
		return f, nil
	}}
	return w, s, opts
}

func TestRenderSheetRangePixelsSourceAndSnapshot(t *testing.T) {
	w, s, opts := renderTestSheet(t)
	before, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.PrepareRender(context.Background(), "A1:B2", opts)
	if err != nil {
		t.Fatal(err)
	}
	width, height, err := page.Size(96)
	if err != nil || width != 122 || height != 40 {
		t.Fatalf("size %d,%d: %v", width, height, err)
	}
	if s.FindCell("B1") != nil || s.FindCell("B2") != nil {
		t.Fatal("missing cells created")
	}
	after, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unzipParts(t, before), unzipParts(t, after)) {
		t.Fatal("render changed source")
	}
	if err = s.SetCellValue("A1", ""); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x, y int
		want color.NRGBA
	}{{3, 5, color.NRGBA{A: 255}}, {35, 5, color.NRGBA{R: 255, G: 255, B: 255, A: 255}}, {61, 5, color.NRGBA{R: 210, G: 210, B: 210, A: 255}}, {45, 25, color.NRGBA{A: 255}}, {3, 25, color.NRGBA{R: 255, G: 255, B: 255, A: 255}}} {
		got := color.NRGBAModel.Convert(img.At(tc.x, tc.y)).(color.NRGBA)
		if got != tc.want {
			t.Fatalf("pixel %d,%d: %+v != %+v", tc.x, tc.y, got, tc.want)
		}
	}
	opened, err := OpenReader(bytes.NewReader(before), int64(len(before)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = opened.Sheets()[0].PrepareRender(context.Background(), "A1:B2", opts); err != nil {
		t.Fatal(err)
	}
}

func TestRenderSheetRejectsHugeRangeAndUnsupportedValues(t *testing.T) {
	_, s, opts := renderTestSheet(t)
	noCallback := opts
	noCallback.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) {
		t.Fatal("resolved font before range check")
		return nil, nil
	}
	if page, err := s.PrepareRender(context.Background(), "A1:XFD1048576", noCallback); page != nil || !errors.Is(err, render.ErrLimit) {
		t.Fatalf("range: %v %v", page, err)
	}
	if err := s.SetCellValue("A1", 1.5); err != nil {
		t.Fatal(err)
	}
	if page, err := s.PrepareRender(context.Background(), "A1:B2", opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("decimal: %v %v", page, err)
	}
	if err := s.SetCellValue("A1", "AA"); err != nil {
		t.Fatal(err)
	}
	opts.Limits.MaxShapeWork = 1
	if page, err := s.PrepareRender(context.Background(), "A1:B2", opts); page != nil || !errors.Is(err, render.ErrLimit) {
		t.Fatalf("work: %v %v", page, err)
	}
}

func TestRenderSheetOriginalUnknownContentRejectedBeforeFonts(t *testing.T) {
	w, _, opts := renderTestSheet(t)
	data, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	part := unzipParts(t, data)["xl/worksheets/sheet1.xml"]
	rewritten := strings.Replace(string(part), "</sheetData>", "</sheetData><unknownVisible/>", 1)
	if rewritten == string(part) {
		t.Fatal("fixture unchanged")
	}
	modified := replaceZipEntry(t, data, "xl/worksheets/sheet1.xml", rewritten)
	opened, err := OpenReader(bytes.NewReader(modified), int64(len(modified)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) {
		t.Fatal("font callback before source check")
		return nil, nil
	}
	if page, err := opened.Sheets()[0].PrepareRender(context.Background(), "A1:B2", opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unknown source: %v %v", page, err)
	}
}

func FuzzXlsxRender(f *testing.F) {
	seed, _, prototype := renderTestSheet(f)
	valid, e := seed.SaveBytes()
	if e != nil {
		f.Fatal(e)
	}
	face, e := prototype.Fonts(context.Background(), render.FontRequest{Family: "Calibri"})
	if e != nil {
		f.Fatal(e)
	}
	const part = "xl/worksheets/sheet1.xml"
	f.Add(fuzzseed.ZipEntry(valid, part))
	f.Add([]byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><cols><col min="1" max="2" width="8.7109375"/></cols><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>AA</t></is></c></row></sheetData></worksheet>`))
	f.Add([]byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="4294967295"/></sheetData></worksheet>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		wrapped := fuzzseed.ReplaceZipEntry(valid, part, data)
		if wrapped == nil {
			t.Fatal("seed")
		}
		w, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() {
			if e := w.Close(); e != nil {
				t.Error(e)
			}
		}()
		if len(w.Sheets()) == 0 {
			return
		}
		opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }}
		opts.MaxSourceBytes = 65536
		opts.MaxLayoutNodes = 256
		opts.Limits = render.Limits{MaxDimension: 256, MaxPixels: 65536, MaxOperations: 128, MaxGlyphs: 256, MaxShapeWork: 65536, MaxOutputBytes: 65536, MaxTextBytes: 1024, MaxRunBytes: 1024}
		page, err := w.Sheets()[0].PrepareRender(context.Background(), "A1:B2", opts)
		if err != nil {
			if page != nil {
				t.Fatal("partial page")
			}
			return
		}
		var out bytes.Buffer
		if err = page.WritePNG(context.Background(), &out, 96); err != nil && !errors.Is(err, render.ErrLimit) {
			t.Fatal(err)
		}
	})
}
