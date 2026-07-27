package dml

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// pctCase is one swept percentage attribute site (C400).
type pctCase struct {
	name   string
	newVal func() interface{}
	elem   string // element local name
	pct    string // transitional source element
	canon  string // strict source element
	get    func(interface{}) []int32
	want   []int32
}

// TestPercentClassTransitionalForms exercises the swept elements end to end:
// the transitional "n%" form must parse (not fail the part) and re-emit
// verbatim, and a canonical integer source must still emit canonically.
func TestPercentClassTransitionalForms(t *testing.T) {
	tests := []pctCase{
		{
			name:   "alphaBiLevel/thresh",
			newVal: func() interface{} { return &AlphaBiLevel{} },
			elem:   "alphaBiLevel",
			pct:    `<a:alphaBiLevel thresh="50%"/>`,
			canon:  `<a:alphaBiLevel thresh="50000"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*AlphaBiLevel).Thresh.Int32()} },
			want:   []int32{50000},
		},
		{
			name:   "alphaRepl/a",
			newVal: func() interface{} { return &AlphaRepl{} },
			elem:   "alphaRepl",
			pct:    `<a:alphaRepl a="12.5%"/>`,
			canon:  `<a:alphaRepl a="12500"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*AlphaRepl).A.Int32()} },
			want:   []int32{12500},
		},
		{
			name:   "camera/zoom",
			newVal: func() interface{} { return &Camera{} },
			elem:   "camera",
			pct:    `<a:camera prst="orthographicFront" fov="0" zoom="150%"/>`,
			canon:  `<a:camera prst="orthographicFront" fov="0" zoom="150000"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*Camera).Zoom.Int32()} },
			want:   []int32{150000},
		},
		{
			name:   "ds/d+sp",
			newVal: func() interface{} { return &Ds{} },
			elem:   "ds",
			pct:    `<a:ds d="300%" sp="100%"/>`,
			canon:  `<a:ds d="300000" sp="100000"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*Ds).D.Int32(), v.(*Ds).Sp.Int32()} },
			want:   []int32{300000, 100000},
		},
		{
			name:   "miter/lim",
			newVal: func() interface{} { return &Miter{} },
			elem:   "miter",
			pct:    `<a:miter lim="800%"/>`,
			canon:  `<a:miter lim="800000"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*Miter).Lim.Int32()} },
			want:   []int32{800000},
		},
		{
			name:   "tile/sx+sy",
			newVal: func() interface{} { return &Tile{} },
			elem:   "tile",
			pct:    `<a:tile tx="0" ty="0" sx="50%" sy="50%" flip="xy" algn="ctr"/>`,
			canon:  `<a:tile tx="0" ty="0" sx="50000" sy="50000" flip="xy" algn="ctr"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*Tile).Sx.Int32(), v.(*Tile).Sy.Int32()} },
			want:   []int32{50000, 50000},
		},
		{
			name:   "tileXML/sx+sy",
			newVal: func() interface{} { return &TileXML{} },
			elem:   "tile",
			pct:    `<a:tile sx="50%" sy="50%"/>`,
			canon:  `<a:tile sx="50000" sy="50000"/>`,
			get:    func(v interface{}) []int32 { return []int32{v.(*TileXML).Sx.Int32(), v.(*TileXML).Sy.Int32()} },
			want:   []int32{50000, 50000},
		},
		{
			name:   "rPr/baseline",
			newVal: func() interface{} { return &RPr{} },
			elem:   "rPr",
			pct:    `<a:rPr lang="en-US" baseline="30%"/>`,
			canon:  `<a:rPr lang="en-US" baseline="30000"/>`,
			get: func(v interface{}) []int32 {
				b := v.(*RPr).Baseline
				if b == nil {
					return nil
				}
				return []int32{b.Int32()}
			},
			want: []int32{30000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, src := range []string{tt.pct, tt.canon} {
				v := tt.newVal()
				if err := parseElement(t, src, v); err != nil {
					t.Fatalf("parsing %s failed (this is the C400/C277 bug): %v", src, err)
				}
				if got := tt.get(v); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("%s: value = %v, want %v", src, got, tt.want)
				}
				// Both lexical forms must re-emit verbatim: the transitional
				// "n%" keeps its spelling, and an int-sourced 50000 must NOT
				// start emitting "50%".
				if out := marshalElement(t, tt.elem, v); out != src {
					t.Errorf("re-emit = %s\nwant      %s", out, src)
				}
			}
		})
	}
}

// TestPercentClassLivePathBlip is the audit's exact repro: a transitional
// alphaBiLevel inside an a:blip used to propagate a ParseInt error out of
// BlipXML.UnmarshalXML and fail the whole part on the production Open path.
func TestPercentClassLivePathBlip(t *testing.T) {
	src := `<a:blip r:embed="rId1"><a:alphaBiLevel thresh="50%"/></a:blip>`
	var b BlipXML
	if err := parseElement(t, src, &b); err != nil {
		t.Fatalf("blip with transitional alphaBiLevel failed to parse: %v", err)
	}
	if len(b.Effects) != 1 || b.Effects[0].AlphaBiLevel == nil {
		t.Fatalf("alphaBiLevel effect not parsed: %+v", b.Effects)
	}
	if got := b.Effects[0].AlphaBiLevel.Thresh.Int32(); got != 50000 {
		t.Errorf("thresh = %d, want 50000", got)
	}
	if out := marshalElement(t, "blip", &b); out != src {
		t.Errorf("re-emit = %s\nwant      %s", out, src)
	}
}

// pctWrapperNS declares the prefixes the test fragments use, on a wrapper root
// rather than on the element under test: a real part declares them once at the
// part root, and putting them on the element itself would both feed a spurious
// xmlns into CapturedAttrs and (for a:alphaRepl) collide with its "a" attribute
// under encoding/xml's namespace-insensitive attribute matching.
var pctWrapperNS = `<a:wrap xmlns:a="` + NsDrawingML + `" xmlns:r="` + xmlb.NSOfficeDocumentRels + `">`

// parseElement decodes a single element fragment into v, with the DrawingML
// and relationship prefixes bound on an enclosing wrapper.
func parseElement(t *testing.T, elem string, v interface{}) error {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(pctWrapperNS + elem + `</a:wrap>`))
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if se, ok := tok.(xml.StartElement); ok {
			depth++
			if depth == 2 {
				return d.DecodeElement(v, &se)
			}
		}
	}
}

// marshalElement marshals v through the production Builder path and returns
// the element source, with the wrapper's declarations stripped back off.
func marshalElement(t *testing.T, localName string, v interface{}) string {
	t.Helper()
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	b.SetCollapseEmptyElements(true)
	b.StartElementWithNS(NsDrawingML, "wrap", []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingML, URI: NsDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	})
	b.MarshalElement(NsDrawingML, localName, v)
	b.EndElement(NsDrawingML, "wrap")
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := string(b.Bytes())
	out = strings.TrimPrefix(out, pctWrapperNS)
	return strings.TrimSuffix(out, `</a:wrap>`)
}
