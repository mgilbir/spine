package opc

import (
	"bytes"
	"testing"
	"time"
)

func TestCustomPropertiesSetGetTypes(t *testing.T) {
	cp := &CustomProperties{}
	tm := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)
	cases := map[string]any{
		"Str":    "hello",
		"Int":    int64(42),
		"BigInt": int64(5_000_000_000),
		"Float":  3.5,
		"Bool":   true,
		"Date":   tm,
	}
	for k, v := range cases {
		if err := cp.Set(k, v); err != nil {
			t.Fatalf("Set(%q): %v", k, err)
		}
	}
	// int and float32 coerce to int64/float64.
	if err := cp.Set("PlainInt", 7); err != nil {
		t.Fatal(err)
	}
	if err := cp.Set("F32", float32(1.5)); err != nil {
		t.Fatal(err)
	}
	if v, ok := cp.Get("PlainInt"); !ok || v != int64(7) {
		t.Errorf("PlainInt = %v (%T), want int64(7)", v, v)
	}
	if v, ok := cp.Get("F32"); !ok || v != float64(1.5) {
		t.Errorf("F32 = %v (%T), want float64(1.5)", v, v)
	}
	for k, want := range cases {
		got, ok := cp.Get(k)
		if !ok {
			t.Errorf("Get(%q) missing", k)
			continue
		}
		if wt, isTime := want.(time.Time); isTime {
			if !got.(time.Time).Equal(wt) {
				t.Errorf("Get(%q) = %v, want %v", k, got, wt)
			}
			continue
		}
		if got != want {
			t.Errorf("Get(%q) = %v, want %v", k, got, want)
		}
	}

	// Unsupported type is rejected.
	if err := cp.Set("Bad", []string{"x"}); err == nil {
		t.Error("Set with unsupported type: want error")
	}
	// Empty name is rejected.
	if err := cp.Set("", "x"); err == nil {
		t.Error("Set with empty name: want error")
	}
}

func TestCustomPropertiesMarshalRoundTrip(t *testing.T) {
	cp := &CustomProperties{}
	tm := time.Date(2020, 1, 5, 0, 0, 0, 0, time.UTC)
	_ = cp.Set("A", "with & <special>")
	_ = cp.Set("B", int64(-17))
	_ = cp.Set("C", 2.75)
	_ = cp.Set("D", false)
	_ = cp.Set("E", tm)
	_ = cp.Set("F", "") // empty string

	data, err := cp.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := UnmarshalCustomProperties(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !cp.Equal(parsed) {
		t.Errorf("round-trip mismatch\n in: %#v\nout: %#v", cp.props, parsed.props)
	}
	// pids are assigned sequentially from 2.
	if got := parsed.Names(); len(got) != 6 {
		t.Fatalf("Names len = %d, want 6", len(got))
	}
	// Re-marshaling the parsed set reproduces identical bytes.
	data2, _ := parsed.Marshal()
	if !bytes.Equal(data, data2) {
		t.Errorf("re-marshal drift\n%s\n%s", data, data2)
	}
}

func TestCustomPropertiesReplaceKeepsPID(t *testing.T) {
	cp := &CustomProperties{}
	_ = cp.Set("A", "one")
	_ = cp.Set("B", "two")
	// Replace A: pid must stay 2 (not jump to next free id).
	_ = cp.Set("A", "updated")
	data, _ := cp.Marshal()
	parsed, _ := UnmarshalCustomProperties(data)
	if v, _ := parsed.Get("A"); v != "updated" {
		t.Errorf("A = %v, want updated", v)
	}
	if len(parsed.props) != 2 {
		t.Fatalf("props = %d, want 2", len(parsed.props))
	}
	if parsed.props[0].pid != 2 || parsed.props[1].pid != 3 {
		t.Errorf("pids = %d,%d want 2,3", parsed.props[0].pid, parsed.props[1].pid)
	}
}

func TestCustomPropertiesRemove(t *testing.T) {
	cp := &CustomProperties{}
	_ = cp.Set("A", "one")
	_ = cp.Set("B", "two")
	if !cp.Remove("A") {
		t.Error("Remove(A) = false")
	}
	if cp.Remove("A") {
		t.Error("Remove(A) twice = true")
	}
	if _, ok := cp.Get("A"); ok {
		t.Error("A still present")
	}
	if cp.Len() != 1 {
		t.Errorf("Len = %d, want 1", cp.Len())
	}
}

func TestCustomPropertiesUnknownVariantPreserved(t *testing.T) {
	// A vt:vector value is not modeled; it must survive an unmarshal→marshal
	// cycle verbatim so a modify-and-save does not drop it.
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Properties xmlns="` + nsCustomProperties + `" xmlns:vt="` + nsDocPropsVTypes + `">` +
		`<property fmtid="` + CustomPropertiesFmtID + `" pid="2" name="Vec">` +
		`<vt:vector size="2" baseType="lpstr"><vt:lpstr>a</vt:lpstr><vt:lpstr>b</vt:lpstr></vt:vector>` +
		`</property></Properties>`)
	cp, err := UnmarshalCustomProperties(src)
	if err != nil {
		t.Fatal(err)
	}
	// The unmodeled value is not exposed through the typed map.
	if _, ok := cp.Get("Vec"); ok {
		t.Error("unmodeled value should not be typed")
	}
	out, _ := cp.Marshal()
	if !bytes.Equal(src, out) {
		t.Errorf("unknown variant not preserved\n in: %s\nout: %s", src, out)
	}
}

func TestCustomPropertiesEqualAndClone(t *testing.T) {
	cp := &CustomProperties{}
	_ = cp.Set("A", "one")
	clone := cp.Clone()
	if !cp.Equal(clone) {
		t.Error("clone not equal to original")
	}
	_ = clone.Set("A", "changed")
	if cp.Equal(clone) {
		t.Error("mutating clone affected equality (aliasing)")
	}
	if v, _ := cp.Get("A"); v != "one" {
		t.Errorf("original mutated: A = %v", v)
	}
}

func TestEnsureRelationshipInRels(t *testing.T) {
	rels := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Relationships xmlns="` + RelationshipsNamespace + `">` +
		`<Relationship Id="rId1" Type="` + RelTypeOfficeDocument + `" Target="ppt/presentation.xml"/>` +
		`</Relationships>`)
	out, rel, added := EnsureRelationshipInRels(rels, RelTypeCustom, "docProps/custom.xml")
	if !added || rel == nil {
		t.Fatal("expected relationship to be added")
	}
	if rel.ID != "rId2" {
		t.Errorf("assigned id = %q, want rId2", rel.ID)
	}
	if !bytes.Contains(out, []byte(`Target="docProps/custom.xml"`)) {
		t.Error("target not present in output")
	}
	// Original bytes preserved (only inserted before the closing tag).
	if !bytes.HasPrefix(out, rels[:len(rels)-len("</Relationships>")]) {
		t.Error("existing bytes were perturbed")
	}
	// Idempotent: a second call finds it present.
	out2, _, added2 := EnsureRelationshipInRels(out, RelTypeCustom, "docProps/custom.xml")
	if added2 {
		t.Error("second call should not add a duplicate")
	}
	if !bytes.Equal(out, out2) {
		t.Error("idempotent call changed bytes")
	}
}
