package dml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestBoolAttrCaptureCoverage already guarantees every type with a boolean
// attribute *has* a capture hook. That is a structural guard: it is satisfied by
// a hook that captures into a field nothing ever replays, or one that was
// pasted from its neighbour and decodes through the wrong alias. Nothing
// executed most of these hooks — six of the twenty-two never ran in the whole
// module's test suite — so a hook could have been silently inert.
//
// This closes the loop behaviourally: for every hook in xml_bool_capture.go,
// take a fragment as a producer wrote it, parse it through the source-aware
// decoder (the path that feeds CaptureAttrsSource its raw bytes) and require
// the whole attribute list to come back verbatim — spelling, order and
// spacing. The case list is checked against the hooks found in the file by
// AST, so a hook added tomorrow fails this test until it has a fragment.

// nsChartDrawing is the cdr: namespace (dml-chartDrawing.xsd); common/xml has
// no constant for it.
const nsChartDrawing = "http://schemas.openxmlformats.org/drawingml/2006/chartDrawing"

// boolCaptureCase is one hook's verbatim round trip.
type boolCaptureCase struct {
	// typ is the receiver type name of the UnmarshalXML hook in
	// xml_bool_capture.go.
	typ string
	// local is the element name to re-emit under.
	local string
	// src is the source element exactly as a producer wrote it. Its attribute
	// list must survive a parse/marshal cycle byte for byte.
	src string
	// newV allocates the model.
	newV func() interface{}
	// parsed reports the boolean the fragment sets, read back off the model.
	// Replay is driven by the capture, not by the model, so a hook that
	// captured but forgot to decode (or decoded through the wrong alias) would
	// still round-trip perfectly while every reader of the model saw a zero.
	parsed func(interface{}) bool
}

// Value-typed bools with omitempty are deliberately only given "true"
// spellings: C586 makes a cleared value-typed bool delete the attribute, so a
// source false would correctly *not* be replayed and would say nothing about
// the capture. Pointer bools carry both spellings, including the XSD
// default-TRUE ones where dropping the attribute would invert its meaning.
var boolCaptureCases = []boolCaptureCase{
	{"BlipFillXML", "blipFill",
		`<blipFill xmlns="` + NsDrawingML + `" dpi="300" rotWithShape="true"/>`,
		func() interface{} { return &BlipFillXML{} },
		func(v interface{}) bool { return deref(v.(*BlipFillXML).RotWithShape) }},
	{"BlipFill", "blipFill",
		`<blipFill xmlns="` + NsDrawingML + `" rotWithShape="false" dpi="96"/>`,
		func() interface{} { return &BlipFill{} },
		func(v interface{}) bool { return !deref(v.(*BlipFill).RotWithShape) }},
	{"GradFill", "gradFill",
		`<gradFill xmlns="` + NsDrawingML + `" flip="tile" rotWithShape="true"/>`,
		func() interface{} { return &GradFill{} },
		func(v interface{}) bool { return deref(v.(*GradFill).RotWithShape) }},
	{"Lin", "lin",
		`<lin xmlns="` + NsDrawingML + `" ang="5400000" scaled="false"/>`,
		func() interface{} { return &Lin{} },
		func(v interface{}) bool { return !deref(v.(*Lin).Scaled) }},
	{"BlurXML", "blur",
		`<blur xmlns="` + NsDrawingML + `" rad="12700" grow="false"/>`,
		func() interface{} { return &BlurXML{} },
		func(v interface{}) bool { return !deref(v.(*BlurXML).Grow) }},
	{"ClrChange", "clrChange",
		`<clrChange xmlns="` + NsDrawingML + `" useA="false"/>`,
		func() interface{} { return &ClrChange{} },
		func(v interface{}) bool { return !deref(v.(*ClrChange).UseA) }},
	{"CNvPicPr", "cNvPicPr",
		`<cNvPicPr xmlns="` + NsDrawingML + `" preferRelativeResize="false"/>`,
		func() interface{} { return &CNvPicPr{} },
		func(v interface{}) bool { return !deref(v.(*CNvPicPr).PreferRelativeResize) }},
	{"NvAudioPr", "nvAudioPr",
		`<nvAudioPr xmlns="` + NsDrawingML + `" isPhoto="true"/>`,
		func() interface{} { return &NvAudioPr{} },
		func(v interface{}) bool { return v.(*NvAudioPr).IsPhoto }},
	{"CNvAudioPr", "cNvAudioPr",
		`<cNvAudioPr xmlns="` + NsDrawingML + `" isPhoto="true"/>`,
		func() interface{} { return &CNvAudioPr{} },
		func(v interface{}) bool { return v.(*CNvAudioPr).IsPhoto }},
	{"OleObject", "oleObj",
		`<oleObj xmlns="` + NsDrawingML + `" xmlns:r="` + xmlb.NSOfficeDocumentRels +
			`" r:id="rId3" progId="Excel.Sheet.12" imgW="2540" imgH="1270" updateAutomatic="true"/>`,
		func() interface{} { return &OleObject{} },
		func(v interface{}) bool { return v.(*OleObject).UpdateAutomatic }},
	{"PMLNvPr", "nvPr",
		`<nvPr xmlns="` + NsDrawingML + `" userDrawn="true"/>`,
		func() interface{} { return &PMLNvPr{} },
		func(v interface{}) bool { return v.(*PMLNvPr).UserDrawn }},
	{"TextRtl", "rtl",
		`<rtl xmlns="` + NsDrawingML + `" val="true"/>`,
		func() interface{} { return &TextRtl{} },
		func(v interface{}) bool { return v.(*TextRtl).Val }},
	{"Wsp", "wsp",
		`<wsp xmlns="` + xmlb.NSWordprocessingShape + `" normalEastAsianFlow="false"/>`,
		func() interface{} { return &Wsp{} },
		func(v interface{}) bool { return !deref(v.(*Wsp).NormalEastAsianFlow) }},
	{"WPAnchor", "anchor",
		`<anchor xmlns="` + xmlb.NSDrawingMLWordprocessing + `" distT="0" distB="0" distL="114300" distR="114300"` +
			` simplePos="0" relativeHeight="251658240" behindDoc="false" locked="false"` +
			` layoutInCell="true" allowOverlap="true"/>`,
		func() interface{} { return &WPAnchor{} },
		func(v interface{}) bool { return v.(*WPAnchor).LayoutInCell && v.(*WPAnchor).AllowOverlap }},
	{"WPWrapPolygon", "wrapPolygon",
		`<wrapPolygon xmlns="` + xmlb.NSDrawingMLWordprocessing + `" edited="true"/>`,
		func() interface{} { return &WPWrapPolygon{} },
		func(v interface{}) bool { return v.(*WPWrapPolygon).Edited }},
	{"XDRSp", "sp",
		`<sp xmlns="` + xmlb.NSDrawingMLSpreadsheet + `" macro="" textlink="" fLocksText="true" fPublished="false"/>`,
		func() interface{} { return &XDRSp{} },
		func(v interface{}) bool { return deref(v.(*XDRSp).FLocksText) && !deref(v.(*XDRSp).FPublished) }},
	{"XDRPic", "pic",
		`<pic xmlns="` + xmlb.NSDrawingMLSpreadsheet + `" macro="" fPublished="true"/>`,
		func() interface{} { return &XDRPic{} },
		func(v interface{}) bool { return deref(v.(*XDRPic).FPublished) }},
	{"XDRCxnSp", "cxnSp",
		`<cxnSp xmlns="` + xmlb.NSDrawingMLSpreadsheet + `" macro="" fPublished="false"/>`,
		func() interface{} { return &XDRCxnSp{} },
		func(v interface{}) bool { return !deref(v.(*XDRCxnSp).FPublished) }},
	{"XDRClientData", "clientData",
		`<clientData xmlns="` + xmlb.NSDrawingMLSpreadsheet + `" fLocksWithSheet="false" fPrintsWithSheet="true"/>`,
		func() interface{} { return &XDRClientData{} },
		func(v interface{}) bool {
			return !deref(v.(*XDRClientData).FLocksWithSheet) && deref(v.(*XDRClientData).FPrintsWithSheet)
		}},
	{"CDRSp", "sp",
		`<sp xmlns="` + nsChartDrawing + `" macro="" textlink="" fLocksText="false" fPublished="true"/>`,
		func() interface{} { return &CDRSp{} },
		func(v interface{}) bool { return !deref(v.(*CDRSp).FLocksText) && deref(v.(*CDRSp).FPublished) }},
	{"CDRPic", "pic",
		`<pic xmlns="` + nsChartDrawing + `" macro="" fPublished="false"/>`,
		func() interface{} { return &CDRPic{} },
		func(v interface{}) bool { return !deref(v.(*CDRPic).FPublished) }},
	{"CDRCxnSp", "cxnSp",
		`<cxnSp xmlns="` + nsChartDrawing + `" macro="" fPublished="true"/>`,
		func() interface{} { return &CDRCxnSp{} },
		func(v interface{}) bool { return deref(v.(*CDRCxnSp).FPublished) }},
}

// TestBoolCaptureHooksReplayTheSourceAttrList is the behavioural half of
// TestBoolAttrCaptureCoverage: every capture hook must actually round-trip the
// producer's attribute list.
func TestBoolCaptureHooksReplayTheSourceAttrList(t *testing.T) {
	for _, tc := range boolCaptureCases {
		t.Run(tc.typ, func(t *testing.T) {
			v := tc.newV()
			// UnmarshalWithSource is what registers the raw bytes the capture
			// slices its verbatim spellings from; plain xml.Unmarshal silently
			// degrades to the re-rendered form.
			if err := xmlb.UnmarshalWithSource([]byte(tc.src), v); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !tc.parsed(v) {
				t.Errorf("hook captured the attributes but did not decode them into the model; "+
					"replay is driven by the capture, so this would round-trip cleanly while "+
					"every reader of %s saw a zero value", tc.typ)
			}
			want := attrSection(t, tc.src)
			got := buildFragment(t, tc.local, v)
			if !strings.Contains(got, want) {
				t.Errorf("capture hook did not replay the source attribute list.\n"+
					" want attrs: %s\n got output: %s", want, got)
			}
		})
	}
}

// Every hook calls CaptureAttrsSource, not CaptureAttrs, and the difference is
// invisible on a well-formed self-contained tag: both replay the same
// attributes. It shows up on C348's case — an attribute whose namespace is
// declared on an *ancestor* and is not in the well-known prefix table, so
// nothing on the element itself can resolve it. Only the source-aware capture
// recovers the producer's prefix from the raw tag; the plain one emits the
// attribute unprefixed, silently re-homing it into the default namespace.
func TestBoolCaptureHooksRecoverAncestorDeclaredPrefixes(t *testing.T) {
	src := `<wrap xmlns="` + NsDrawingML + `" xmlns:xr="urn:not-a-known-namespace">` +
		`<blipFill xr:uid="{6C4C8F1B}" rotWithShape="true"/></wrap>`
	var w struct {
		BlipFill *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill"`
	}
	if err := xmlb.UnmarshalWithSource([]byte(src), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if w.BlipFill == nil {
		t.Fatal("blipFill not decoded")
	}
	out := buildFragment(t, "blipFill", w.BlipFill)
	if !strings.Contains(out, `xr:uid="{6C4C8F1B}"`) {
		t.Errorf("prefix of an ancestor-declared unknown namespace lost on replay "+
			"(the capture must be source-aware, see C348): %s", out)
	}
}

// deref reads a *bool, treating nil ("no value held") as false.
func deref(p *bool) bool { return p != nil && *p }

// attrSection returns everything between the element name and the closing
// "/>" of a self-closing start tag: the producer's attribute list verbatim,
// including its spelling, order and separating spaces.
func attrSection(t *testing.T, src string) string {
	t.Helper()
	if !strings.HasPrefix(src, "<") || !strings.HasSuffix(src, "/>") {
		t.Fatalf("fixture must be a single self-closing element: %s", src)
	}
	body := src[1 : len(src)-2]
	sp := strings.IndexByte(body, ' ')
	if sp < 0 {
		t.Fatalf("fixture has no attributes: %s", src)
	}
	return strings.TrimRight(body[sp+1:], " ")
}

// The case list is derived from the file rather than maintained beside it: a
// hook added to xml_bool_capture.go with no fragment here would otherwise ship
// unexecuted, which is exactly the state six of these hooks were already in.
func TestBoolCaptureCasesCoverEveryHook(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(thisFile), "xml_bool_capture.go")
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatalf("parse xml_bool_capture.go: %v", err)
	}

	hooks := map[string]bool{}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "UnmarshalXML" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
		if !ok {
			continue
		}
		if id, ok := star.X.(*ast.Ident); ok {
			hooks[id.Name] = true
		}
	}
	if len(hooks) == 0 {
		t.Fatal("found no UnmarshalXML hooks; the guard would pass vacuously")
	}

	covered := map[string]bool{}
	for _, tc := range boolCaptureCases {
		if covered[tc.typ] {
			t.Errorf("duplicate boolCaptureCases entry for %s", tc.typ)
		}
		covered[tc.typ] = true
	}

	var missing, stale []string
	for typ := range hooks {
		if !covered[typ] {
			missing = append(missing, typ)
		}
	}
	for typ := range covered {
		if !hooks[typ] {
			stale = append(stale, typ)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)
	if len(missing) > 0 {
		t.Errorf("xml_bool_capture.go hooks with no round-trip fragment (an inert hook would "+
			"go unnoticed): %s", strings.Join(missing, ", "))
	}
	if len(stale) > 0 {
		t.Errorf("boolCaptureCases entries whose hook no longer exists: %s", strings.Join(stale, ", "))
	}
}
