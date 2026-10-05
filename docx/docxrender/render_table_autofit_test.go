package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

func TestTableAutofitColumnTooNarrowForAWordIsApproximated(t *testing.T) {
	// Word widens an autofit column for a word that does not fit; a fixed column
	// breaks the word, which is what Word does there.
	word := "wwwwwwwwww"
	autofit := wordTestBody(wordTestTable("", []int{600}, wordTestRow("", wordTestCell("", word))))
	_, err := Prepare(context.Background(), wordTestDoc(t, autofit, wordTestParts{styles: wordTestStyles}), newWordTestOpts(t, false).Options)
	if !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("autofit, strict: %v", err)
	}
	o := newWordTestOpts(t, true)
	wordTestPages(t, wordTestDoc(t, autofit, wordTestParts{styles: wordTestStyles}), o)
	// Layout names the word that does not fit.
	found := false
	for _, w := range o.warnings {
		found = found || (errors.Is(w, render.ErrApproximated) && strings.Contains(w.Error(), "unbreakable"))
	}
	if !found {
		t.Errorf("warnings %v", o.warnings)
	}
	fixed := wordTestBody(wordTestTable(wordTestFixed, []int{600}, wordTestRow("", wordTestCell("", word))))
	p, _ := wordTestRender(t, fixed)
	if n := len(p.lines(1)); n != 2 {
		t.Errorf("fixed column: %d lines", n)
	}
}
