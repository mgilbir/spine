package spectest

import (
	"reflect"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// attrCarrier is a stand-in for a modeled type that captures unmodeled
// attributes for round-trip replay via the conventional CapturedAttrs field.
type attrCarrier struct {
	Val           string          `xml:"val,attr"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// TestMissingCapturedAttrsCatchesDroppedReplay guards C338: the round-trip
// harness zeroes CapturedAttrs on both sides before DeepEqual, so an unmodeled
// attribute that a marshaler failed to replay would otherwise go unnoticed. The
// attribute-survival check must flag exactly that case.
func TestMissingCapturedAttrsCatchesDroppedReplay(t *testing.T) {
	src := &attrCarrier{
		Val: "5",
		CapturedAttrs: []xmlb.RootAttr{
			{IsNS: false, Prefix: "mc", LocalName: "Ignorable", Value: "x15"},
			// A namespace declaration must NOT be asserted (formatting only).
			{IsNS: true, Prefix: "mc", Value: "http://example/mc"},
		},
	}
	want := collectCapturedAttrs(reflect.ValueOf(src).Elem())
	if len(want) != 1 || want[0].LocalName != "Ignorable" {
		t.Fatalf("collectCapturedAttrs = %+v, want the single non-NS attr", want)
	}

	// A faithful marshaler replays the unmodeled attribute: nothing missing.
	faithful := []byte(`<w:e val="5" mc:Ignorable="x15" xmlns:mc="http://example/mc"/>`)
	if missing := missingCapturedAttrs(want, faithful); len(missing) != 0 {
		t.Errorf("faithful marshal reported missing attrs %v, want none", missing)
	}

	// A marshaler that DROPS the CapturedAttrs replay emits only the modeled
	// attribute; the check must catch the lost unmodeled attribute.
	dropped := []byte(`<w:e val="5"/>`)
	missing := missingCapturedAttrs(want, dropped)
	if len(missing) != 1 || missing[0] != "Ignorable=x15" {
		t.Errorf("dropped-replay marshal reported missing=%v, want [Ignorable=x15]", missing)
	}

	// Namespace reshuffling (different prefix) on the modeled/unmodeled attrs is
	// tolerated: local name + value still match.
	reshuffled := []byte(`<x:e val="5" alt:Ignorable="x15" xmlns:x="u" xmlns:alt="http://example/mc"/>`)
	if missing := missingCapturedAttrs(want, reshuffled); len(missing) != 0 {
		t.Errorf("namespace reshuffle reported missing %v, want none", missing)
	}
}
