package oxml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

const compatNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

func marshalCompat(c *CT_Compat) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	c.MarshalToBuilder(b, xmlb.NSWordprocessingML, "compat")
	return b.Bytes()
}

// C352: CT_Compat marshaled the flag options by ranging over a map, so the
// output element order was random per marshal (violating the deterministic-
// output property). Marshaling the same parsed value repeatedly must be
// byte-identical.
func TestCTCompat_DeterministicMarshal(t *testing.T) {
	src := `<w:compat ` + compatNS + `>` +
		`<w:spaceForUL/><w:balanceSingleByteDoubleByteWidth/>` +
		`<w:doNotLeaveBackslashAlone w:val="0"/><w:ulTrailSpace/>` +
		`<w:doNotExpandShiftReturn/><w:adjustLineHeightInTable/>` +
		`</w:compat>`
	var c CT_Compat
	if err := xml.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	first := marshalCompat(&c)
	for i := 0; i < 50; i++ {
		if got := marshalCompat(&c); !bytes.Equal(got, first) {
			t.Fatalf("nondeterministic marshal on iter %d:\n%s\nvs\n%s", i, first, got)
		}
	}
}

// C352: source order and non-val attributes on compat flags must survive a
// round-trip. The old map-based marshal dropped every attribute except w:val
// and reordered the flags.
func TestCTCompat_PreservesOrderAndAttrs(t *testing.T) {
	src := `<w:compat ` + compatNS + `>` +
		`<w:zzzLast/><w:aaaFirst w:val="1" w:extra="keep"/>` +
		`</w:compat>`
	var c CT_Compat
	if err := xml.Unmarshal([]byte(src), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out := string(marshalCompat(&c))
	iZ, iA := strings.Index(out, "zzzLast"), strings.Index(out, "aaaFirst")
	if iZ < 0 || iA < 0 || iZ > iA {
		t.Errorf("flag order not preserved (zzzLast before aaaFirst): %s", out)
	}
	if !strings.Contains(out, `w:extra="keep"`) {
		t.Errorf("non-val attribute dropped: %s", out)
	}
}
