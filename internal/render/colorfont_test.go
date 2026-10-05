package render

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"sort"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

// A builder of small color fonts for the tests: a font whose outlines are
// rectangles, and whose COLR, CPAL, sbix, CBLC and CBDT tables are written
// here, so that what a glyph paints is stated beside the test that checks it.

// Glyphs 1 to 5 are outlines; base glyphs follow from firstBase. Boxes are in
// font units of a 1000-unit em, y up.
const (
	gidFull   = 1 // 0,0 to 1000,1000
	gidLower  = 2 // 0,0 to 500,500
	gidRight  = 3 // 500,0 to 1000,500
	gidTop    = 4 // 0,500 to 1000,1000
	gidMarker = 5 // 450,450 to 550,550: a base glyph's own outline
	firstBase = 6
)

// paintNode is a COLR table's paint or color line, with the offsets in it to
// what it holds. Offsets in COLR run forwards from the start of their table, so
// a node's children follow it.
type paintNode struct {
	data []byte
	kids []paintKid
}

type paintKid struct {
	at   int // where in data a 24-bit offset to the node is
	node *paintNode
}

// emit appends the node and, after it, its children, which it points at.
func (n *paintNode) emit(out *[]byte) int {
	start := len(*out)
	*out = append(*out, n.data...)
	for _, k := range n.kids {
		off := k.node.emit(out) - start
		(*out)[start+k.at], (*out)[start+k.at+1], (*out)[start+k.at+2] = byte(off>>16), byte(off>>8), byte(off)
	}
	return start
}

func be16(v int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(v)) }
func be32(v int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(v)) }
func f2dot14(v float64) []byte {
	return binary.BigEndian.AppendUint16(nil, uint16(int16(math.Round(v*16384))))
}
func join(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

type colorStop struct {
	offset float64
	index  int
	alpha  float64
}

// colorLine is a color line: extend 0 pads, 1 repeats and 2 reflects.
func colorLine(extend int, stops ...colorStop) *paintNode {
	data := join([]byte{byte(extend)}, be16(len(stops)))
	for _, s := range stops {
		data = join(data, f2dot14(s.offset), be16(s.index), f2dot14(s.alpha))
	}
	return &paintNode{data: data}
}

func paintSolid(index int, alpha float64) *paintNode {
	return &paintNode{data: join([]byte{2}, be16(index), f2dot14(alpha))}
}

func paintGlyph(gid int, paint *paintNode) *paintNode {
	return &paintNode{data: join([]byte{10, 0, 0, 0}, be16(gid)), kids: []paintKid{{1, paint}}}
}

func paintLinear(line *paintNode, x0, y0, x1, y1, x2, y2 int) *paintNode {
	data := []byte{4, 0, 0, 0}
	for _, v := range []int{x0, y0, x1, y1, x2, y2} {
		data = join(data, be16(v))
	}
	return &paintNode{data: data, kids: []paintKid{{1, line}}}
}

func paintRadial(line *paintNode, x0, y0, r0, x1, y1, r1 int) *paintNode {
	data := []byte{6, 0, 0, 0}
	for _, v := range []int{x0, y0, r0, x1, y1, r1} {
		data = join(data, be16(v))
	}
	return &paintNode{data: data, kids: []paintKid{{1, line}}}
}

// paintSweep's angles are in half turns, from the font's 180 degree bias.
func paintSweep(line *paintNode, cx, cy int, start, end float64) *paintNode {
	return &paintNode{data: join([]byte{8, 0, 0, 0}, be16(cx), be16(cy), f2dot14(start), f2dot14(end)), kids: []paintKid{{1, line}}}
}

func paintTranslate(paint *paintNode, dx, dy int) *paintNode {
	return &paintNode{data: join([]byte{14, 0, 0, 0}, be16(dx), be16(dy)), kids: []paintKid{{1, paint}}}
}

func paintScale(paint *paintNode, sx, sy float64) *paintNode {
	return &paintNode{data: join([]byte{16, 0, 0, 0}, f2dot14(sx), f2dot14(sy)), kids: []paintKid{{1, paint}}}
}

// paintRotate turns by angle half turns, counter-clockwise about the origin.
func paintRotate(paint *paintNode, angle float64) *paintNode {
	return &paintNode{data: join([]byte{24, 0, 0, 0}, f2dot14(angle)), kids: []paintKid{{1, paint}}}
}

// paintTransform applies (xx, yx, xy, yy, dx, dy), as shape.Transform says.
func paintTransform(paint *paintNode, xx, yx, xy, yy, dx, dy float64) *paintNode {
	var matrix []byte
	for _, v := range []float64{xx, yx, xy, yy, dx, dy} {
		matrix = binary.BigEndian.AppendUint32(matrix, uint32(int32(math.Round(v*65536))))
	}
	return &paintNode{data: []byte{12, 0, 0, 0, 0, 0, 0}, kids: []paintKid{{1, paint}, {4, &paintNode{data: matrix}}}}
}

func paintComposite(source *paintNode, mode int, backdrop *paintNode) *paintNode {
	return &paintNode{data: []byte{32, 0, 0, 0, byte(mode), 0, 0, 0}, kids: []paintKid{{1, source}, {5, backdrop}}}
}

func paintLayers(first, count int) *paintNode {
	return &paintNode{data: join([]byte{1, byte(count)}, be32(first))}
}

// colorFace is what colorFont builds a font of.
type colorFace struct {
	// bases are the paints of glyphs firstBase onwards.
	bases []*paintNode
	// layers is COLR's LayerList, which PaintColrLayers paints from.
	layers []*paintNode
	// v0 are COLRv0 glyphs, from the glyph after the last of bases: each a list
	// of (outline glyph, palette index).
	v0 [][][2]int
	// palette are CPAL's colors, in RGBA.
	palette []color.RGBA
	// bitmaps are the images of glyphs after the v0 glyphs, one glyph each.
	bitmaps []*image.NRGBA
	// sbix says whether the bitmaps are in an sbix table rather than CBDT.
	sbix bool
	// extra adds tables as they are.
	extra map[string][]byte
}

func (c colorFace) glyphs() []fonttest.Glyph {
	boxes := [][4]int{{0, 0, 1000, 1000}, {0, 0, 500, 500}, {500, 0, 1000, 500}, {0, 500, 1000, 1000}, {450, 450, 550, 550}}
	n := len(boxes) + len(c.bases) + len(c.v0) + len(c.bitmaps)
	var out []fonttest.Glyph
	for i := 0; i < n; i++ {
		box := boxes[len(boxes)-1]
		if i < len(boxes) {
			box = boxes[i]
		}
		out = append(out, fonttest.Glyph{Rune: rune(0x41 + i), Advance: 1000, HasShape: true, Ink: box})
	}
	return out
}

func (c colorFace) tables() map[string][]byte {
	tables := map[string][]byte{}
	for tag, data := range c.extra {
		tables[tag] = data
	}
	if len(c.bases)+len(c.v0) > 0 {
		tables["COLR"] = c.colr()
		palette := c.palette
		if palette == nil {
			palette = []color.RGBA{{255, 0, 0, 255}, {0, 0, 255, 255}, {0, 255, 0, 255}}
		}
		cpal := join(be16(0), be16(len(palette)), be16(1), be16(len(palette)), be32(14), be16(0))
		for _, p := range palette {
			cpal = append(cpal, p.B, p.G, p.R, p.A)
		}
		tables["CPAL"] = cpal
	}
	if len(c.bitmaps) > 0 {
		first := firstBase + len(c.bases) + len(c.v0)
		if c.sbix {
			tables["sbix"] = sbixTable(first+len(c.bitmaps), first, c.bitmaps)
		} else {
			tables["CBLC"], tables["CBDT"] = cbdtTables(first, c.bitmaps)
		}
	}
	return tables
}

// colr writes a COLR table of version 1.
func (c colorFace) colr() []byte {
	var records, layerRecords []byte
	type base struct {
		gid   int
		paint *paintNode
	}
	var v1 []base
	for i, p := range c.bases {
		v1 = append(v1, base{firstBase + i, p})
	}
	for i, layers := range c.v0 {
		records = join(records, be16(firstBase+len(c.bases)+i), be16(len(layerRecords)/4), be16(len(layers)))
		for _, l := range layers {
			layerRecords = join(layerRecords, be16(l[0]), be16(l[1]))
		}
	}
	sort.Slice(v1, func(i, j int) bool { return v1[i].gid < v1[j].gid })
	const header = 34
	recordsAt := header
	layerRecordsAt := recordsAt + len(records)
	listAt := layerRecordsAt + len(layerRecords)
	baseList := be32(len(v1))
	for range v1 {
		baseList = append(baseList, make([]byte, 6)...)
	}
	for i, b := range v1 {
		off := b.paint.emit(&baseList)
		copy(baseList[4+6*i:], join(be16(b.gid), be32(off)))
	}
	layerList := be32(len(c.layers))
	for range c.layers {
		layerList = append(layerList, make([]byte, 4)...)
	}
	for i, l := range c.layers {
		off := l.emit(&layerList)
		copy(layerList[4+4*i:], be32(off))
	}
	baseAt := listAt
	layerAt := 0
	if len(c.layers) > 0 {
		layerAt = baseAt + len(baseList)
	}
	table := join(be16(1), be16(len(records)/6), be32(recordsAt), be32(layerRecordsAt), be16(len(layerRecords)/4),
		be32(baseAt), be32(layerAt), be32(0), be32(0), be32(0))
	table = join(table, records, layerRecords, baseList)
	if layerAt != 0 {
		table = join(table, layerList)
	}
	return table
}

// quadrants is a square image of four colors: red, green, blue and yellow in
// reading order.
func quadrants(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	colors := [4]color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			q := 0
			if x >= size/2 {
				q++
			}
			if y >= size/2 {
				q += 2
			}
			img.SetNRGBA(x, y, colors[q])
		}
	}
	return img
}

// bitmapPNG is the PNG of a bitmap glyph; it is a test's own, so it cannot fail.
func bitmapPNG(img *image.NRGBA) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// sbixTable is an sbix table with one strike of 100 pixels per em, whose
// glyphs first onwards are the images, each placed with its lower left corner
// at the origin.
func sbixTable(numGlyphs, first int, images []*image.NRGBA) []byte {
	strike := join(be16(100), be16(72))
	offsets := 4 + 4*(numGlyphs+1)
	var data []byte
	for gid := 0; gid <= numGlyphs; gid++ {
		strike = join(strike, be32(offsets+len(data)))
		if gid >= first && gid-first < len(images) {
			data = join(data, be16(0), be16(0), []byte("png "), bitmapPNG(images[gid-first]))
		}
	}
	strike = join(strike, data)
	return join(be16(1), be16(1), be32(1), be32(12), strike)
}

// cbdtTables are CBLC and CBDT tables with one strike of 100 pixels per em,
// and the images, one glyph each from first, in format 17 with their lower
// left corner at the origin.
func cbdtTables(first int, images []*image.NRGBA) (cblc, cbdt []byte) {
	cbdt = join(be16(3), be16(0))
	offsets := make([]int, 0, len(images)+1)
	for _, img := range images {
		offsets = append(offsets, len(cbdt)-4)
		b := bitmapPNG(img)
		w, h := img.Rect.Dx(), img.Rect.Dy()
		cbdt = join(cbdt, []byte{byte(h), byte(w), 0, byte(h), byte(w)}, be32(len(b)), b)
	}
	offsets = append(offsets, len(cbdt)-4)
	sub := join(be16(1), be16(17), be32(4))
	for _, o := range offsets {
		sub = join(sub, be32(o))
	}
	array := join(be16(first), be16(first+len(images)-1), be32(8))
	line := join([]byte{100, 0, 0, 0}, make([]byte, 8))
	size := join(be32(56), be32(len(array)+len(sub)), be32(1), be32(0), line, line,
		be16(first), be16(first+len(images)-1), []byte{100, 100, 32, 1})
	cblc = join(be16(3), be16(0), be32(1), size, array, sub)
	return cblc, cbdt
}

// colorFont loads the face of a colorFace.
func colorFont(t testing.TB, c colorFace) *shape.Face {
	t.Helper()
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: c.glyphs(), Extra: c.tables()}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
