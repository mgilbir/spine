package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// The four effect getters exist twice, once on AutoShape and once on TextBox,
// and the existing tests happened to cover Glow/Reflection/Bevel on AutoShape
// and SoftEdge on TextBox — i.e. exactly one receiver each. The table below
// runs both receivers through the same assertions so a getter wired to the
// wrong receiver's buffer, or to a neighbouring effect's field, fails.
//
// All four effects are set at once with mutually distinct magnitudes: if
// SoftEdge() read the glow radius it would report 5 instead of 3, which a
// one-effect-at-a-time test would miss (both would be nil, or both would
// happen to carry the same number).

const (
	effGlowRadius       = 5.0
	effSoftEdgeRadius   = 3.0
	effReflectionBlur   = 7.0
	effReflectionDist   = 11.0
	effBevelWidth       = 6.0
	effBevelHeight      = 9.0
	effReflectionStartA = 0.5
	effReflectionEndPos = 0.25
)

// effectShape is the common surface of the two receivers that carry effects.
type effectShape interface {
	Shape
	SetGlow(Glow)
	Glow() *Glow
	SetReflection(Reflection)
	Reflection() *Reflection
	SetSoftEdge(SoftEdge)
	SoftEdge() *SoftEdge
	SetBevel(Bevel)
	Bevel() *Bevel
}

var (
	_ effectShape = (*AutoShape)(nil)
	_ effectShape = (*TextBox)(nil)
)

func effectReceivers() []struct {
	name string
	make func() effectShape
} {
	return []struct {
		name string
		make func() effectShape
	}{
		{"AutoShape", func() effectShape {
			a := NewAutoShape(PresetRect)
			a.SetSize(dml.Inches(2), dml.Inches(1))
			return a
		}},
		{"TextBox", func() effectShape {
			tb := NewTextBox()
			tb.SetSize(dml.Inches(2), dml.Inches(1))
			tb.SetText("x")
			return tb
		}},
	}
}

// TestShapeEffectGettersPerReceiver sets all four effects on each receiver and
// checks each getter reports its own values, then checks the effects reached
// the saved slide part rather than only the in-memory struct.
func TestShapeEffectGettersPerReceiver(t *testing.T) {
	for _, rc := range effectReceivers() {
		t.Run(rc.name, func(t *testing.T) {
			sh := rc.make()

			// Nothing set: every getter must report absence, not a zero value.
			if got := sh.Glow(); got != nil {
				t.Errorf("Glow() before SetGlow = %+v, want nil", got)
			}
			if got := sh.Reflection(); got != nil {
				t.Errorf("Reflection() before SetReflection = %+v, want nil", got)
			}
			if got := sh.SoftEdge(); got != nil {
				t.Errorf("SoftEdge() before SetSoftEdge = %+v, want nil", got)
			}
			if got := sh.Bevel(); got != nil {
				t.Errorf("Bevel() before SetBevel = %+v, want nil", got)
			}

			red := dml.NewRGB(0xFF, 0x00, 0x00).ToColor()
			sh.SetGlow(Glow{Color: red, Radius: effGlowRadius})
			sh.SetSoftEdge(SoftEdge{Radius: effSoftEdgeRadius})
			sh.SetReflection(Reflection{
				BlurRadius:   effReflectionBlur,
				Distance:     effReflectionDist,
				StartOpacity: effReflectionStartA,
				EndPosition:  effReflectionEndPos,
			})
			sh.SetBevel(Bevel{Preset: dml.BevelCircle, Width: effBevelWidth, Height: effBevelHeight})

			glow := sh.Glow()
			if glow == nil {
				t.Fatal("Glow() = nil after SetGlow")
			}
			if glow.Radius != effGlowRadius {
				t.Errorf("Glow().Radius = %v, want %v", glow.Radius, effGlowRadius)
			}
			if glow.Color.RGB != dml.NewRGB(0xFF, 0x00, 0x00) {
				t.Errorf("Glow().Color = %+v, want FF0000", glow.Color)
			}

			se := sh.SoftEdge()
			if se == nil {
				t.Fatal("SoftEdge() = nil after SetSoftEdge")
			}
			if se.Radius != effSoftEdgeRadius {
				t.Errorf("SoftEdge().Radius = %v, want %v (the glow radius is %v)", se.Radius, effSoftEdgeRadius, effGlowRadius)
			}

			rf := sh.Reflection()
			if rf == nil {
				t.Fatal("Reflection() = nil after SetReflection")
			}
			if rf.BlurRadius != effReflectionBlur {
				t.Errorf("Reflection().BlurRadius = %v, want %v", rf.BlurRadius, effReflectionBlur)
			}
			if rf.Distance != effReflectionDist {
				t.Errorf("Reflection().Distance = %v, want %v", rf.Distance, effReflectionDist)
			}
			if rf.StartOpacity != effReflectionStartA {
				t.Errorf("Reflection().StartOpacity = %v, want %v", rf.StartOpacity, effReflectionStartA)
			}
			if rf.EndPosition != effReflectionEndPos {
				t.Errorf("Reflection().EndPosition = %v, want %v", rf.EndPosition, effReflectionEndPos)
			}

			bv := sh.Bevel()
			if bv == nil {
				t.Fatal("Bevel() = nil after SetBevel")
			}
			if bv.Preset != dml.BevelCircle {
				t.Errorf("Bevel().Preset = %q, want %q", bv.Preset, dml.BevelCircle)
			}
			// Width and height differ, so a transposed bevel reader fails.
			if bv.Width != effBevelWidth {
				t.Errorf("Bevel().Width = %v, want %v", bv.Width, effBevelWidth)
			}
			if bv.Height != effBevelHeight {
				t.Errorf("Bevel().Height = %v, want %v", bv.Height, effBevelHeight)
			}

			// The effects must reach the part, not just the struct.
			xml := marshalSlideWithShape(t, sh)
			for _, want := range []string{
				`<a:glow rad="63500"`,   // 5 pt
				`<a:softEdge rad="38100"`, // 3 pt
				`<a:reflection`,
				`<a:bevelT`,
			} {
				if !strings.Contains(xml, want) {
					t.Errorf("slide XML is missing %s:\n%s", want, xml)
				}
			}
			// The bevel's own dimensions, distinct from each other and from
			// every other effect's magnitude.
			if !strings.Contains(xml, `w="76200"`) || !strings.Contains(xml, `h="114300"`) {
				t.Errorf("a:bevelT w/h not emitted as 76200/114300:\n%s", xml)
			}
		})
	}
}
