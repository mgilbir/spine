package render

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

func smallPage(t testing.TB, limits Limits) *Page {
	t.Helper()
	ops := []layout.Op{
		layout.FillRect{Rect: layout.Rect{W: unit(2), H: unit(1)}, Color: style.RGBA{R: 255, A: 1}},
		layout.FillRect{Rect: layout.Rect{X: unit(1), W: unit(1), H: unit(1)}, Color: style.RGBA{B: 255, A: 0.5}},
		layout.FillRect{Rect: layout.Rect{Y: unit(1), W: unit(0.5), H: unit(1)}, Color: style.RGBA{G: 255, A: 1}},
	}
	p, err := Prepare(context.Background(), dml.Pixels(2), dml.Pixels(2), ops, limits)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPNGIndependentPixelExpectations(t *testing.T) {
	p := smallPage(t, Limits{})
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatal(img.Bounds())
	}
	// Independent source-over calculation: opaque red then half blue; half
	// coverage green remains green in straight-alpha form, with half alpha.
	want := []color.NRGBA{{R: 255, A: 255}, {R: 128, B: 128, A: 255}, {G: 255, A: 128}, {}}
	for i, c := range want {
		got := color.NRGBAModel.Convert(img.At(i%2, i/2)).(color.NRGBA)
		if got != c {
			t.Fatalf("pixel %d: %+v, want %+v", i, got, c)
		}
	}
	var scaled bytes.Buffer
	if err := p.WritePNG(context.Background(), &scaled, 192); err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(&scaled)
	if err != nil || config.Width != 4 || config.Height != 4 {
		t.Fatalf("scaled: %+v %v", config, err)
	}
}

func TestSVGGeneratedGeometry(t *testing.T) {
	p := smallPage(t, Limits{})
	var b bytes.Buffer
	if err := p.WriteSVG(context.Background(), &b, 192); err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(&b)
	var starts []xml.StartElement
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if s, ok := token.(xml.StartElement); ok {
			starts = append(starts, s)
		}
	}
	if len(starts) != 4 || starts[0].Name.Space != "http://www.w3.org/2000/svg" {
		t.Fatalf("SVG: %+v", starts)
	}
	attrs := func(s xml.StartElement) map[string]string {
		m := map[string]string{}
		for _, a := range s.Attr {
			m[a.Name.Local] = a.Value
		}
		return m
	}
	root := attrs(starts[0])
	if root["width"] != "4" || root["height"] != "4" || root["viewBox"] != "0 0 4 4" {
		t.Fatal(root)
	}
	r := attrs(starts[2])
	if starts[2].Name.Local != "rect" || r["x"] != "2" || r["width"] != "2" || r["fill"] != "rgb(0,0,255)" || r["fill-opacity"] != "0.5" {
		t.Fatal(r)
	}
	for _, s := range starts {
		if s.Name.Local != "svg" && s.Name.Local != "rect" {
			t.Fatal(s)
		}
		for _, a := range s.Attr {
			if strings.HasPrefix(a.Name.Local, "on") || a.Name.Local == "href" {
				t.Fatal(a)
			}
		}
	}
}

func TestPaintLimitsAndWriterFailures(t *testing.T) {
	for _, format := range []string{"png", "svg"} {
		t.Run(format, func(t *testing.T) {
			write := func(p *Page, ctx context.Context, w io.Writer) error {
				if format == "png" {
					return p.WritePNG(ctx, w, 96)
				}
				return p.WriteSVG(ctx, w, 96)
			}
			p := smallPage(t, Limits{MaxOutputBytes: 16})
			var b bytes.Buffer
			if err := write(p, context.Background(), &b); !errors.Is(err, ErrLimit) || b.Len() > 16 {
				t.Fatalf("byte limit: %v, bytes=%d", err, b.Len())
			}
			p = smallPage(t, Limits{})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			b.Reset()
			if err := write(p, ctx, &b); !errors.Is(err, context.Canceled) || b.Len() != 0 {
				t.Fatalf("cancel: %v", err)
			}
			if err := write(p, context.Background(), nil); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
			if err := write(p, context.Background(), shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
				t.Fatal(err)
			}
			failure := errors.New("output failure")
			if err := write(p, context.Background(), failedWriter{failure}); !errors.Is(err, failure) {
				t.Fatal(err)
			}
			ctx, cancel = context.WithCancel(context.Background())
			if err := write(p, ctx, cancelWriter{cancel}); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation during output: %v", err)
			}
		})
	}
	p := smallPage(t, Limits{MaxPixelVisits: 3}) // Three commands visit four pixels.
	var b bytes.Buffer
	if err := p.WritePNG(context.Background(), &b, 96); !errors.Is(err, ErrLimit) || b.Len() != 0 {
		t.Fatalf("work limit: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

type failedWriter struct{ err error }

func (w failedWriter) Write([]byte) (int, error) { return 0, w.err }

type cancelWriter struct{ cancel context.CancelFunc }

func (w cancelWriter) Write(b []byte) (int, error) { w.cancel(); return len(b), nil }

func TestConcurrentSnapshotRendering(t *testing.T) {
	p := smallPage(t, Limits{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			var pngOut, svgOut bytes.Buffer
			if err := p.WritePNG(context.Background(), &pngOut, 96); err != nil {
				t.Error(err)
			}
			if err := p.WriteSVG(context.Background(), &svgOut, 96); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}

func BenchmarkPNGRectangles(b *testing.B) {
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: unit(960), H: unit(540)}, Color: style.RGBA{R: 255, A: 1}}}
	p, err := Prepare(context.Background(), dml.Pixels(960), dml.Pixels(540), ops, Limits{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := p.WritePNG(context.Background(), io.Discard, 96); err != nil {
			b.Fatal(err)
		}
	}
}
