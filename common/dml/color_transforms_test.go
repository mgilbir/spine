package dml

import (
	"reflect"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

func TestColorTransformsFollowWriteOrder(t *testing.T) {
	parse := func(t *testing.T, s string) *SolidFill {
		t.Helper()
		var f SolidFill
		if err := xmlb.UnmarshalWithSource([]byte(`<a:solidFill xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`+s+`</a:solidFill>`), &f); err != nil {
			t.Fatal(err)
		}
		return &f
	}
	lumModOff := []ColorTransformStep{{Name: "lumMod", Val: NewPercentage(75000)}, {Name: "lumOff", Val: NewPercentage(25000)}}
	lumOffMod := []ColorTransformStep{{Name: "lumOff", Val: NewPercentage(25000)}, {Name: "lumMod", Val: NewPercentage(75000)}}
	for _, tc := range []struct {
		name  string
		steps func() ([]ColorTransformStep, bool)
		want  []ColorTransformStep
		ok    bool
	}{
		{"scheme source order", parse(t, `<a:schemeClr val="bg1"><a:lumMod val="75000"/><a:lumOff val="25000"/></a:schemeClr>`).SchemeClr.Transforms, lumModOff, true},
		{"scheme reversed source order", parse(t, `<a:schemeClr val="bg1"><a:lumOff val="25000"/><a:lumMod val="75000"/></a:schemeClr>`).SchemeClr.Transforms, lumOffMod, true},
		{"rgb source order", parse(t, `<a:srgbClr val="FFFFFF"><a:lumOff val="25000"/><a:lumMod val="75000"/></a:srgbClr>`).SrgbClr.Transforms, lumOffMod, true},
		{"rgb arg-less", parse(t, `<a:srgbClr val="FFFFFF"><a:inv/></a:srgbClr>`).SrgbClr.Transforms, []ColorTransformStep{{Name: "inv"}}, true},
		{"rgb repeated transform", parse(t, `<a:srgbClr val="FFFFFF"><a:lumMod val="75000"/><a:lumMod val="50000"/></a:srgbClr>`).SrgbClr.Transforms, nil, false},
		{"rgb unknown child", parse(t, `<a:srgbClr val="FFFFFF"><a:future val="1"/></a:srgbClr>`).SrgbClr.Transforms, nil, false},
		{"system color", parse(t, `<a:sysClr val="windowText" lastClr="000000"><a:lumMod val="75000"/><a:lumOff val="25000"/></a:sysClr>`).SysClr.Transforms, lumModOff, true},
		{"built rgb uses field order", (&SrgbClr{Val: "FFFFFF", LumOff: &ColorTransform{Val: NewPercentage(25000)}, LumMod: &ColorTransform{Val: NewPercentage(75000)}}).Transforms, lumModOff, true},
		// A built scheme color is written in XSD declaration order, which puts
		// lumOff first; the steps report what a save writes.
		{"built scheme uses declaration order", (&SchemeClrTransform{Val: "bg1", LumMod: []*ColorTransform{{Val: NewPercentage(75000)}}, LumOff: []*ColorTransform{{Val: NewPercentage(25000)}}}).Transforms, lumOffMod, true},
		{"nil", (*SchemeClrTransform)(nil).Transforms, nil, true},
	} {
		got, ok := tc.steps()
		if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: %+v %v, want %+v %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestFillStyleListEntryUsesDocumentOrder(t *testing.T) {
	var l BgFillStyleLst
	if err := xmlb.UnmarshalWithSource([]byte(`<a:bgFillStyleLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:gradFill/><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:noFill/></a:bgFillStyleLst>`), &l); err != nil {
		t.Fatal(err)
	}
	if e, ok := l.Entry(0); !ok || e.GradFill == nil {
		t.Fatalf("entry 0: %+v", e)
	}
	if e, ok := l.Entry(1); !ok || e.SolidFill == nil || e.SolidFill.SchemeClr.Val != "phClr" {
		t.Fatalf("entry 1: %+v", e)
	}
	if e, ok := l.Entry(2); !ok || e.NoFill == nil {
		t.Fatalf("entry 2: %+v", e)
	}
	for _, i := range []int{-1, 3} {
		if _, ok := l.Entry(i); ok {
			t.Fatalf("entry %d in range", i)
		}
	}
	if _, ok := (*FillStyleLst)(nil).Entry(0); ok {
		t.Fatal("nil list has entries")
	}
}
