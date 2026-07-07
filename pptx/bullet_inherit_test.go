package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

// C135: a paragraph that sets alignment/level/spacing but not a bullet must not
// emit <a:buNone/> (which would suppress an inherited bullet); an explicit
// BulletNone still does.
func TestParagraphBullet_InheritVsNone(t *testing.T) {
	// Alignment set, bullet left at the default (inherit).
	inherit := NewParagraph()
	inherit.SetAlignment(enum.TextAlignCenter)
	ap := paragraphToOxml(inherit)
	if ap.PPr == nil {
		t.Fatal("expected PPr for an aligned paragraph")
	}
	if ap.PPr.BuNone != nil {
		t.Error("inherited-bullet paragraph emitted <a:buNone/>, suppressing the layout bullet")
	}
	if ap.PPr.BuChar != nil || ap.PPr.BuAutoNum != nil {
		t.Error("inherited-bullet paragraph emitted a bullet element")
	}

	// Explicit none.
	none := NewParagraph()
	none.SetBullet(BulletNone)
	if paragraphToOxml(none).PPr.BuNone == nil {
		t.Error("explicit BulletNone did not emit <a:buNone/>")
	}

	// BulletAuto is now serialized (previously silently unhandled).
	auto := NewParagraph()
	auto.SetBullet(BulletAuto)
	if paragraphToOxml(auto).PPr.BuAutoNum == nil {
		t.Error("BulletAuto did not emit <a:buAutoNum/>")
	}
}
