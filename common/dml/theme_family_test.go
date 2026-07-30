package dml

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// The theme editor exposes two accessor *families*: twelve colour slots and six
// typeface slots, each a getter/setter pair that must address one specific
// a:clrScheme / a:fontScheme child and no other. Families written out by hand
// one member at a time fail by copy-paste — SetAccent4 assigning the accent3
// slot, Light2 reading lt1 — and a per-member test written the same way makes
// the same mistake in the same place, because it asserts only that the value
// came back from the accessor it was written through.
//
// These tests instead set every member of a family to a distinct value and then
// read *every* member, so a cross-wired pair shows up as two failures; and they
// re-serialize the theme and assert each value landed in its own element, which
// is what catches a getter and setter that are wrong the same way and therefore
// agree with each other.
//
// The member set is derived from the type by reflection and only then checked
// against the slot table, so an accent7 added tomorrow fails the guard until it
// is mapped, rather than silently going untested.

// themeColorSlots maps each ThemeColorScheme accessor-family member to the
// a:clrScheme child element it owns.
var themeColorSlots = map[string]string{
	"Dark1":             "dk1",
	"Light1":            "lt1",
	"Dark2":             "dk2",
	"Light2":            "lt2",
	"Accent1":           "accent1",
	"Accent2":           "accent2",
	"Accent3":           "accent3",
	"Accent4":           "accent4",
	"Accent5":           "accent5",
	"Accent6":           "accent6",
	"Hyperlink":         "hlink",
	"FollowedHyperlink": "folHlink",
}

// themeFontSlot names one typeface slot of one font collection.
type themeFontSlot struct {
	collection string // majorFont | minorFont
	element    string // latin | ea | cs
}

// themeFontSlots maps each ThemeFontScheme typeface accessor-family member to
// the (collection, slot) pair it owns. Name/SetName are not part of this family
// — they address the scheme's name attribute, not a typeface — and are covered
// by TestThemeNameAccessors.
var themeFontSlots = map[string]themeFontSlot{
	"MajorLatin":         {"majorFont", "latin"},
	"MinorLatin":         {"minorFont", "latin"},
	"MajorEastAsia":      {"majorFont", "ea"},
	"MinorEastAsia":      {"minorFont", "ea"},
	"MajorComplexScript": {"majorFont", "cs"},
	"MinorComplexScript": {"minorFont", "cs"},
}

// accessorFamily derives a type's getter/setter family for one value type:
// every method X() V that has a matching Set X(V). Deriving the set instead of
// listing it is what stops these tests falling behind the type.
func accessorFamily(typ, value reflect.Type) []string {
	getters := map[string]bool{}
	setters := map[string]bool{}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		sig := m.Type // method expression: receiver is In(0)
		if name, ok := strings.CutPrefix(m.Name, "Set"); ok {
			if sig.NumIn() == 2 && sig.NumOut() == 0 && sig.In(1) == value {
				setters[name] = true
			}
			continue
		}
		if sig.NumIn() == 1 && sig.NumOut() == 1 && sig.Out(0) == value {
			getters[m.Name] = true
		}
	}
	var out []string
	for name := range getters {
		if setters[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// requireSameMembers reports family members the slot table does not map and
// slot-table entries the type no longer has.
func requireSameMembers(t *testing.T, what string, derived, mapped []string) {
	t.Helper()
	have := map[string]bool{}
	for _, m := range derived {
		have[m] = true
	}
	want := map[string]bool{}
	for _, m := range mapped {
		want[m] = true
	}
	for _, m := range derived {
		if !want[m] {
			t.Errorf("%s has accessor family member %q that no test maps to an element; "+
				"add it to the slot table so it is exercised", what, m)
		}
	}
	for _, m := range mapped {
		if !have[m] {
			t.Errorf("%s no longer exposes %q, but the slot table still maps it; remove the entry",
				what, m)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// newThemeEditorForTest parses the shared theme fixture into a fresh editor.
func newThemeEditorForTest(t *testing.T) *ThemeEditor {
	t.Helper()
	var theme Theme
	if err := xmlb.Unmarshal([]byte(themeWithExtensions), &theme); err != nil {
		t.Fatalf("parse theme fixture: %v", err)
	}
	ed := NewThemeEditor(&theme, []byte(themeWithExtensions))
	if ed == nil {
		t.Fatal("NewThemeEditor returned nil for a parsed theme")
	}
	return ed
}

func TestThemeColorSchemeFamilyIsFullyMapped(t *testing.T) {
	derived := accessorFamily(reflect.TypeOf(&ThemeColorScheme{}), reflect.TypeOf(Color{}))
	if len(derived) == 0 {
		t.Fatal("derived no colour accessors; the guard would pass vacuously")
	}
	requireSameMembers(t, "ThemeColorScheme", derived, sortedKeys(themeColorSlots))
}

// Every colour slot is set to a distinct value, then every slot is read and
// every slot is looked for in the re-serialized part. A setter writing a
// neighbouring slot, or a getter reading one, changes two members at once and
// is reported by name.
func TestThemeColorSchemeSlotsAreIndependent(t *testing.T) {
	ed := newThemeEditorForTest(t)
	cs := ed.ColorScheme()
	if cs == nil {
		t.Fatal("theme fixture has an a:clrScheme but ColorScheme() returned nil")
	}
	v := reflect.ValueOf(cs)

	members := sortedKeys(themeColorSlots)
	want := make(map[string]RGB, len(members))
	for i, m := range members {
		// Distinct in every channel so a mis-wired slot cannot coincide.
		rgb := NewRGB(uint8(0x11*(i+1)), uint8(0x40+i), uint8(0xF0-3*i))
		want[m] = rgb
		v.MethodByName("Set" + m).Call([]reflect.Value{reflect.ValueOf(rgb.ToColor())})
	}

	for _, m := range members {
		got := v.MethodByName(m).Call(nil)[0].Interface().(Color)
		if got.RGB != want[m] {
			t.Errorf("%s() = %s, want %s: the getter/setter pair does not address the a:%s slot",
				m, got.RGB, want[m], themeColorSlots[m])
		}
	}

	if !ed.Modified() {
		t.Fatal("colour setters did not mark the theme modified; the edit would never be saved")
	}
	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal edited theme: %v", err)
	}
	got := string(out)
	for _, m := range members {
		el := themeColorSlots[m]
		frag := fmt.Sprintf(`<a:%s><a:srgbClr val="%s"/></a:%s>`, el, want[m], el)
		if !strings.Contains(got, frag) {
			t.Errorf("Set%s did not write a:%s: %s missing from the serialized theme", m, el, frag)
		}
	}
}

func TestThemeFontSchemeFamilyIsFullyMapped(t *testing.T) {
	derived := accessorFamily(reflect.TypeOf(&ThemeFontScheme{}), reflect.TypeOf(""))
	var typefaces []string
	for _, m := range derived {
		if m == "Name" {
			continue // the scheme name attribute, not a typeface slot
		}
		typefaces = append(typefaces, m)
	}
	if len(typefaces) == 0 {
		t.Fatal("derived no typeface accessors; the guard would pass vacuously")
	}
	requireSameMembers(t, "ThemeFontScheme", typefaces, sortedKeys(themeFontSlots))
}

// The font scheme has the same shape as the colour scheme and the same failure
// mode: six accessors over two collections × three slots, where
// SetMinorEastAsia writing majorFont/ea is a one-character mistake.
func TestThemeFontSchemeSlotsAreIndependent(t *testing.T) {
	ed := newThemeEditorForTest(t)
	fs := ed.FontScheme()
	if fs == nil {
		t.Fatal("theme fixture has an a:fontScheme but FontScheme() returned nil")
	}
	v := reflect.ValueOf(fs)

	members := sortedKeys(themeFontSlots)
	want := make(map[string]string, len(members))
	for _, m := range members {
		want[m] = "TF-" + m
		v.MethodByName("Set" + m).Call([]reflect.Value{reflect.ValueOf(want[m])})
	}

	for _, m := range members {
		got := v.MethodByName(m).Call(nil)[0].String()
		if got != want[m] {
			slot := themeFontSlots[m]
			t.Errorf("%s() = %q, want %q: the getter/setter pair does not address a:%s/a:%s",
				m, got, want[m], slot.collection, slot.element)
		}
	}

	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal edited theme: %v", err)
	}
	for _, m := range members {
		slot := themeFontSlots[m]
		body := elementBody(t, string(out), "a:"+slot.collection)
		frag := fmt.Sprintf(`<a:%s typeface="%s"/>`, slot.element, want[m])
		if !strings.Contains(body, frag) {
			t.Errorf("Set%s did not write a:%s/a:%s: %s missing from %s",
				m, slot.collection, slot.element, frag, body)
		}
	}
}

// elementBody returns the content between the first <name ...> and its </name>.
func elementBody(t *testing.T, doc, name string) string {
	t.Helper()
	open := strings.Index(doc, "<"+name)
	if open < 0 {
		t.Fatalf("element %s not found in %s", name, doc)
	}
	gt := strings.Index(doc[open:], ">")
	if gt < 0 {
		t.Fatalf("element %s start tag unterminated", name)
	}
	start := open + gt + 1
	end := strings.Index(doc[start:], "</"+name+">")
	if end < 0 {
		t.Fatalf("element %s has no end tag in %s", name, doc)
	}
	return doc[start : start+end]
}

// The four Name accessors are separate types reached through separate views;
// each must read and write its own element's name attribute, and a set that
// changes nothing must not mark the theme modified — a theme flagged modified
// re-serializes from the model and gives up byte identity for no reason.
func TestThemeNameAccessors(t *testing.T) {
	ed := newThemeEditorForTest(t)
	cs, fs, fmts := ed.ColorScheme(), ed.FontScheme(), ed.FormatScheme()
	if cs == nil || fs == nil || fmts == nil {
		t.Fatal("theme fixture views: clrScheme/fontScheme/fmtScheme must all be present")
	}

	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"ThemeEditor.Name", ed.Name(), "Office Theme"},
		{"ThemeColorScheme.Name", cs.Name(), "Office"},
		{"ThemeFontScheme.Name", fs.Name(), "Office"},
		{"ThemeFormatScheme.Name", fmts.Name(), "Office"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s() = %q, want %q", tc.what, tc.got, tc.want)
		}
	}

	// Re-setting the value already held must be a no-op.
	ed.SetName(ed.Name())
	cs.SetName(cs.Name())
	fs.SetName(fs.Name())
	if ed.Modified() {
		t.Error("setting a name to the value it already holds marked the theme modified; " +
			"an unmodified theme must round-trip from its source bytes")
	}

	ed.SetName("Theme X")
	cs.SetName("Colors X")
	fs.SetName("Fonts X")
	if !ed.Modified() {
		t.Fatal("name setters did not mark the theme modified")
	}
	if ed.Name() != "Theme X" || cs.Name() != "Colors X" || fs.Name() != "Fonts X" {
		t.Errorf("name getters disagree with the setters: %q / %q / %q",
			ed.Name(), cs.Name(), fs.Name())
	}

	out, err := ed.Marshal()
	if err != nil {
		t.Fatalf("marshal edited theme: %v", err)
	}
	got := string(out)
	for _, want := range []string{
		`<a:theme `, `name="Theme X"`,
		`<a:clrScheme name="Colors X">`,
		`<a:fontScheme name="Fonts X">`,
		`<a:fmtScheme name="Office">`, // untouched, and must stay untouched
	} {
		if !strings.Contains(got, want) {
			t.Errorf("edited theme is missing %s", want)
		}
	}
}

// themeSlotColor is the shared resolver behind every colour getter.
// TestThemeSlotColorResolvesMoreKinds already pins the four kinds it can
// resolve; this pins the documented *unresolvable* cases, which all return the
// zero Color and are therefore indistinguishable from opaque black. Each is a
// branch where a slot could start silently reporting a wrong colour instead of
// nothing, and no assertion elsewhere would notice.
func TestThemeSlotColorUnresolvableCases(t *testing.T) {
	tests := []struct {
		name string
		cc   *ColorChoice
	}{
		{"nil slot", nil},
		{"empty choice", &ColorChoice{}},
		{"srgbClr with an unparseable val", &ColorChoice{SrgbClr: &SrgbClr{Val: "nope"}}},
		{"sysClr with no lastClr rendering", &ColorChoice{SysClr: &SystemClr{Val: "windowText"}}},
		{"sysClr with an unparseable lastClr", &ColorChoice{SysClr: &SystemClr{Val: "x", LastClr: "zz"}}},
		{"schemeClr phClr, a placeholder not a colour", &ColorChoice{SchemeClr: &SchemeClrTransform{Val: "phClr"}}},
		{"schemeClr bg1, an alias the model has no slot for", &ColorChoice{SchemeClr: &SchemeClrTransform{Val: "bg1"}}},
		{"hslClr, an unmodeled kind", &ColorChoice{HslClr: &HslClr{}}},
		{"prstClr, an unmodeled kind", &ColorChoice{PrstClr: &PrstClr{Val: "black"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := themeSlotColor(tt.cc); got != (Color{}) {
				t.Errorf("themeSlotColor = %+v, want the zero Color: this slot is not resolvable "+
					"and must not be reported as a concrete colour", got)
			}
		})
	}
}

// parseThemeColorName is the schemeClr half of the resolver. Every ST_
// SchemeColorVal the model can represent must map to its own ThemeColor: two
// names sharing one constant would make two theme slots indistinguishable.
func TestParseThemeColorNameIsInjective(t *testing.T) {
	names := []string{"dk1", "lt1", "dk2", "lt2", "accent1", "accent2", "accent3",
		"accent4", "accent5", "accent6", "hlink", "folHlink"}
	seen := map[ThemeColor]string{}
	for _, n := range names {
		tc, ok := parseThemeColorName(n)
		if !ok {
			t.Errorf("parseThemeColorName(%q) not recognised", n)
			continue
		}
		if prev, dup := seen[tc]; dup {
			t.Errorf("parseThemeColorName(%q) and (%q) both map to %v", prev, n, tc)
		}
		seen[tc] = n
	}
	for _, n := range []string{"phClr", "bg1", "tx1", "bg2", "tx2", "", "Accent1"} {
		if _, ok := parseThemeColorName(n); ok {
			t.Errorf("parseThemeColorName(%q) reported representable; the model has no slot for it", n)
		}
	}
}

// pctToByte clamps and rounds an ST_Percentage channel; off-by-one here shifts
// every scrgbClr-defined theme colour.
func TestPctToByte(t *testing.T) {
	tests := []struct {
		in   int32
		want uint8
	}{
		{-1, 0}, {0, 0}, {1, 0}, {50000, 128}, {99999, 255}, {100000, 255}, {200000, 255},
		{25000, 64}, {75000, 191},
	}
	for _, tt := range tests {
		if got := pctToByte(NewPercentage(tt.in)); got != tt.want {
			t.Errorf("pctToByte(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
