package chart

import (
	"encoding/xml"
	"math"
	"testing"
	"time"

	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzbound"
)

// chartBudget bounds one read of a chart part.
//
// This budget is why the target is worth having. A c:chartSpace is a deeply
// nested document — plot area, groups, series, caches, and a full dml.SpPr /
// dml.TxBody under most of them — so the typed model it inflates into is
// several times the bytes it came from; measured over 400 chart parts sampled
// from the corpus, the worst costs 77x its size to parse. But a cache's
// ptCount was used directly as an allocation size, so a 374-byte part
// declaring `<c:ptCount val="50000000"/>` allocated 400 MB: 1,070,000x, and
// unbounded in the count rather than merely large. That is fixed (see
// maxCachePoints in parse.go); the clamped worst case an adversarial input can
// still reach is about 1900x, on a part that is nothing but empty caches.
//
// A 1 MiB floor absorbs the decoder's fixed buffers, which dominate on the
// small inputs a fuzz run is mostly made of, and a 4096x rate leaves the
// clamped worst case a factor of two while still failing a regression to a
// count taken on faith by two and a half orders of magnitude.
var chartBudget = fuzzbound.Budget{
	What:              "chart parse",
	Bytes:             1 << 20,
	BytesPerInputByte: 4096,
	Time:              10 * time.Second,
	TimePerMiB:        5 * time.Second,
}

// chartSeeds are chart parts reduced from the corpus, chosen for the shapes
// that carry data references and caches — the parts of a chart.xml that a
// read/modify/write cycle has to reproduce.
var chartSeeds = []string{
	// A clustered column chart with a cached currency format.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<c:chart><c:title><c:tx><c:rich><a:bodyPr/><a:p><a:r><a:t>Revenue</a:t></a:r></a:p></c:rich></c:tx></c:title>` +
		`<c:plotArea><c:layout/><c:barChart><c:barDir val="col"/><c:grouping val="stacked"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>North</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:spPr><a:solidFill><a:srgbClr val="4472C4"/></a:solidFill></c:spPr>` +
		`<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Q1</c:v></c:pt><c:pt idx="1"><c:v>Q2</c:v></c:pt></c:strCache></c:strRef></c:cat>` +
		`<c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f><c:numCache><c:formatCode>&quot;$&quot;#,##0.00</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numCache></c:numRef></c:val>` +
		`<c:extLst><c:ext uri="{C3380CC4-5D6E-409C-BE32-E72D297353CC}" xmlns:c16="http://schemas.microsoft.com/office/drawing/2014/chart">` +
		`<c16:uniqueId val="{00000000-CAE2-4B95-830E-EF473911C3D8}"/></c:ext></c:extLst>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea>` +
		`<c:legend><c:legendPos val="b"/></c:legend></c:chart>` +
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData></c:chartSpace>`,
	// A sheet name that needs quoting and escapes an apostrophe by doubling it,
	// alongside a multi-area (union) reference: the two reference forms whose
	// sheet name grew on every save until sheetOf learned to undo the quoting.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<c:chart><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:tx><c:strRef><c:f>'Vue d''ensemble FC'!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>S</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:cat><c:multiLvlStrRef><c:f>('Vue d''ensemble FC'!$Z$2,'Vue d''ensemble FC'!$AD$2)</c:f></c:multiLvlStrRef></c:cat>` +
		`<c:val><c:numRef><c:f>('Vue d''ensemble FC'!$Z$3,'Vue d''ensemble FC'!$AD$3)</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numCache></c:numRef></c:val>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea></c:chart></c:chartSpace>`,
	// A scatter chart whose X source is categorical, so re-serializing writes
	// c:xVal as a literal and only the value reference names the sheet.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<c:chart><c:plotArea><c:layout/><c:scatterChart><c:scatterStyle val="lineMarker"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:tx><c:strRef><c:f>'Through Egypt'!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>Trip</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:xVal><c:strRef><c:f>'Through Egypt'!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/><c:pt idx="0"><c:v>Jan</c:v></c:pt><c:pt idx="1"><c:v>Feb</c:v></c:pt></c:strCache></c:strRef></c:xVal>` +
		`<c:yVal><c:numRef><c:f>'Through Egypt'!$B$2:$B$3</c:f><c:numCache><c:ptCount val="2"/><c:pt idx="0"><c:v>1.5</c:v></c:pt><c:pt idx="1"><c:v>2.5</c:v></c:pt></c:numCache></c:numRef></c:yVal>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:scatterChart></c:plotArea></c:chart></c:chartSpace>`,
	// A combination chart: a bar group and a line group on two value axes.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<c:chart><c:plotArea><c:layout/>` +
		`<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/><c:tx><c:v>Units</c:v></c:tx>` +
		`<c:val><c:numLit><c:formatCode>General</c:formatCode><c:ptCount val="2"/><c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>4</c:v></c:pt></c:numLit></c:val></c:ser>` +
		`<c:axId val="1"/><c:axId val="2"/></c:barChart>` +
		`<c:lineChart><c:grouping val="standard"/>` +
		`<c:ser><c:idx val="1"/><c:order val="1"/><c:tx><c:v>Rate</c:v></c:tx>` +
		`<c:val><c:numLit><c:ptCount val="2"/><c:pt idx="0"><c:v>0.1</c:v></c:pt><c:pt idx="1"><c:v>0.2</c:v></c:pt></c:numLit></c:val></c:ser>` +
		`<c:axId val="3"/><c:axId val="4"/></c:lineChart></c:plotArea></c:chart></c:chartSpace>`,
	// A pie chart with a blank cached point and a 3D view.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<c:chart><c:view3D><c:rotX val="30"/><c:rotY val="0"/></c:view3D><c:plotArea><c:layout/><c:pie3DChart><c:varyColors val="1"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:cat><c:strLit><c:ptCount val="3"/><c:pt idx="0"><c:v>a</c:v></c:pt><c:pt idx="2"><c:v>c</c:v></c:pt></c:strLit></c:cat>` +
		`<c:val><c:numLit><c:ptCount val="3"/><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="2"><c:v>3</c:v></c:pt></c:numLit></c:val>` +
		`</c:ser></c:pie3DChart></c:plotArea></c:chart></c:chartSpace>`,
	// A cache declaring more points than the part could ever hold: the shape
	// that drove a 400 MB allocation out of 374 bytes before ptCount was
	// clamped. It is a seed rather than a note so a regression is reported by
	// `go test` over the seed corpus, not only by a -fuzz run that happens to
	// invent the digits.
	//
	// The count is deliberately one the machine can still serve. At the 32-bit
	// maximum an unclamped parse asks for 68 GB, which the allocator cannot map
	// at all: the process dies with a fatal out-of-memory before chartBudget can
	// measure anything, and a fatal OOM is the failure mode fuzzbound exists to
	// turn into a readable report. 50 million is 400 MB — comfortably past the
	// budget, comfortably inside a capped test run. The 32-bit maximum is
	// covered by TestCachePointCountIsClamped, which asserts the clamp directly
	// rather than through an allocation.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart><c:plotArea><c:layout/><c:barChart><c:barDir val="col"/>` +
		`<c:ser><c:idx val="0"/><c:order val="0"/>` +
		`<c:cat><c:strLit><c:ptCount val="50000000"/></c:strLit></c:cat>` +
		`<c:val><c:numLit><c:ptCount val="50000000"/><c:pt idx="0"><c:v>1</c:v></c:pt></c:numLit></c:val>` +
		`</c:ser><c:axId val="1"/><c:axId val="2"/></c:barChart></c:plotArea></c:chart></c:chartSpace>`,
	// A chart part carrying only the wrapper: Parse must reject it, honestly.
	`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:layout/></c:plotArea></c:chart></c:chartSpace>`,
}

// FuzzParseChartXML drives chart.Parse and MarshalChartXML as a pair. Chart XML
// is reached only incidentally by the package-open fuzzers, which mutate raw
// archive bytes and so usually break the zip before any chart parser runs.
//
// Three oracles:
//
//   - Errors are honest. Parse returns a chart or an error, never both and
//     never neither.
//   - The package can read what it writes. Whatever MarshalChartXML produced
//     from a parsed chart must parse again.
//   - Fixed point. Parse is documented as lossy — a Chart is the builder's
//     model, not a faithful c:chartSpace — so the first marshal legitimately
//     drops what the model cannot hold. What it may not do is keep changing:
//     the second marshal must equal the first, byte for byte, and the two
//     parses must agree on every value the public API exposes. A reference that
//     grows on every save is exactly what this caught on real files (see
//     sheetOf and firstDataRef).
//
// A marshal that errors ends the case rather than failing it. Parse accepts a
// series whose cache holds no points — Excel writes one whenever the values
// come from a multi-area reference it chose not to cache — and MarshalChartXML
// rejects it, because a ptCount="0" cache pointing at no cells is a part Office
// reports as damaged. The asymmetry is real, but "everything Parse accepts is
// serializable" is not a promise this package has made.
func FuzzParseChartXML(f *testing.F) {
	for _, s := range chartSeeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if fuzzbound.Tripped() {
			return
		}
		var c1 *Chart
		var err error
		chartBudget.Check(t, len(data), func() {
			c1, err = Parse(data)
		})
		switch {
		case err != nil && c1 != nil:
			t.Fatalf("Parse returned both a chart and an error: %v", err)
		case err == nil && c1 == nil:
			t.Fatal("Parse returned neither a chart nor an error")
		case err != nil:
			return
		}

		out1, err := c1.MarshalChartXML()
		if err != nil {
			return // see the doc comment: not every parsable chart is serializable
		}
		c2, err := Parse(out1)
		if err != nil {
			t.Fatalf("MarshalChartXML wrote a part Parse cannot read: %v\n%s", err, out1)
		}
		out2, err := c2.MarshalChartXML()
		if err != nil {
			t.Fatalf("re-marshaling a chart parsed from our own output failed: %v\n%s", err, out1)
		}
		if string(out1) != string(out2) {
			t.Fatalf("MarshalChartXML is not a fixed point:\n%s\n\n%s", out1, out2)
		}
		assertChartsAgree(t, c1, c2, out1)
	})
}

// assertChartsAgree compares what the public API of a Chart exposes. Byte
// equality of the two marshals does not imply it: a value the serializer
// re-derives (the data-reference sheet, the number format) can differ in the
// model and still land on the same bytes, and it is the model a caller reads.
//
// Two fields are compared only when the first parse recovered one, because
// Parse is documented to re-emit at the builder's defaults what it could not
// read: a c:barChart with no c:grouping parses to no grouping, serializes as
// "clustered", and reads back as "clustered". That is the documented
// normalization, not drift — but a grouping the source *did* state must
// survive, and that is what is asserted.
func assertChartsAgree(t *testing.T, a, b *Chart, out []byte) {
	t.Helper()
	// A combination chart whose series all turn out to share one plot type and
	// one axis serializes as that single chart-type group, and reads back as
	// that kind: a "combo" with one group is that group. Any other change of
	// kind is a chart that came back as something else.
	collapsedCombo := a.Kind() == KindCombo && b.Kind().isComboMember()
	if a.Kind() != b.Kind() && !collapsedCombo {
		t.Fatalf("Kind changed across a round trip: %v -> %v\n%s", a.Kind(), b.Kind(), out)
	}
	if a.Title() != b.Title() {
		t.Fatalf("Title changed across a round trip: %q -> %q\n%s", a.Title(), b.Title(), out)
	}
	if a.DataRef != b.DataRef {
		t.Fatalf("DataRef changed across a round trip: %q -> %q\n%s", a.DataRef, b.DataRef, out)
	}
	if a.NumberFormat != "" && a.NumberFormat != b.NumberFormat {
		t.Fatalf("NumberFormat changed across a round trip: %q -> %q\n%s", a.NumberFormat, b.NumberFormat, out)
	}
	if a.Grouping() != "" && a.Grouping() != b.Grouping() {
		t.Fatalf("Grouping changed across a round trip: %q -> %q\n%s", a.Grouping(), b.Grouping(), out)
	}
	if a.DataLabels() != b.DataLabels() {
		t.Fatalf("DataLabels changed across a round trip: %v -> %v\n%s", a.DataLabels(), b.DataLabels(), out)
	}
	ac, av := a.AxisTitles()
	bc, bv := b.AxisTitles()
	if ac != bc || av != bv {
		t.Fatalf("AxisTitles changed across a round trip: (%q,%q) -> (%q,%q)\n%s", ac, av, bc, bv, out)
	}
	ap, aShown := a.LegendPos()
	bp, bShown := b.LegendPos()
	if ap != bp || aShown != bShown {
		t.Fatalf("LegendPos changed across a round trip: (%q,%v) -> (%q,%v)\n%s", ap, aShown, bp, bShown, out)
	}
	if !sameStrings(a.Categories(), b.Categories()) {
		t.Fatalf("Categories changed across a round trip: %q -> %q\n%s", a.Categories(), b.Categories(), out)
	}
	as, bs := a.SeriesList(), b.SeriesList()
	if len(as) != len(bs) {
		t.Fatalf("series count changed across a round trip: %d -> %d\n%s", len(as), len(bs), out)
	}
	// A series' plot type and axis assignment mean something only inside a
	// combination chart; a single-type chart carries them at the model's
	// defaults, so they are compared only when the chart stayed a combo.
	combo := a.Kind() == KindCombo && b.Kind() == KindCombo
	for i := range as {
		x, y := as[i], bs[i]
		if x.Name != y.Name || x.Color != y.Color ||
			!sameFloats(x.Values, y.Values) || !sameFloats(x.XValues, y.XValues) || !sameFloats(x.Sizes, y.Sizes) ||
			(combo && (x.PlotType != y.PlotType || x.SecondaryAxis != y.SecondaryAxis)) {
			t.Fatalf("series %d changed across a round trip:\n%+v\n%+v\n%s", i, x, y, out)
		}
	}
}

// sameStrings compares two label lists, treating a nil list and an empty one as
// the same absence.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameFloats compares two value lists element by element. A blank cached point
// is carried as a NaN sentinel (see Blank), and NaN != NaN, so the comparison
// has to be explicit about it: two blanks in the same slot are the same value.
func sameFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && !(math.IsNaN(a[i]) && math.IsNaN(b[i])) {
			return false
		}
	}
	return true
}

// marshalChartSpace renders a ChartSpace exactly as MarshalChartXML does.
func marshalChartSpace(cs *dmlchart.ChartSpace) (string, bool) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(xmlb.NSDrawingMLChart, xmlb.PrefixDrawingMLChart)
	b.RegisterNamespace(xmlb.NSDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	b.SetCollapseEmptyElements(true)
	b.SetSelfClosingSpace(false)
	b.WriteHeader()
	b.MarshalRoot(xmlb.NSDrawingMLChart, "chartSpace", cs, []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingMLChart, URI: xmlb.NSDrawingMLChart},
		{Prefix: xmlb.PrefixDrawingML, URI: xmlb.NSDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	})
	if err := b.Finish(); err != nil {
		return "", false
	}
	return b.String(), true
}

// FuzzChartSpaceRoundTrip drives the layer underneath chart.Parse: the
// dml-chart model, read by encoding/xml and written by the reflection Builder.
// Two serializers over one model is where this pair has gone wrong before —
// the Builder writes names through a prefix registry the stdlib decoder knows
// nothing about, and a name that resolves to the wrong prefix produces bytes
// that neither Builder.Finish nor a well-formedness check objects to.
//
// The oracle is a fixed point: the model the Builder's own output parses back
// into must serialize to the same bytes. Whatever the first pass normalizes
// away (elements the model does not type, attribute order) is legitimate; a
// value that keeps changing is the decoder and the Builder disagreeing about
// what a field means. Verified to hold on all 400 chart parts sampled from the
// corpus before this target was written.
func FuzzChartSpaceRoundTrip(f *testing.F) {
	for _, s := range chartSeeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if fuzzbound.Tripped() {
			return
		}
		var cs dmlchart.ChartSpace
		var err error
		chartBudget.Check(t, len(data), func() {
			err = xml.Unmarshal(data, &cs)
		})
		if err != nil {
			return
		}
		first, ok := marshalChartSpace(&cs)
		if !ok {
			t.Fatalf("a parsed chartSpace must marshal")
		}
		var back dmlchart.ChartSpace
		if err := xml.Unmarshal([]byte(first), &back); err != nil {
			t.Fatalf("the Builder's output must re-parse: %v\n%s", err, first)
		}
		second, ok := marshalChartSpace(&back)
		if !ok {
			t.Fatalf("the second marshal must succeed once the first did:\n%s", first)
		}
		if first != second {
			t.Fatalf("chartSpace marshal is not a fixed point:\n%s\n\n%s", first, second)
		}
	})
}
