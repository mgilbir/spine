package pptx

import (
	"errors"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// Lookups are the classic place for a defect that a single happy-path call
// cannot see: returning the first entry regardless of the key, or returning a
// non-nil zero value instead of reporting a miss. Every test below therefore
// asserts a hit against a DISTINGUISHABLE candidate, a miss, and — where the
// deck holds several candidates — that the right one comes back.

// TestMasterLayoutByType looks every layout the master actually holds up by its
// own type and checks the returned layout is that one, then asserts a type the
// deck does not have is reported as ErrLayoutNotFound. The subject list is
// derived from Layouts(), so a layout added to the default deck tomorrow is
// covered by construction.
func TestMasterLayoutByType(t *testing.T) {
	p := Create()
	if len(p.SlideMasters()) != 1 {
		t.Fatalf("SlideMasters() = %d, want 1", len(p.SlideMasters()))
	}
	sm := p.SlideMasters()[0]
	layouts := sm.Layouts()
	if len(layouts) < 2 {
		t.Fatalf("the default master has %d layouts; the lookup tests need several distinguishable candidates", len(layouts))
	}

	for i, want := range layouts {
		got, err := sm.LayoutByType(want.Type())
		if err != nil {
			t.Errorf("LayoutByType(%q): %v", want.Type(), err)
			continue
		}
		if got != want {
			t.Errorf("LayoutByType(%q) returned layout %q (type %q), want the layout at index %d (%q)",
				want.Type(), got.Name(), got.Type(), i, want.Name())
		}
		if got.Type() != want.Type() {
			t.Errorf("LayoutByType(%q) returned a layout of type %q", want.Type(), got.Type())
		}
	}

	// Miss: LayoutComparison is not among the default layouts.
	for _, l := range layouts {
		if l.Type() == LayoutComparison {
			t.Fatal("LayoutComparison is now a default layout; pick another absent type for the miss case")
		}
	}
	got, err := sm.LayoutByType(LayoutComparison)
	if err == nil {
		t.Fatalf("LayoutByType(LayoutComparison) returned %v, want an error", got)
	}
	if !errors.Is(err, ErrLayoutNotFound) {
		t.Errorf("LayoutByType miss error = %v, want ErrLayoutNotFound", err)
	}
	if got != nil {
		t.Errorf("LayoutByType miss returned a non-nil layout %v", got)
	}
	if l := sm.GetLayout(LayoutComparison); l != nil {
		t.Errorf("GetLayout miss returned %v, want nil", l)
	}
}

// TestMasterLayoutByName is TestMasterLayoutByType for the name key: a lookup
// that ignored its argument would return layout 0 for every name and fail on
// the second iteration.
func TestMasterLayoutByName(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]
	layouts := sm.Layouts()

	seen := map[string]bool{}
	for _, want := range layouts {
		name := want.Name()
		if name == "" {
			t.Errorf("layout of type %q has an empty name; the name lookup cannot be tested against it", want.Type())
			continue
		}
		if seen[name] {
			t.Errorf("duplicate layout name %q: the by-name lookup cannot be proven to pick the right one", name)
		}
		seen[name] = true

		got, err := sm.LayoutByName(name)
		if err != nil {
			t.Errorf("LayoutByName(%q): %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("LayoutByName(%q) returned %q, want the layout of type %q", name, got.Name(), want.Type())
		}
	}

	got, err := sm.LayoutByName("No Such Layout")
	if err == nil {
		t.Fatalf("LayoutByName miss returned %v, want an error", got)
	}
	if !errors.Is(err, ErrLayoutNotFound) {
		t.Errorf("LayoutByName miss error = %v, want ErrLayoutNotFound", err)
	}
	if got != nil {
		t.Errorf("LayoutByName miss returned a non-nil layout %v", got)
	}
	if l := sm.GetLayoutByName("No Such Layout"); l != nil {
		t.Errorf("GetLayoutByName miss returned %v, want nil", l)
	}

	// The name key must be exact, not a prefix or a case-folded match.
	if _, err := sm.LayoutByName("title"); err == nil {
		t.Error(`LayoutByName("title") matched "Title Slide"; the lookup must be exact`)
	}
}

// TestLayoutPlaceholderLookup checks SlideLayout.Placeholder / GetPlaceholder
// on a layout with more than one placeholder, so returning the first entry
// regardless of the requested type is visible.
func TestLayoutPlaceholderLookup(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]

	// The title layout carries a centred title and a subtitle: two different
	// placeholder types in a known order.
	sl, err := sm.LayoutByType(LayoutTitle)
	if err != nil {
		t.Fatalf("LayoutByType(LayoutTitle): %v", err)
	}
	phs := sl.Placeholders()
	if len(phs) < 2 {
		t.Fatalf("the title layout has %d placeholders; need at least 2 distinguishable ones", len(phs))
	}

	for _, want := range phs {
		got, err := sl.Placeholder(want.PlaceholderType())
		if err != nil {
			t.Errorf("Placeholder(%q): %v", want.PlaceholderType(), err)
			continue
		}
		if got.PlaceholderType() != want.PlaceholderType() {
			t.Errorf("Placeholder(%q) returned a %q placeholder", want.PlaceholderType(), got.PlaceholderType())
		}
		if got.Index() != want.Index() {
			t.Errorf("Placeholder(%q) returned index %d, want %d", want.PlaceholderType(), got.Index(), want.Index())
		}
	}

	// Miss: the title layout has no chart placeholder.
	got, err := sl.Placeholder(PlaceholderChart)
	if err == nil {
		t.Fatalf("Placeholder(PlaceholderChart) returned %v, want an error", got)
	}
	if !errors.Is(err, ErrPlaceholderNotFound) {
		t.Errorf("Placeholder miss error = %v, want ErrPlaceholderNotFound", err)
	}
	if got != nil {
		t.Errorf("Placeholder miss returned a non-nil placeholder %v", got)
	}
	if ph := sl.GetPlaceholder(PlaceholderChart); ph != nil {
		t.Errorf("GetPlaceholder miss returned %v, want nil", ph)
	}

	// A layout with no placeholders at all must still report a clean miss.
	blank, err := sm.LayoutByType(LayoutBlank)
	if err != nil {
		t.Fatalf("LayoutByType(LayoutBlank): %v", err)
	}
	if len(blank.Placeholders()) != 0 {
		t.Fatalf("the blank layout unexpectedly has %d placeholders", len(blank.Placeholders()))
	}
	if _, err := blank.Placeholder(PlaceholderTitle); !errors.Is(err, ErrPlaceholderNotFound) {
		t.Errorf("blank layout Placeholder error = %v, want ErrPlaceholderNotFound", err)
	}
}

// TestMasterPlaceholderLookup is the master-side counterpart.
func TestMasterPlaceholderLookup(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]
	phs := sm.Placeholders()
	if len(phs) < 2 {
		t.Fatalf("the default master has %d placeholders; need at least 2", len(phs))
	}
	for _, want := range phs {
		got, err := sm.Placeholder(want.PlaceholderType())
		if err != nil {
			t.Errorf("Placeholder(%q): %v", want.PlaceholderType(), err)
			continue
		}
		if got.PlaceholderType() != want.PlaceholderType() {
			t.Errorf("Placeholder(%q) returned a %q placeholder", want.PlaceholderType(), got.PlaceholderType())
		}
	}
	got, err := sm.Placeholder(PlaceholderChart)
	if err == nil {
		t.Fatalf("Placeholder(PlaceholderChart) returned %v, want an error", got)
	}
	if !errors.Is(err, ErrPlaceholderNotFound) {
		t.Errorf("miss error = %v, want ErrPlaceholderNotFound", err)
	}
	if got != nil {
		t.Errorf("miss returned a non-nil placeholder %v", got)
	}
}

// TestEditablePlaceholderLookupAndIndex covers SlideMaster.EditablePlaceholder
// and EditablePlaceholder.Index. The two-content layout carries three
// placeholders at indices 0, 1 and 2, so an Index that returned a constant (or
// the shape's position rather than the p:ph idx) is visible.
func TestEditablePlaceholderLookupAndIndex(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]

	// Master side: title (idx 0) and body (idx 1) are different types AND
	// different indices, so a lookup keyed on the wrong field shows up.
	title := sm.EditablePlaceholder(PlaceholderTitle)
	if title == nil {
		t.Fatal("EditablePlaceholder(PlaceholderTitle) = nil on the default master")
	}
	if title.Type() != PlaceholderTitle {
		t.Errorf("EditablePlaceholder(PlaceholderTitle).Type() = %q", title.Type())
	}
	if title.Index() != 0 {
		t.Errorf("master title placeholder Index() = %d, want 0", title.Index())
	}
	body := sm.EditablePlaceholder(PlaceholderBody)
	if body == nil {
		t.Fatal("EditablePlaceholder(PlaceholderBody) = nil on the default master")
	}
	if body.Type() != PlaceholderBody {
		t.Errorf("EditablePlaceholder(PlaceholderBody).Type() = %q", body.Type())
	}
	if body.Index() != 1 {
		t.Errorf("master body placeholder Index() = %d, want 1", body.Index())
	}
	if title == body {
		t.Error("EditablePlaceholder returned the same handle for two different types")
	}
	if miss := sm.EditablePlaceholder(PlaceholderChart); miss != nil {
		t.Errorf("EditablePlaceholder(PlaceholderChart) = %v, want nil", miss)
	}

	// Layout side: the two-content layout has three placeholders whose p:ph
	// indices are 0, 1, 2 in shape order.
	sl, err := sm.LayoutByType(LayoutTwoContent)
	if err != nil {
		t.Fatalf("LayoutByType(LayoutTwoContent): %v", err)
	}
	eps := sl.EditablePlaceholders()
	if len(eps) != 3 {
		t.Fatalf("two-content layout has %d editable placeholders, want 3", len(eps))
	}
	for i, ep := range eps {
		if got := ep.Index(); got != uint32(i) {
			t.Errorf("editable placeholder %d: Index() = %d, want %d", i, got, i)
		}
	}

	// Index must track the p:ph idx that a setter writes, not the slice
	// position: move a placeholder and confirm Index is unchanged, then change
	// the underlying idx and confirm Index follows.
	eps[2].SetPosition(dml.Inches(1), dml.Inches(2))
	if got := eps[2].Index(); got != 2 {
		t.Errorf("after SetPosition, Index() = %d, want 2", got)
	}
	eps[2].ph().Idx = 7
	if got := eps[2].Index(); got != 7 {
		t.Errorf("Index() = %d after the p:ph idx was set to 7", got)
	}

	// A placeholder handle whose p:ph is missing reports the zero index rather
	// than panicking.
	bare := &EditablePlaceholder{sp: eps[0].sp, owner: p}
	bare.sp.NvSpPr.NvPr.Ph = nil
	if got := bare.Index(); got != 0 {
		t.Errorf("Index() with no p:ph = %d, want 0", got)
	}
	if got := bare.Type(); got != "" {
		t.Errorf("Type() with no p:ph = %q, want \"\"", got)
	}
}

// TestSlideMasterColorMap checks every field of the returned ColorMap maps to
// its own source value. The default map already distinguishes bg1/tx1/bg2/tx2
// and the six accents; the sentinel pass below makes all twelve unique so any
// duplicated or transposed assignment in the constructor fails.
func TestSlideMasterColorMap(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]

	cm := sm.ColorMap()
	if cm == nil {
		t.Fatal("ColorMap() = nil for the default master")
	}
	defaults := []struct {
		name string
		got  string
		want string
	}{
		{"Background1", cm.Background1, "lt1"},
		{"Text1", cm.Text1, "dk1"},
		{"Background2", cm.Background2, "lt2"},
		{"Text2", cm.Text2, "dk2"},
		{"Accent1", cm.Accent1, "accent1"},
		{"Accent2", cm.Accent2, "accent2"},
		{"Accent3", cm.Accent3, "accent3"},
		{"Accent4", cm.Accent4, "accent4"},
		{"Accent5", cm.Accent5, "accent5"},
		{"Accent6", cm.Accent6, "accent6"},
		{"Hyperlink", cm.Hyperlink, "hlink"},
		{"FollowedHyperlink", cm.FollowedHyperlink, "folHlink"},
	}
	for _, f := range defaults {
		if f.got != f.want {
			t.Errorf("ColorMap().%s = %q, want %q", f.name, f.got, f.want)
		}
	}

	// Sentinel pass: twelve distinct values, so a field read from the wrong
	// source cannot accidentally match.
	src := sm.masterXML.ClrMap
	src.Bg1, src.Tx1, src.Bg2, src.Tx2 = "s-bg1", "s-tx1", "s-bg2", "s-tx2"
	src.Accent1, src.Accent2, src.Accent3 = "s-a1", "s-a2", "s-a3"
	src.Accent4, src.Accent5, src.Accent6 = "s-a4", "s-a5", "s-a6"
	src.Hlink, src.FolHlink = "s-hlink", "s-folhlink"

	cm = sm.ColorMap()
	sentinels := map[string]string{
		"Background1": cm.Background1, "Text1": cm.Text1,
		"Background2": cm.Background2, "Text2": cm.Text2,
		"Accent1": cm.Accent1, "Accent2": cm.Accent2, "Accent3": cm.Accent3,
		"Accent4": cm.Accent4, "Accent5": cm.Accent5, "Accent6": cm.Accent6,
		"Hyperlink": cm.Hyperlink, "FollowedHyperlink": cm.FollowedHyperlink,
	}
	want := map[string]string{
		"Background1": "s-bg1", "Text1": "s-tx1",
		"Background2": "s-bg2", "Text2": "s-tx2",
		"Accent1": "s-a1", "Accent2": "s-a2", "Accent3": "s-a3",
		"Accent4": "s-a4", "Accent5": "s-a5", "Accent6": "s-a6",
		"Hyperlink": "s-hlink", "FollowedHyperlink": "s-folhlink",
	}
	seen := map[string]string{}
	for field, got := range sentinels {
		if got != want[field] {
			t.Errorf("ColorMap().%s = %q, want %q", field, got, want[field])
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("ColorMap().%s and .%s both read %q", prev, field, got)
		}
		seen[got] = field
	}

	// A master with no p:clrMap has no mapping to report.
	sm.masterXML.ClrMap = nil
	if got := sm.ColorMap(); got != nil {
		t.Errorf("ColorMap() with no p:clrMap = %+v, want nil", got)
	}
	sm.masterXML = nil
	if got := sm.ColorMap(); got != nil {
		t.Errorf("ColorMap() with no master XML = %+v, want nil", got)
	}
}
