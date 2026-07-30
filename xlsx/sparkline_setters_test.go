package xlsx

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// The sparkline setters are eight near-identical colour writers and six
// near-identical flag writers, each differing from its neighbours only in the
// model slot its closure picks. That shape fails silently: SetLastColor writing
// the "first" slot, or SetHigh toggling "low", produces a workbook that opens
// fine and looks plausible, and a test that exercises one setter at a time with
// the same colour cannot tell the difference.
//
// The tests below therefore never assert on one slot in isolation: every
// setter is driven with a value distinct from every other setter's, and the
// whole slot vector is asserted after a save/reopen round trip, so a swap shows
// up as two named failures. sparklineSetterCoverage then derives the setter
// list from the type itself, so a ninth colour or a seventh flag added
// tomorrow fails until it is wired into the matrix.

// addTestSparklineGroup builds a one-sheet workbook carrying a single sparkline
// group and returns the workbook, sheet and a live handle on the group.
func addTestSparklineGroup(t *testing.T) (*Workbook, *Sheet, *SparklineGroup) {
	t.Helper()
	w := Create()
	s := addSheetT(w, "Data")
	for c, v := range []int{3, 1, 4, 1} {
		ref, err := CellRef(1, c+1)
		if err != nil {
			t.Fatalf("CellRef: %v", err)
		}
		if err := s.SetCellValue(ref, v); err != nil {
			t.Fatalf("SetCellValue: %v", err)
		}
	}
	g, err := s.AddSparklineGroup(SparklineOptions{
		Type: SparklineLine,
		Data: []SparklineData{{DataRange: "Data!A1:D1", LocationCell: "E1"}},
	})
	if err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}
	return w, s, g
}

// reopenSparklineGroup saves the workbook, reopens it and returns the first
// sheet's first sparkline group as parsed from the saved bytes, together with
// the saved worksheet XML. Reading the model back through a real save/reopen
// means a slot mix-up in the setter, in the marshaler or in the parser all
// surface here rather than being hidden by a symmetric in-memory read.
func reopenSparklineGroup(t *testing.T, w *Workbook) (*oxml.CT_SparklineGroup, string) {
	t.Helper()
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = re.Close() })
	groups := re.Sheets()[0].Sparklines()
	if len(groups) != 1 {
		t.Fatalf("reopened sparkline groups = %d, want 1", len(groups))
	}
	m := groups[0].resolve()
	if m == nil {
		t.Fatal("reopened sparkline group does not resolve")
	}
	return m, sheetXML
}

// sparklineColorSetter names one colour setter, the x14 child element it must
// write, the model slot that element is parsed back into, and a colour used by
// no other entry.
type sparklineColorSetter struct {
	method  string // exported setter name, checked against the type by reflection
	element string // x14 child element the colour must land in
	rgb     string // value unique to this setter
	slot    func(*oxml.CT_SparklineGroup) *oxml.SparklineColor
	set     func(*SparklineGroup, string)
}

func sparklineColorSetters() []sparklineColorSetter {
	return []sparklineColorSetter{
		{"SetSeriesColor", "colorSeries", "FF111111",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorSeries },
			func(g *SparklineGroup, h string) { g.SetSeriesColor(h) }},
		{"SetNegativeColor", "colorNegative", "FF222222",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorNegative },
			func(g *SparklineGroup, h string) { g.SetNegativeColor(h) }},
		{"SetAxisColor", "colorAxis", "FF333333",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorAxis },
			func(g *SparklineGroup, h string) { g.SetAxisColor(h) }},
		{"SetMarkersColor", "colorMarkers", "FF444444",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorMarkers },
			func(g *SparklineGroup, h string) { g.SetMarkersColor(h) }},
		{"SetFirstColor", "colorFirst", "FF555555",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorFirst },
			func(g *SparklineGroup, h string) { g.SetFirstColor(h) }},
		{"SetLastColor", "colorLast", "FF666666",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorLast },
			func(g *SparklineGroup, h string) { g.SetLastColor(h) }},
		{"SetHighColor", "colorHigh", "FF777777",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorHigh },
			func(g *SparklineGroup, h string) { g.SetHighColor(h) }},
		{"SetLowColor", "colorLow", "FF888888",
			func(m *oxml.CT_SparklineGroup) *oxml.SparklineColor { return m.ColorLow },
			func(g *SparklineGroup, h string) { g.SetLowColor(h) }},
	}
}

// Every sparkline colour setter writes its own slot: all eight are set to
// distinct colours in one pass, so any setter writing a neighbour's slot leaves
// two slots wrong and is named by the failure.
func TestSparklineColorSettersWriteDistinctSlots(t *testing.T) {
	w, _, g := addTestSparklineGroup(t)
	defer func() { _ = w.Close() }()

	setters := sparklineColorSetters()
	for _, sc := range setters {
		sc.set(g, sc.rgb)
	}

	m, sheetXML := reopenSparklineGroup(t, w)
	for _, sc := range setters {
		got := sc.slot(m)
		if got == nil {
			t.Errorf("%s: slot %s is unset after the setter ran", sc.method, sc.element)
			continue
		}
		if got.Rgb != sc.rgb {
			t.Errorf("%s: slot %s rgb = %q, want %q (a setter wrote the wrong slot)",
				sc.method, sc.element, got.Rgb, sc.rgb)
		}
		// The serialized element must carry the same colour: a mix-up in the
		// marshaler that the parser mirrors would be invisible above.
		want := `<x14:` + sc.element + ` rgb="` + sc.rgb + `"/>`
		if !strings.Contains(sheetXML, want) {
			t.Errorf("%s: saved sheet is missing %s", sc.method, want)
		}
	}
}

// A colour setter called with an empty string clears only its own slot; the
// others keep the colours they were given.
func TestSparklineColorSetterEmptyClearsOnlyItsOwnSlot(t *testing.T) {
	setters := sparklineColorSetters()
	for _, target := range setters {
		t.Run(target.method, func(t *testing.T) {
			w, _, g := addTestSparklineGroup(t)
			defer func() { _ = w.Close() }()
			for _, sc := range setters {
				sc.set(g, sc.rgb)
			}
			target.set(g, "")

			m, _ := reopenSparklineGroup(t, w)
			for _, sc := range setters {
				got := sc.slot(m)
				if sc.method == target.method {
					if got != nil {
						t.Errorf("%s(\"\") left slot %s set to %q", sc.method, sc.element, got.Rgb)
					}
					continue
				}
				if got == nil || got.Rgb != sc.rgb {
					t.Errorf("%s(\"\") also disturbed %s: got %v, want %q",
						target.method, sc.element, got, sc.rgb)
				}
			}
		})
	}
}

// A 6-digit colour is stored padded to 8-digit ARGB, the same normalization
// AddSparklineGroup applies to SeriesColor.
func TestSparklineColorSetterNormalizesShortHex(t *testing.T) {
	w, _, g := addTestSparklineGroup(t)
	defer func() { _ = w.Close() }()
	g.SetHighColor("#00b050")

	m, _ := reopenSparklineGroup(t, w)
	if m.ColorHigh == nil || m.ColorHigh.Rgb != "FF00B050" {
		t.Errorf("colorHigh = %v, want rgb FF00B050", m.ColorHigh)
	}
}

// sparklineFlagSetter names one boolean setter and the model slot it owns.
type sparklineFlagSetter struct {
	method string
	attr   string // sparklineGroup attribute the flag is written as
	slot   func(*oxml.CT_SparklineGroup) *bool
	set    func(*SparklineGroup, bool)
}

func sparklineFlagSetters() []sparklineFlagSetter {
	return []sparklineFlagSetter{
		{"SetMarkers", "markers",
			func(m *oxml.CT_SparklineGroup) *bool { return m.Markers },
			func(g *SparklineGroup, v bool) { g.SetMarkers(v) }},
		{"SetHigh", "high",
			func(m *oxml.CT_SparklineGroup) *bool { return m.High },
			func(g *SparklineGroup, v bool) { g.SetHigh(v) }},
		{"SetLow", "low",
			func(m *oxml.CT_SparklineGroup) *bool { return m.Low },
			func(g *SparklineGroup, v bool) { g.SetLow(v) }},
		{"SetFirst", "first",
			func(m *oxml.CT_SparklineGroup) *bool { return m.First },
			func(g *SparklineGroup, v bool) { g.SetFirst(v) }},
		{"SetLast", "last",
			func(m *oxml.CT_SparklineGroup) *bool { return m.Last },
			func(g *SparklineGroup, v bool) { g.SetLast(v) }},
		{"SetNegative", "negative",
			func(m *oxml.CT_SparklineGroup) *bool { return m.Negative },
			func(g *SparklineGroup, v bool) { g.SetNegative(v) }},
	}
}

// Each sparkline flag setter toggles exactly one flag. Booleans cannot be made
// mutually distinct by value, so distinctness comes from the pattern: every
// flag is written false and then one is flipped true, and the whole vector is
// asserted. SetHigh reaching the "low" slot flips the wrong entry and is named.
func TestSparklineFlagSettersToggleIndependently(t *testing.T) {
	setters := sparklineFlagSetters()
	for _, target := range setters {
		t.Run(target.method, func(t *testing.T) {
			w, _, g := addTestSparklineGroup(t)
			defer func() { _ = w.Close() }()
			for _, fs := range setters {
				fs.set(g, false)
			}
			target.set(g, true)

			m, sheetXML := reopenSparklineGroup(t, w)
			for _, fs := range setters {
				want := fs.method == target.method
				got := fs.slot(m)
				if got == nil {
					t.Errorf("%s: flag %s unset after all setters ran", fs.method, fs.attr)
					continue
				}
				if *got != want {
					t.Errorf("after %s(true): %s = %v, want %v (a setter toggled the wrong flag)",
						target.method, fs.attr, *got, want)
				}
			}
			if !strings.Contains(sheetXML, target.attr+`="1"`) {
				t.Errorf("%s(true): saved sheet has no %s=\"1\"", target.method, target.attr)
			}
		})
	}
}

// Markers is the one flag with a public getter; it must agree with the model
// slot SetMarkers writes, which pins the getter to the same slot as the setter.
func TestSparklineMarkersGetterMatchesSetter(t *testing.T) {
	w, _, g := addTestSparklineGroup(t)
	defer func() { _ = w.Close() }()
	g.SetMarkers(true)
	g.SetHigh(false)
	if !g.Markers() {
		t.Error("Markers() = false after SetMarkers(true)")
	}
	g.SetMarkers(false)
	g.SetHigh(true)
	if g.Markers() {
		t.Error("Markers() = true after SetMarkers(false) (reading the wrong slot?)")
	}
}

// Every setter on SparklineGroup must be covered by one of the matrices above.
// Deriving the list from the type means a setter added later fails this test
// until it is added to a matrix, rather than silently joining the untested set
// the whole file exists to close.
func TestSparklineSetterMatricesAreComplete(t *testing.T) {
	covered := map[string]bool{}
	for _, sc := range sparklineColorSetters() {
		covered[sc.method] = true
	}
	for _, fs := range sparklineFlagSetters() {
		covered[fs.method] = true
	}

	typ := reflect.TypeOf(&SparklineGroup{})
	found := 0
	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if !strings.HasPrefix(name, "Set") {
			continue
		}
		found++
		if !covered[name] {
			t.Errorf("(*SparklineGroup).%s is not covered by a setter matrix in this file", name)
		}
	}
	if found != len(covered) {
		t.Errorf("matrices cover %d setters but the type has %d", len(covered), found)
	}
}

// Setters on a handle whose group was deleted are no-ops rather than writes to
// detached memory, and they must not resurrect the extension.
func TestSparklineSettersOnDeletedGroupAreNoOps(t *testing.T) {
	w, s, g := addTestSparklineGroup(t)
	defer func() { _ = w.Close() }()
	g.Delete()

	g.SetLastColor("FF00FF00")
	g.SetLow(true)
	g.SetNegative(true)

	if groups := s.Sparklines(); len(groups) != 0 {
		t.Fatalf("setters on a deleted group resurrected it: %d groups", len(groups))
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml")); strings.Contains(sheetXML, "sparklineGroup") {
		t.Errorf("deleted sparkline group still serialized:\n%s", sheetXML)
	}
}
