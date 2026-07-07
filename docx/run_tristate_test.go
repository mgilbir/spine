package docx

import "testing"

// C59: SetBold(false) sets an explicit "off" (so inherited bold is turned off),
// and ClearBold removes the property to inherit from the style.
func TestRun_BoldTriState(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()

	// Explicit off must be present (not absent) so it overrides an inherited on.
	run.SetBold(false)
	if run.r.RPr == nil || run.r.RPr.B == nil {
		t.Fatal("SetBold(false) removed the w:b element instead of emitting an explicit off")
	}
	if run.r.RPr.B.IsOn() {
		t.Error("SetBold(false) should be off")
	}
	if run.Bold() {
		t.Error("Bold() should be false after SetBold(false)")
	}

	// Explicit on.
	run.SetBold(true)
	if !run.Bold() {
		t.Error("SetBold(true) should be bold")
	}

	// Inherit.
	run.ClearBold()
	if run.r.RPr.B != nil {
		t.Error("ClearBold should remove w:b so the run inherits from its style")
	}
}
