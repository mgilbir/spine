package dml

import (
	"encoding/xml"
	"strings"
	"testing"
)

// C29: default-true boolean attributes must be able to express an explicit
// false — modeled as *bool so "0"/"false" is emitted rather than omitted (which
// a reader would treat as the default true).
func TestDefaultTrueBools_ExplicitFalseRoundTrips(t *testing.T) {
	f := false

	// a:path stroke/extrusionOk (custom encoding/xml marshal).
	p := &PathXML2D{Stroke: &f, ExtrusionOk: &f}
	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `stroke="0"`) || !strings.Contains(string(out), `extrusionOk="0"`) {
		t.Errorf("path stroke/extrusionOk false not emitted: %s", out)
	}
	var p2 PathXML2D
	if err := xml.Unmarshal(out, &p2); err != nil {
		t.Fatal(err)
	}
	if p2.Stroke == nil || *p2.Stroke {
		t.Error("path stroke=false lost on round-trip")
	}

	// a:blur grow.
	blur := &BlurXML{Rad: 100, Grow: &f}
	if out, _ := xml.Marshal(blur); !strings.Contains(string(out), "grow=") {
		t.Errorf("blur grow false omitted: %s", out)
	} else {
		var b2 BlurXML
		_ = xml.Unmarshal(out, &b2)
		if b2.Grow == nil || *b2.Grow {
			t.Error("blur grow=false lost on round-trip")
		}
	}
}
