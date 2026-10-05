package metafile

import (
	"fmt"
	"math"

	"github.com/mgilbir/gowemf"
	"github.com/mgilbir/spine/render"
)

func (it *interp) wmfRecord(c gowemf.Command) error {
	t := c.Source.Type & 0xff
	d := &it.dc
	switch v := c.Body.(type) {
	case gowemf.Value:
		switch t {
		case 0x02:
			d.bkMode = v.Value
		case 0x03:
			return it.setMapMode(int(v.Value))
		case 0x04:
			d.rop2 = v.Value
		case 0x06:
			d.polyFill = v.Value
		case 0x07:
			d.stretchMode = v.Value
		case 0x09:
			d.textColor = v.Value
		case 0x01:
			d.bkColor = v.Value
		case 0x2e:
			d.textAlign = v.Value
		case 0x2d:
			return it.selectObject(v.Value)
		case 0xf0:
			delete(it.objs, v.Value)
		case 0x2c:
			return it.wmfClipRegion(v.Value)
		case 0x2a, 0x2b:
			return it.soft("region invert or paint left out")
		case 0x49:
			if v.Value&1 != 0 {
				return it.soft("right-to-left layout drawn left to right")
			}
		case 0x31, 0x34:
		default:
			return it.soft("WMF record %#x left out", t)
		}
		return nil
	case gowemf.SignedValue:
		switch t {
		case 0x27:
			return it.restoreDC(v.Value)
		case 0x08:
			d.charExtra = v.Value
		}
		return nil
	case gowemf.Empty:
		if t == 0x1e {
			return it.saveDC()
		}
		return nil
	case gowemf.PointRecord:
		p := point{v.Point.X, v.Point.Y}
		switch t {
		case 0x0b:
			d.wndOrg = p
		case 0x0c:
			if !it.fixedMode() {
				d.wndExt = p
				it.fixIsotropic()
			}
		case 0x0d:
			d.vpOrg = p
		case 0x0e:
			if !it.fixedMode() {
				d.vpExt = p
				it.fixIsotropic()
			}
		case 0x0f:
			d.wndOrg = point{d.wndOrg.x + p.x, d.wndOrg.y + p.y}
		case 0x11:
			d.vpOrg = point{d.vpOrg.x + p.x, d.vpOrg.y + p.y}
		case 0x13:
			return it.lineTo(p)
		case 0x14:
			it.moveTo(p)
		case 0x20:
			return it.offsetClip(p)
		}
		return nil
	case gowemf.Scale:
		if it.fixedMode() {
			return nil
		}
		if t == 0x10 {
			d.wndExt = point{d.wndExt.x * float64(v.XNum) / float64(v.XDenom), d.wndExt.y * float64(v.YNum) / float64(v.YDenom)}
		} else {
			d.vpExt = point{d.vpExt.x * float64(v.XNum) / float64(v.XDenom), d.vpExt.y * float64(v.YNum) / float64(v.YDenom)}
		}
		it.fixIsotropic()
		return nil
	case gowemf.RectRecord:
		l, tp, r, b := rectOf(v.Rect)
		switch t {
		case 0x15:
			return it.clipRect(l, tp, r, b, clipDiff)
		case 0x16:
			return it.clipRect(l, tp, r, b, clipAnd)
		case 0x18:
			return it.ellipseShape(l, tp, r, b)
		case 0x1b:
			return it.rectShape(l, tp, r, b, nil)
		}
		return nil
	case gowemf.RoundRect:
		l, tp, r, b := rectOf(v.Rect)
		return it.rectShape(l, tp, r, b, &point{v.Corner.X, v.Corner.Y})
	case gowemf.Arc:
		switch t {
		case 0x17:
			return it.arc(arcOpen, v.Rect, v.Start, v.End)
		case 0x1a:
			return it.arc(arcPie, v.Rect, v.Start, v.End)
		case 0x30:
			return it.arc(arcChord, v.Rect, v.Start, v.End)
		}
		return nil
	case gowemf.Poly:
		switch t {
		case 0x24:
			return it.polys(v, true, false)
		case 0x25:
			return it.polys(v, false, false)
		case 0x38:
			return it.polyPolys(v, true)
		}
		return nil
	case gowemf.Pixel:
		return it.setPixel(v)
	case gowemf.Text:
		return it.text(v)
	case gowemf.TextJustification:
		d.justExtra, d.justCount = v.Extra, v.Count
		return nil
	case gowemf.RegionPaint:
		return it.wmfRegionPaint(t, v)
	case gowemf.PackedDIBTransfer:
		return it.wmfBitmap(t, v)
	case gowemf.Bitmap16Transfer:
		return it.soft("device-dependent bitmap left out")
	case gowemf.FloodFill:
		return it.soft("flood fill left out")
	case gowemf.WMFEnhancedMetafile, gowemf.Palette:
		return nil
	}
	return it.soft("WMF record %#x left out", t)
}

// wmfRegion is a WMF region object's rectangles.
func (it *interp) wmfRegionContours(r gowemf.WMFRegion) ([][]point, error) {
	var cs [][]point
	m := it.matrix()
	for _, s := range r.Scans {
		for i := 0; i+1 < s.Endpoints.Len(); i += 2 {
			if err := it.charge(4); err != nil {
				return nil, err
			}
			l, rr := float64(s.Endpoints.At(i)), float64(s.Endpoints.At(i+1))
			t, b := float64(s.Top), float64(s.Bottom)
			if rr <= l || b <= t {
				continue
			}
			cs = append(cs, []point{m.apply(point{l, t}), m.apply(point{rr, t}), m.apply(point{rr, b}), m.apply(point{l, b})})
		}
	}
	return cs, nil
}

func (it *interp) wmfClipRegion(id uint32) error {
	r, ok := it.objs[id].(gowemf.WMFRegion)
	if !ok {
		return nil
	}
	if err := it.soft("region coordinates read as logical units"); err != nil {
		return err
	}
	cs, err := it.wmfRegionContours(r)
	if err != nil {
		return err
	}
	return it.combineClip(cs, false, clipCopy)
}

func (it *interp) wmfRegionPaint(t uint32, v gowemf.RegionPaint) error {
	if t != 0x28 {
		return it.soft("region frame left out")
	}
	r, ok := it.objs[v.Region].(gowemf.WMFRegion)
	br, ok2 := it.objs[v.Brush].(*brushObj)
	if !ok || !ok2 || br.null {
		return nil
	}
	if br.unsupported != "" {
		return it.soft("%s fill left out", br.unsupported)
	}
	if err := it.soft("region coordinates read as logical units"); err != nil {
		return err
	}
	cs, err := it.wmfRegionContours(r)
	if err != nil {
		return err
	}
	if err = it.b.op(); err != nil {
		return err
	}
	col, err := it.color(br.color)
	if err != nil {
		return err
	}
	return it.r.fill(cs, false, fillStyle{solid: col}, it.dc.clip)
}

// wmfBounds finds the window a WMF without a placeable header draws in, from
// its first window origin and extent records.
func wmfWindow(data []byte, limits gowemf.Limits) (org, ext point, ok bool) {
	var haveExt bool
	_, err := gowemf.Walk(data, limits, func(r gowemf.Record) error {
		if r.Format != gowemf.WMF || haveExt {
			return nil
		}
		switch r.Type & 0xff {
		case 0x0b, 0x0c:
			body, err := gowemf.Decode(r, gowemf.DecodeLimits{})
			if err != nil {
				return nil
			}
			p, isPoint := body.(gowemf.PointRecord)
			if !isPoint {
				return nil
			}
			if r.Type&0xff == 0x0b {
				org = point{p.Point.X, p.Point.Y}
			} else {
				ext, haveExt = point{p.Point.X, p.Point.Y}, true
			}
		}
		return nil
	})
	if err != nil || !haveExt || ext.x == 0 || ext.y == 0 || math.IsNaN(ext.x) {
		return point{}, point{}, false
	}
	return org, ext, true
}

var errNoSize = fmt.Errorf("%w: metafile: WMF without a placeable header or window extent", render.ErrUnsupported)
