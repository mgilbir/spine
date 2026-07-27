package oxml

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// A sldMasterId id of 0 is invalid (ST_SlideMasterId requires >= 2147483648), so
// the reflection safety-net marshaler omits it rather than emitting id="0" (C355).
func TestSlideMasterID_MarshalToBuilder_OmitsZeroID(t *testing.T) {
	b := xmlb.NewPresentationMLBuilder()
	SlideMasterID{ID: 0, RID: "rId1"}.MarshalToBuilder(b, xmlb.NSPresentationML, "sldMasterId")
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()
	if strings.Contains(out, `id="0"`) {
		t.Errorf("emitted invalid id=\"0\":\n%s", out)
	}
	if !strings.Contains(out, `r:id="rId1"`) {
		t.Errorf("dropped r:id:\n%s", out)
	}

	// A valid high id is still emitted.
	b2 := xmlb.NewPresentationMLBuilder()
	SlideMasterID{ID: 2147483648, RID: "rId2"}.MarshalToBuilder(b2, xmlb.NSPresentationML, "sldMasterId")
	if err := b2.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got := b2.String(); !strings.Contains(got, `id="2147483648"`) {
		t.Errorf("valid id not emitted:\n%s", got)
	}
}

func TestPresentation_MarshalUnmarshal(t *testing.T) {
	p := &Presentation{
		SlideSize: &SlideSize{Cx: 9144000, Cy: 6858000},
		SlideIDs: &SlideIDs{
			SlideID: []SlideID{
				{ID: 256, RID: "rId1"},
			},
		},
	}

	data, err := xml.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	fmt.Println("Marshaled XML:")
	fmt.Println(string(data))

	var p2 Presentation
	err = xml.Unmarshal(data, &p2)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if p2.SlideIDs == nil || len(p2.SlideIDs.SlideID) == 0 {
		t.Fatal("SlideIDs not unmarshaled")
	}

	t.Logf("Unmarshaled RID: %q", p2.SlideIDs.SlideID[0].RID)
}
