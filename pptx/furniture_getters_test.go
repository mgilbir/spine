package pptx

import (
	"bytes"
	"testing"
)

func TestSlideFooterGetter(t *testing.T) {
	p := newDeckWithSlide()
	if text, ok := p.SlideFooter(); ok || text != "" {
		t.Errorf("fresh deck SlideFooter = (%q, %v), want (\"\", false)", text, ok)
	}

	p.SetSlideFooter("Confidential")
	text, ok := p.SlideFooter()
	if !ok || text != "Confidential" {
		t.Errorf("SlideFooter = (%q, %v), want (Confidential, true)", text, ok)
	}

	// Footer text is written as literal runs, so it survives a reopen.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	text, ok = p2.SlideFooter()
	if !ok || text != "Confidential" {
		t.Errorf("after reopen SlideFooter = (%q, %v), want (Confidential, true)", text, ok)
	}

	p.ClearSlideFooter()
	if _, ok := p.SlideFooter(); ok {
		t.Error("SlideFooter present after ClearSlideFooter")
	}
}

func TestSlideNumbersVisibleGetter(t *testing.T) {
	p := newDeckWithSlide()
	if p.SlideNumbersVisible() {
		t.Error("fresh deck should not show slide numbers")
	}

	p.ShowSlideNumbers(true)
	if !p.SlideNumbersVisible() {
		t.Error("SlideNumbersVisible = false after ShowSlideNumbers(true)")
	}

	// The slide-number placeholder survives a reopen (presence-based).
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if !p2.SlideNumbersVisible() {
		t.Error("SlideNumbersVisible = false after reopen")
	}

	p.ShowSlideNumbers(false)
	if p.SlideNumbersVisible() {
		t.Error("SlideNumbersVisible = true after ShowSlideNumbers(false)")
	}
}

func TestSlideDateGetters(t *testing.T) {
	p := newDeckWithSlide()
	if text, ok := p.SlideDate(); ok || text != "" {
		t.Errorf("fresh deck SlideDate = (%q, %v), want (\"\", false)", text, ok)
	}
	if p.SlideDateIsAuto() {
		t.Error("fresh deck SlideDateIsAuto = true")
	}

	// Fixed date text round-trips through reopen (literal runs).
	p.SetSlideDate("March 2026")
	text, ok := p.SlideDate()
	if !ok || text != "March 2026" {
		t.Errorf("SlideDate = (%q, %v), want (March 2026, true)", text, ok)
	}
	if p.SlideDateIsAuto() {
		t.Error("SlideDateIsAuto = true for a fixed date")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if text, ok := p2.SlideDate(); !ok || text != "March 2026" {
		t.Errorf("after reopen SlideDate = (%q, %v), want (March 2026, true)", text, ok)
	}
}

func TestSlideDateAutoGetter(t *testing.T) {
	p := newDeckWithSlide()
	p.SetSlideDateAuto()

	if _, ok := p.SlideDate(); !ok {
		t.Error("SlideDate presence = false after SetSlideDateAuto")
	}
	if !p.SlideDateIsAuto() {
		t.Error("SlideDateIsAuto = false after SetSlideDateAuto")
	}
	// An auto field carries no literal text.
	if text, _ := p.SlideDate(); text != "" {
		t.Errorf("auto SlideDate text = %q, want empty", text)
	}
}
