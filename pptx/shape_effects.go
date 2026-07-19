package pptx

import (
	"math"

	"github.com/mgilbir/spine/common/dml"
)

// This file adds glow / reflection / soft-edge / 3D-bevel accessors to the
// text-bearing auto shape and text box, mirroring the SetShadow shape. Each
// setter routes through the matching dml value type's ApplyToSpPr, which stores
// the effect on the shape's a:effectLst (or a:sp3d) without disturbing effects
// already set. The getters report the effects set through this API on the
// shape; they do not resolve effects inherited from a style or theme.

// --- effect helpers (shared by AutoShape and TextBox) ---

// glowFromSpPr reconstructs a Glow from a shape's effect list, or nil.
func glowFromSpPr(spPr *dml.SpPr) *Glow {
	if spPr.EffectLst == nil || spPr.EffectLst.Glow == nil {
		return nil
	}
	g := spPr.EffectLst.Glow
	return &Glow{
		Color:  effectColor(g.SrgbClr, g.SchemeClr),
		Radius: float64(g.Rad) / 12700,
	}
}

// softEdgeFromSpPr reconstructs a SoftEdge from a shape's effect list, or nil.
func softEdgeFromSpPr(spPr *dml.SpPr) *SoftEdge {
	if spPr.EffectLst == nil || spPr.EffectLst.SoftEdge == nil {
		return nil
	}
	return &SoftEdge{Radius: float64(spPr.EffectLst.SoftEdge.Rad) / 12700}
}

// reflectionFromSpPr reconstructs a Reflection from a shape's effect list, or nil.
func reflectionFromSpPr(spPr *dml.SpPr) *Reflection {
	if spPr.EffectLst == nil || spPr.EffectLst.Reflection == nil {
		return nil
	}
	rf := spPr.EffectLst.Reflection
	r := &Reflection{}
	if rf.BlurRad != nil {
		r.BlurRadius = float64(*rf.BlurRad) / 12700
	}
	if rf.Dist != nil {
		r.Distance = float64(*rf.Dist) / 12700
	}
	if rf.Dir != nil {
		r.Direction = float64(*rf.Dir) / 60000
	}
	if rf.FadeDir != nil {
		r.FadeDirection = float64(*rf.FadeDir) / 60000
	}
	if rf.StA != nil {
		r.StartOpacity = pctToFraction(*rf.StA)
	}
	if rf.StPos != nil {
		r.StartPosition = pctToFraction(*rf.StPos)
	}
	if rf.EndA != nil {
		r.EndOpacity = pctToFraction(*rf.EndA)
	}
	if rf.EndPos != nil {
		r.EndPosition = pctToFraction(*rf.EndPos)
	}
	return r
}

// bevelFromSpPr reconstructs a Bevel from a shape's 3D properties, or nil.
func bevelFromSpPr(spPr *dml.SpPr) *Bevel {
	if spPr.Sp3d == nil || spPr.Sp3d.BevelT == nil {
		return nil
	}
	b := spPr.Sp3d.BevelT
	return &Bevel{
		Preset: b.Prst,
		Width:  float64(b.W) / 12700,
		Height: float64(b.H) / 12700,
	}
}

// effectColor reads a color from an effect's srgb/scheme color-choice pair,
// reusing the solid-fill color reader.
func effectColor(srgb *dml.SrgbClr, scheme *dml.SchemeClrTransform) dml.Color {
	if c := oxmlToColor(&dml.SolidFill{SrgbClr: srgb, SchemeClr: scheme}); c != nil {
		return *c
	}
	return dml.Color{}
}

// pctToFraction maps a 0..100000 ST_Percentage back to a 0..1 fraction.
func pctToFraction(p dml.Percentage) float64 {
	return math.Round(float64(p.Int32())/10) / 10000
}

// Glow aliases dml.Glow so callers can set glow effects without a second import.
type Glow = dml.Glow

// Reflection aliases dml.Reflection (see Glow).
type Reflection = dml.Reflection

// SoftEdge aliases dml.SoftEdge (see Glow).
type SoftEdge = dml.SoftEdge

// Bevel aliases dml.Bevel3D (see Glow).
type Bevel = dml.Bevel3D

// --- AutoShape effect accessors ---

// SetGlow sets a glow effect on the auto shape.
func (a *AutoShape) SetGlow(glow Glow) {
	glow.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// Glow returns the glow effect set on the auto shape, or nil when none was set.
func (a *AutoShape) Glow() *Glow { return glowFromSpPr(&a.spPr) }

// SetReflection sets a reflection effect on the auto shape.
func (a *AutoShape) SetReflection(reflection Reflection) {
	reflection.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// Reflection returns the reflection effect set on the auto shape, or nil.
func (a *AutoShape) Reflection() *Reflection { return reflectionFromSpPr(&a.spPr) }

// SetSoftEdge sets a soft-edge effect on the auto shape.
func (a *AutoShape) SetSoftEdge(softEdge SoftEdge) {
	softEdge.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// SoftEdge returns the soft-edge effect set on the auto shape, or nil.
func (a *AutoShape) SoftEdge() *SoftEdge { return softEdgeFromSpPr(&a.spPr) }

// SetBevel sets a basic 3D top bevel on the auto shape.
func (a *AutoShape) SetBevel(bevel Bevel) {
	bevel.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// Bevel returns the 3D top bevel set on the auto shape, or nil.
func (a *AutoShape) Bevel() *Bevel { return bevelFromSpPr(&a.spPr) }

// --- TextBox effect accessors ---

// SetGlow sets a glow effect on the text box.
func (t *TextBox) SetGlow(glow Glow) {
	glow.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// Glow returns the glow effect set on the text box, or nil when none was set.
func (t *TextBox) Glow() *Glow { return glowFromSpPr(&t.spPr) }

// SetReflection sets a reflection effect on the text box.
func (t *TextBox) SetReflection(reflection Reflection) {
	reflection.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// Reflection returns the reflection effect set on the text box, or nil.
func (t *TextBox) Reflection() *Reflection { return reflectionFromSpPr(&t.spPr) }

// SetSoftEdge sets a soft-edge effect on the text box.
func (t *TextBox) SetSoftEdge(softEdge SoftEdge) {
	softEdge.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// SoftEdge returns the soft-edge effect set on the text box, or nil.
func (t *TextBox) SoftEdge() *SoftEdge { return softEdgeFromSpPr(&t.spPr) }

// SetBevel sets a basic 3D top bevel on the text box.
func (t *TextBox) SetBevel(bevel Bevel) {
	bevel.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// Bevel returns the 3D top bevel set on the text box, or nil.
func (t *TextBox) Bevel() *Bevel { return bevelFromSpPr(&t.spPr) }
