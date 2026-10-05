package render

import (
	"bytes"
	"context"
	"errors"
	"image"
	"strings"
	"sync"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
)

func TestColorGlyphLimits(t *testing.T) {
	// Layer lists of glyphs filling the page, and a bitmap of 50 by 50 pixels.
	var leaves []*paintNode
	for i := 0; i < 20; i++ {
		leaves = append(leaves, paintGlyph(gidFull, paintSolid(0, 0.5)))
	}
	nest := paintSolid(0, 1)
	for i := 0; i < 40; i++ {
		nest = paintGlyph(gidFull, nest)
	}
	var stops []colorStop
	for i := 0; i < 5000; i++ {
		stops = append(stops, colorStop{float64(i) / 5000, i % 3, 1})
	}
	// A color glyph of layers that are color glyphs of as many layers: far past
	// what forme paints.
	var explosion []*paintNode
	for i := 0; i < 255; i++ {
		explosion = append(explosion, paintLayers(0, 255))
	}
	face := colorFont(t, colorFace{
		layers: append(append([]*paintNode(nil), leaves...), explosion...),
		bases: []*paintNode{
			paintLayers(0, 20),
			nest,
			paintGlyph(gidFull, paintLinear(colorLine(0, stops...), 0, 0, 1000, 0, 0, 100)),
			paintLayers(20, 255),
			paintGlyph(gidFull, paintSolid(0, 1)),
		},
		bitmaps: []*image.NRGBA{quadrants(50)},
	})
	for _, tc := range []struct {
		name   string
		gid    int
		limits Limits
	}{
		{"operations", firstBase, Limits{MaxOperations: 10}},
		{"clip depth", firstBase + 1, Limits{}},
		{"clip depth, set low", firstBase + 4, Limits{MaxClipDepth: 0, MaxPathSegments: 3}},
		{"path segments", firstBase + 4, Limits{MaxPathSegments: 3}},
		{"gradient stops", firstBase + 2, Limits{MaxPathSegments: 1000}},
		{"paint graph", firstBase + 3, Limits{}},
		{"image pixels", firstBase + 5, Limits{MaxImagePixels: 100}},
		{"image bytes", firstBase + 5, Limits{MaxImageBytes: 100}},
	} {
		for _, approximate := range []func(error){nil, func(error) {}} {
			if _, err := prepareColor(face, black, tc.limits, approximate, tc.gid); !errors.Is(err, ErrLimit) {
				t.Fatalf("%s: %v", tc.name, err)
			}
		}
	}
	// Pixel work is bounded when the page is painted.
	p, err := prepareColor(face, black, Limits{MaxPixelVisits: 1000}, nil, firstBase)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Raster(context.Background(), 96); !errors.Is(err, ErrLimit) {
		t.Fatalf("visits: %v", err)
	}
	p, err = prepareColor(face, black, Limits{MaxEdgeChecks: 1000}, nil, firstBase)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Raster(context.Background(), 96); !errors.Is(err, ErrLimit) {
		t.Fatalf("edge checks: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PrepareBestEffort(ctx, dml.Pixels(colorPage), dml.Pixels(colorPage), colorOps(face, black, firstBase), Limits{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
}

// A page's glyph images are decoded once for each glyph, and count by their
// pixels rather than as pictures: more distinct glyphs than a page has pictures
// are fine, until their pixels are too many.
func TestColorGlyphImagesAreBoundedByPixels(t *testing.T) {
	var images []*image.NRGBA
	for i := 0; i < 40; i++ {
		images = append(images, quadrants(8))
	}
	face := colorFont(t, colorFace{sbix: true, bitmaps: images})
	var gids []int
	for i := 0; i < 40; i++ {
		gids = append(gids, firstBase+i, firstBase+i)
	}
	p, err := prepareColor(face, black, Limits{}, nil, gids...)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.draws) != 80 {
		t.Fatalf("%d drawings", len(p.draws))
	}
	if _, err := prepareColor(face, black, Limits{MaxImagePixels: 39 * 64}, nil, gids...); !errors.Is(err, ErrLimit) {
		t.Fatalf("pixels: %v", err)
	}
	// Pictures are still counted: the glyphs' images are not among them.
	if _, err := prepareColor(face, black, Limits{MaxImages: 1}, nil, gids...); err != nil {
		t.Fatal(err)
	}
}

func TestColorGlyphPlacement(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{paintGlyph(gidFull, paintSolid(0, 1)), paintColorNothing()}})
	// The run's clip crops the glyph, as it crops a plain one.
	op := colorOps(face, black, firstBase)[0].(layout.DrawGlyphs)
	op.Clip = layout.Clip{Active: true, Rect: layout.Rect{X: unit(0), Y: unit(0), W: unit(60), H: unit(120)}}
	p, err := Prepare(context.Background(), dml.Pixels(colorPage), dml.Pixels(colorPage), []layout.Op{op}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	img, err := p.Raster(context.Background(), 96)
	if err != nil {
		t.Fatal(err)
	}
	near(t, img.RGBAAt(30, 60), red, 0, "inside the run's clip")
	near(t, img.RGBAAt(90, 60), nothing, 0, "outside the run's clip")
	// So does a clip of the page.
	square := layout.Path{
		{Op: layout.MoveTo, Point: layout.Point{X: unit(0), Y: unit(0)}}, {Op: layout.LineTo, Point: layout.Point{X: unit(60), Y: unit(0)}},
		{Op: layout.LineTo, Point: layout.Point{X: unit(60), Y: unit(120)}}, {Op: layout.LineTo, Point: layout.Point{X: unit(0), Y: unit(120)}}, {Op: layout.ClosePath},
	}
	clipped := layout.ClipPath{Path: square, Ops: colorOps(face, black, firstBase)}
	p, err = Prepare(context.Background(), dml.Pixels(colorPage), dml.Pixels(colorPage), []layout.Op{clipped}, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if img, err = p.Raster(context.Background(), 96); err != nil {
		t.Fatal(err)
	}
	near(t, img.RGBAAt(30, 60), red, 0, "inside the page's clip")
	near(t, img.RGBAAt(90, 60), nothing, 0, "outside the page's clip")
	// A glyph that paints nothing still carries its text, and a glyph that
	// paints carries it on its first drawing.
	p, err = prepareColor(face, black, Limits{}, nil, firstBase+1, firstBase)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.draws) != 2 || p.draws[0].text != "x" || p.draws[1].text != "" || p.draws[0].fontID == "" || p.draws[1].fontID == "" {
		t.Fatalf("drawings: %+v", p.draws)
	}
	// The second glyph is one advance along.
	if img, err = p.Raster(context.Background(), 96); err != nil {
		t.Fatal(err)
	}
	near(t, img.RGBAAt(150-60, 60), nothing, 0, "second glyph, off the page")
}

// paintColorNothing is a color glyph that paints nothing at all.
func paintColorNothing() *paintNode { return paintGlyph(gidFull, paintSolid(0, 0)) }

func TestColorGlyphOutput(t *testing.T) {
	face := colorFont(t, colorFace{
		bases: []*paintNode{
			paintGlyph(gidFull, paintLinear(colorLine(0, stops2...), 0, 0, 1000, 0, 0, 100)),
			paintGlyph(gidFull, paintRadial(colorLine(0, stops2...), 500, 500, 0, 500, 500, 500)),
			paintGlyph(gidLower, paintSolid(0, 1)),
		},
		bitmaps: []*image.NRGBA{quadrants(50)},
	})
	p, err := prepareColor(face, black, Limits{}, nil, firstBase, firstBase+1, firstBase+2, firstBase+3)
	if err != nil {
		t.Fatal(err)
	}
	var svg bytes.Buffer
	if err := p.WriteSVG(context.Background(), &svg, 96); err != nil {
		t.Fatal(err)
	}
	out := svg.String()
	for _, want := range []string{"<linearGradient", "<radialGradient", `clip-rule="nonzero"`, "<image"} {
		if !strings.Contains(out, want) {
			t.Fatalf("SVG lacks %s: %s", want, out)
		}
	}
	// A glyph's image is smoothed where scaled, and its clips are small.
	if strings.Contains(out, "pixelated") {
		t.Fatal("glyph image drawn pixelated")
	}
	if strings.Contains(out, `<rect x="0" y="0" width="120"`) {
		t.Fatal("drawing over the whole page")
	}
	// Drawing at twice the resolution is the same picture, twice the size.
	for _, gid := range []int{firstBase, firstBase + 1, firstBase + 3} {
		single, err := prepareColor(face, black, Limits{}, nil, gid)
		if err != nil {
			t.Fatal(err)
		}
		small, err := single.Raster(context.Background(), 96)
		if err != nil {
			t.Fatal(err)
		}
		big, err := single.Raster(context.Background(), 192)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range [][2]int{{30, 40}, {60, 60}, {85, 95}, {45, 100}} {
			near(t, big.RGBAAt(2*p[0], 2*p[1]), small.RGBAAt(p[0], p[1]), 12, "twice the resolution")
		}
	}
}

func TestColorGlyphConcurrent(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{paintGlyph(gidFull, paintRadial(colorLine(0, stops2...), 500, 500, 0, 500, 500, 500))}, sbix: true, bitmaps: []*image.NRGBA{quadrants(20)}})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if _, err := prepareColor(face, black, Limits{}, nil, firstBase, firstBase+1); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestHasColorGlyphs(t *testing.T) {
	face := colorFont(t, colorFace{bases: []*paintNode{paintGlyph(gidFull, paintSolid(0, 1))}})
	run := func(gids ...int) layout.DrawGlyphs { return colorOps(face, black, gids...)[0].(layout.DrawGlyphs) }
	if !HasColorGlyphs(run(gidMarker, firstBase)) || HasColorGlyphs(run(gidMarker, gidFull)) || HasColorGlyphs(layout.DrawGlyphs{}) {
		t.Fatal("color glyphs")
	}
	// A turned run is outlined, in the one color, with the glyph's own outline.
	var segments int
	paths, exact, err := GlyphPaths(context.Background(), run(firstBase), func(x, y float64) (float64, float64) { return x, y }, 1000, &segments)
	if err != nil || len(paths) != 1 || !exact {
		t.Fatalf("%v %d %v", err, len(paths), exact)
	}
}
