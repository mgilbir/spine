package oxml

import (
	"encoding/xml"
	"fmt"
	"testing"
)

func TestPresentation_MarshalUnmarshal(t *testing.T) {
	p := &Presentation{
		XmlnsA: NsDrawingML,
		XmlnsR: NsRelationships,
		XmlnsP: NsPresentationML,
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
