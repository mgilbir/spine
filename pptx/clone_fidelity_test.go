package pptx

import (
	"reflect"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// richParagraph returns a paragraph with every modeled property set to a
// distinctive non-zero value.
func richParagraph() *Paragraph {
	p := NewParagraph()
	p.AddRun().SetText("proto")
	p.SetAlignment(enum.TextAlignCenter)
	p.SetLevel(2)
	p.SetLineSpacing(150000)
	p.SetSpaceBefore(dml.Points(6))
	p.SetSpaceAfter(dml.Points(12))
	// Set the char bullet first: SetBulletAutoNumber switches the bullet kind but
	// leaves bulletChar populated, so both fields end up non-zero and the guard
	// below covers each of them.
	p.SetBulletChar("•")
	p.SetBulletAutoNumber(AutoNumberRomanUcPeriod, 5)
	p.SetBulletColor(dml.ColorRed)
	p.SetBulletSizePercent(80000)
	p.SetBulletFont("Wingdings")
	p.SetMarginLeft(dml.Inches(0.5))
	p.SetIndent(dml.Inches(-0.25))
	p.SetTabStops([]TabStop{{Position: dml.Inches(1), Align: TabAlignRight}})
	return p
}

// C415: CloneRow/CloneColumn/CloneShape advertise "full text formatting", but
// Paragraph.clone enumerated a subset of fields and silently dropped
// bulletColor, bulletSizePct, bulletFont, bulletAutoNumType,
// bulletAutoNumStartAt, marL, indent and tabStops — so a prototype cell with a
// roman-numeral auto-number starting at 5 cloned to arabicPeriod from 1.
func TestParagraphClone_KeepsRichTextProperties(t *testing.T) {
	src := richParagraph()
	got := src.clone()

	if got.BulletAutoNumberScheme() != AutoNumberRomanUcPeriod {
		t.Errorf("bullet scheme = %q, want %q", got.BulletAutoNumberScheme(), AutoNumberRomanUcPeriod)
	}
	if got.BulletAutoNumberStart() != 5 {
		t.Errorf("bullet start = %d, want 5", got.BulletAutoNumberStart())
	}
	if got.BulletFont() != "Wingdings" {
		t.Errorf("bullet font = %q, want Wingdings", got.BulletFont())
	}
	if got.BulletSizePercent() != 80000 {
		t.Errorf("bullet size = %d, want 80000", got.BulletSizePercent())
	}
	if got.BulletColor() == nil {
		t.Error("bullet color was dropped")
	}
	if got.MarginLeft() != dml.Inches(0.5) {
		t.Errorf("marL = %d, want %d", got.MarginLeft(), dml.Inches(0.5))
	}
	if got.Indent() != dml.Inches(-0.25) {
		t.Errorf("indent = %d, want %d", got.Indent(), dml.Inches(-0.25))
	}
	if len(got.TabStops()) != 1 || got.TabStops()[0].Align != TabAlignRight {
		t.Errorf("tab stops = %+v, want one right-aligned stop", got.TabStops())
	}
}

// C415: TextFrame.clone dropped autofit.
func TestTextFrameClone_KeepsAutofit(t *testing.T) {
	tf := NewTextFrame()
	tf.SetText("proto")
	tf.SetAutofit(AutofitNormal)

	if got := tf.clone().Autofit(); got != AutofitNormal {
		t.Errorf("cloned autofit = %v, want %v", got, AutofitNormal)
	}
}

// C415 (guard): every field of Paragraph must be carried over by clone, and no
// reference-typed field may be shared with the original. The clone helpers
// predate the rich-text wave that added seven of these fields; this fails if a
// newly added field is not copied, or is copied by aliasing.
func TestParagraphClone_CopiesEveryField(t *testing.T) {
	src := richParagraph()
	got := src.clone()

	sv, gv := reflect.ValueOf(*src), reflect.ValueOf(*got)
	rt := sv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		sf, gf := sv.Field(i), gv.Field(i)

		// The source field must be non-zero, or the case below proves nothing.
		if sf.IsZero() {
			t.Errorf("richParagraph leaves %s at its zero value; extend the fixture "+
				"so the clone guard actually covers it", f.Name)
			continue
		}

		switch f.Type.Kind() {
		case reflect.Ptr, reflect.Slice:
			// A reference field must be present and must not alias the original,
			// or editing the clone would reach back into it.
			if gf.IsZero() {
				t.Errorf("Paragraph.clone dropped %s", f.Name)
				continue
			}
			if sf.Pointer() == gf.Pointer() {
				t.Errorf("Paragraph.clone shares %s with the original instead of copying it", f.Name)
			}
		default:
			// Scalars are compared through the kind-specific accessors, which
			// (unlike Interface) work on unexported fields.
			if a, b := scalarOf(t, sf), scalarOf(t, gf); a != b {
				t.Errorf("Paragraph.clone dropped %s: got %v, want %v", f.Name, b, a)
			}
		}
	}
}

// scalarOf reduces a scalar reflect.Value to a comparable any. Interface()
// rejects values read from unexported fields, so the kind-specific accessors
// are used instead. A kind not handled here fails the test rather than passing
// silently, so a new field of an unexpected kind is noticed.
func scalarOf(t *testing.T, v reflect.Value) any {
	t.Helper()
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint()
	case reflect.Float32, reflect.Float64:
		return v.Float()
	default:
		t.Fatalf("clone guard cannot compare a field of kind %s; extend scalarOf", v.Kind())
		return nil
	}
}

// C415: mutating the clone must not reach back into the original.
func TestParagraphClone_IsIndependent(t *testing.T) {
	src := richParagraph()
	got := src.clone()

	got.SetMarginLeft(dml.Inches(3))
	got.SetBulletFont("Symbol")
	got.SetTabStops([]TabStop{{Position: dml.Inches(9)}})
	got.Runs()[0].SetText("edited")

	if src.MarginLeft() != dml.Inches(0.5) {
		t.Errorf("editing the clone changed the original's marL: %d", src.MarginLeft())
	}
	if src.BulletFont() != "Wingdings" {
		t.Errorf("editing the clone changed the original's bullet font: %q", src.BulletFont())
	}
	if len(src.TabStops()) != 1 || src.TabStops()[0].Position != dml.Inches(1) {
		t.Errorf("editing the clone changed the original's tab stops: %+v", src.TabStops())
	}
	if src.Runs()[0].Text() != "proto" {
		t.Errorf("editing the clone changed the original's run text: %q", src.Runs()[0].Text())
	}
}
