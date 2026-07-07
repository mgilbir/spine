package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

func TestNewTextFrame(t *testing.T) {
	tf := NewTextFrame()
	if tf == nil {
		t.Fatal("NewTextFrame() returned nil")
	}

	if len(tf.Paragraphs()) != 0 {
		t.Errorf("Initial Paragraphs() has %d items, want 0", len(tf.Paragraphs()))
	}
}

func TestTextFrame_AddParagraph(t *testing.T) {
	tf := NewTextFrame()
	p := tf.AddParagraph()

	if p == nil {
		t.Fatal("AddParagraph() returned nil")
	}

	if len(tf.Paragraphs()) != 1 {
		t.Errorf("After AddParagraph, Paragraphs() has %d items, want 1", len(tf.Paragraphs()))
	}
}

func TestTextFrame_SetText(t *testing.T) {
	tf := NewTextFrame()
	tf.SetText("Hello World")

	if tf.Text() != "Hello World" {
		t.Errorf("Text() = %q, want %q", tf.Text(), "Hello World")
	}
}

func TestTextFrame_SetText_MultiLine(t *testing.T) {
	tf := NewTextFrame()
	tf.SetText("Line 1\nLine 2\nLine 3")

	if len(tf.Paragraphs()) != 3 {
		t.Errorf("Multi-line SetText created %d paragraphs, want 3", len(tf.Paragraphs()))
	}

	if tf.Text() != "Line 1\nLine 2\nLine 3" {
		t.Errorf("Text() = %q, want multi-line text", tf.Text())
	}
}

func TestTextFrame_Anchor(t *testing.T) {
	tf := NewTextFrame()

	// Default anchor should be top
	if tf.Anchor() != enum.TextAnchorTop {
		t.Errorf("Default Anchor() = %v, want TextAnchorTop", tf.Anchor())
	}

	tf.SetAnchor(enum.TextAnchorMiddle)
	if tf.Anchor() != enum.TextAnchorMiddle {
		t.Errorf("After SetAnchor, Anchor() = %v, want TextAnchorMiddle", tf.Anchor())
	}
}

func TestTextFrame_WordWrap(t *testing.T) {
	tf := NewTextFrame()

	if tf.WordWrap() != enum.TextWrappingSquare {
		t.Errorf("Default WordWrap() = %v, want TextWrappingSquare", tf.WordWrap())
	}

	tf.SetWordWrap(enum.TextWrappingNone)
	if tf.WordWrap() != enum.TextWrappingNone {
		t.Errorf("After SetWordWrap, WordWrap() = %v, want TextWrappingNone", tf.WordWrap())
	}
}

func TestTextFrame_Margins(t *testing.T) {
	tf := NewTextFrame()
	margins := tf.Margins()

	// Default margins should be set
	if margins.Left == 0 && margins.Right == 0 && margins.Top == 0 && margins.Bottom == 0 {
		t.Error("Default margins should not all be zero")
	}

	newMargins := TextMargins{
		Left:   dml.Inches(0.5),
		Top:    dml.Inches(0.25),
		Right:  dml.Inches(0.5),
		Bottom: dml.Inches(0.25),
	}
	tf.SetMargins(newMargins)

	if tf.Margins() != newMargins {
		t.Error("SetMargins did not update margins correctly")
	}
}

func TestNewParagraph(t *testing.T) {
	p := NewParagraph()
	if p == nil {
		t.Fatal("NewParagraph() returned nil")
	}

	if len(p.Runs()) != 0 {
		t.Errorf("Initial Runs() has %d items, want 0", len(p.Runs()))
	}
}

func TestParagraph_AddRun(t *testing.T) {
	p := NewParagraph()
	r := p.AddRun()

	if r == nil {
		t.Fatal("AddRun() returned nil")
	}

	if len(p.Runs()) != 1 {
		t.Errorf("After AddRun, Runs() has %d items, want 1", len(p.Runs()))
	}
}

func TestParagraph_Text(t *testing.T) {
	p := NewParagraph()
	p.AddRun().SetText("Hello ")
	p.AddRun().SetText("World")

	if p.Text() != "Hello World" {
		t.Errorf("Text() = %q, want %q", p.Text(), "Hello World")
	}
}

func TestParagraph_Alignment(t *testing.T) {
	p := NewParagraph()

	if p.Alignment() != enum.TextAlignLeft {
		t.Errorf("Default Alignment() = %v, want TextAlignLeft", p.Alignment())
	}

	p.SetAlignment(enum.TextAlignCenter)
	if p.Alignment() != enum.TextAlignCenter {
		t.Errorf("After SetAlignment, Alignment() = %v, want TextAlignCenter", p.Alignment())
	}
}

func TestParagraph_Level(t *testing.T) {
	p := NewParagraph()

	if p.Level() != 0 {
		t.Errorf("Default Level() = %d, want 0", p.Level())
	}

	p.SetLevel(3)
	if p.Level() != 3 {
		t.Errorf("After SetLevel(3), Level() = %d, want 3", p.Level())
	}

	// Test clamping
	p.SetLevel(-1)
	if p.Level() != 0 {
		t.Errorf("SetLevel(-1) should clamp to 0, got %d", p.Level())
	}

	p.SetLevel(10)
	if p.Level() != 8 {
		t.Errorf("SetLevel(10) should clamp to 8, got %d", p.Level())
	}
}

func TestParagraph_LineSpacing(t *testing.T) {
	p := NewParagraph()

	if p.LineSpacing() != 100000 {
		t.Errorf("Default LineSpacing() = %d, want 100000", p.LineSpacing())
	}

	p.SetLineSpacing(150000) // 150%
	if p.LineSpacing() != 150000 {
		t.Errorf("After SetLineSpacing, LineSpacing() = %d, want 150000", p.LineSpacing())
	}
}

func TestParagraph_SpaceBefore(t *testing.T) {
	p := NewParagraph()

	if p.SpaceBefore() != 0 {
		t.Errorf("Default SpaceBefore() = %d, want 0", p.SpaceBefore())
	}

	p.SetSpaceBefore(dml.Points(12))
	if p.SpaceBefore() != dml.Points(12) {
		t.Errorf("After SetSpaceBefore, SpaceBefore() = %d, want %d", p.SpaceBefore(), dml.Points(12))
	}
}

func TestParagraph_SpaceAfter(t *testing.T) {
	p := NewParagraph()

	p.SetSpaceAfter(dml.Points(6))
	if p.SpaceAfter() != dml.Points(6) {
		t.Errorf("After SetSpaceAfter, SpaceAfter() = %d, want %d", p.SpaceAfter(), dml.Points(6))
	}
}

func TestParagraph_Bullet(t *testing.T) {
	p := NewParagraph()

	// The default is BulletInherit so a paragraph keeps the layout's bullet
	// unless it explicitly sets one (BulletNone suppresses it).
	if p.Bullet() != BulletInherit {
		t.Errorf("Default Bullet() = %v, want BulletInherit", p.Bullet())
	}

	p.SetBullet(BulletAuto)
	if p.Bullet() != BulletAuto {
		t.Errorf("After SetBullet, Bullet() = %v, want BulletAuto", p.Bullet())
	}
}

func TestParagraph_BulletChar(t *testing.T) {
	p := NewParagraph()

	p.SetBulletChar("•")
	if p.BulletChar() != "•" {
		t.Errorf("BulletChar() = %q, want %q", p.BulletChar(), "•")
	}
	if p.Bullet() != BulletChar {
		t.Errorf("After SetBulletChar, Bullet() = %v, want BulletChar", p.Bullet())
	}
}

func TestNewRun(t *testing.T) {
	r := NewRun()
	if r == nil {
		t.Fatal("NewRun() returned nil")
	}

	// Default font size is unset (0) so the run inherits from its placeholder/
	// layout rather than clobbering it with an explicit size.
	if r.FontSize() != 0 {
		t.Errorf("Default FontSize() = %f, want 0 (unset)", r.FontSize())
	}
}

func TestRun_Text(t *testing.T) {
	r := NewRun()
	r.SetText("Hello")

	if r.Text() != "Hello" {
		t.Errorf("Text() = %q, want %q", r.Text(), "Hello")
	}
}

func TestRun_Font(t *testing.T) {
	r := NewRun()
	r.SetFont("Arial")

	if r.Font() != "Arial" {
		t.Errorf("Font() = %q, want %q", r.Font(), "Arial")
	}
}

func TestRun_FontSize(t *testing.T) {
	r := NewRun()
	r.SetFontSize(24)

	if r.FontSize() != 24 {
		t.Errorf("FontSize() = %f, want 24", r.FontSize())
	}
}

func TestRun_Bold(t *testing.T) {
	r := NewRun()

	if r.Bold() {
		t.Error("Default Bold() should be false")
	}

	r.SetBold(true)
	if !r.Bold() {
		t.Error("After SetBold(true), Bold() should be true")
	}
}

func TestRun_Italic(t *testing.T) {
	r := NewRun()

	if r.Italic() {
		t.Error("Default Italic() should be false")
	}

	r.SetItalic(true)
	if !r.Italic() {
		t.Error("After SetItalic(true), Italic() should be true")
	}
}

func TestRun_Underline(t *testing.T) {
	r := NewRun()

	if r.Underline() != enum.UnderlineNone {
		t.Errorf("Default Underline() = %v, want UnderlineNone", r.Underline())
	}

	r.SetUnderline(enum.UnderlineSingle)
	if r.Underline() != enum.UnderlineSingle {
		t.Errorf("After SetUnderline, Underline() = %v, want UnderlineSingle", r.Underline())
	}
}

func TestRun_Strike(t *testing.T) {
	r := NewRun()

	if r.Strike() != enum.StrikeNone {
		t.Errorf("Default Strike() = %v, want StrikeNone", r.Strike())
	}

	r.SetStrike(enum.StrikeSingle)
	if r.Strike() != enum.StrikeSingle {
		t.Errorf("After SetStrike, Strike() = %v, want StrikeSingle", r.Strike())
	}
}

func TestRun_Color(t *testing.T) {
	r := NewRun()

	if r.Color() != nil {
		t.Error("Default Color() should be nil")
	}

	r.SetColor(dml.ColorRed)
	if r.Color() == nil {
		t.Fatal("After SetColor, Color() should not be nil")
	}
	if r.Color().RGB != dml.ColorRed.RGB {
		t.Error("SetColor did not set correct color")
	}
}

func TestRun_Baseline(t *testing.T) {
	r := NewRun()

	if r.Baseline() != 0 {
		t.Errorf("Default Baseline() = %d, want 0", r.Baseline())
	}

	r.SetBaseline(30000)
	if r.Baseline() != 30000 {
		t.Errorf("After SetBaseline, Baseline() = %d, want 30000", r.Baseline())
	}
}

func TestRun_SetSuperscript(t *testing.T) {
	r := NewRun()
	r.SetSuperscript()

	if r.Baseline() != 30000 {
		t.Errorf("After SetSuperscript, Baseline() = %d, want 30000", r.Baseline())
	}
}

func TestRun_SetSubscript(t *testing.T) {
	r := NewRun()
	r.SetSubscript()

	if r.Baseline() != -30000 {
		t.Errorf("After SetSubscript, Baseline() = %d, want -30000", r.Baseline())
	}
}

func TestRun_Highlight(t *testing.T) {
	r := NewRun()

	if r.Highlight() != nil {
		t.Error("Default Highlight() should be nil")
	}

	r.SetHighlight(dml.ColorYellow)
	if r.Highlight() == nil {
		t.Fatal("After SetHighlight, Highlight() should not be nil")
	}
}
