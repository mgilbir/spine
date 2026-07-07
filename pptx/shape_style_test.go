package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// C21: an AutoShape's gradient and pattern fills survive conversion to XML
// (previously only SolidFill was carried).
func TestAutoShapeToOxml_NonSolidFills(t *testing.T) {
	grad := &AutoShape{presetGeometry: "rect"}
	grad.SetFill(dml.NewGradientFill(45,
		dml.GradientStop{Position: 0, Color: dml.ColorRed},
		dml.GradientStop{Position: 1, Color: dml.ColorBlue}))
	sp := autoShapeToOxml(grad, 2)
	if sp.SpPr.GradFill == nil {
		t.Error("gradient fill dropped from AutoShape")
	}
	if sp.SpPr.SolidFill != nil {
		t.Error("unexpected solid fill on gradient shape")
	}

	pat := &AutoShape{presetGeometry: "rect"}
	pat.SetFill(dml.NewPatternFill("pct50", dml.ColorRed, dml.ColorWhite))
	if autoShapeToOxml(pat, 3).SpPr.PattFill == nil {
		t.Error("pattern fill dropped from AutoShape")
	}

	none := &AutoShape{presetGeometry: "rect"}
	none.SetFill(dml.NewNoFill())
	if autoShapeToOxml(none, 4).SpPr.NoFill == nil {
		t.Error("no-fill dropped from AutoShape")
	}
}

// C22: TextBox SetFill/SetLine/SetShadow reach the XML (textBoxToOxml
// previously ignored the shape's SpPr entirely).
func TestTextBoxToOxml_FillLineShadow(t *testing.T) {
	tb := &TextBox{}
	tb.SetFill(dml.NewSolidFill(dml.ColorRed))
	tb.SetLine(dml.Line{Width: 2, Color: dml.ColorBlue})
	tb.SetShadow(dml.Shadow{Color: dml.ColorBlack, BlurRad: 3, Distance: 2, Angle: 45})

	sp := textBoxToOxml(tb, 2)
	if sp.SpPr.SolidFill == nil {
		t.Error("TextBox fill dropped")
	}
	if sp.SpPr.Ln == nil {
		t.Error("TextBox line dropped")
	}
	if sp.SpPr.EffectLst == nil || sp.SpPr.EffectLst.OuterShdw == nil {
		t.Error("TextBox shadow dropped")
	}
}
