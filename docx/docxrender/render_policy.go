package docxrender

import (
	"context"
	"fmt"

	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// wordRenderer is the state of one document preparation: the options, the
// shared budgets and the strict versus best-effort policy.
//
// Strict mode (no opts.Warn) fails with render.ErrUnsupported on anything it
// cannot draw exactly. Best-effort mode (opts.Warn set) leaves unsupported
// content out and draws approximations, reporting each kind once: left-out
// content wraps render.ErrUnsupported, approximated content wraps
// render.ErrApproximated.
type wordRenderer struct {
	ctx    context.Context
	opts   render.Options
	limits render.Limits
	budget *core.SourceBudget
	// lenient is best-effort mode.
	lenient bool
	// reported dedupes warnings by their text.
	reported map[string]bool
	// textBytes counts the document text emitted to layout.
	textBytes int
	// maxTextBytes caps textBytes.
	maxTextBytes int

	theme  *wordTheme
	styles *wordStyles
	// colorMap is the settings part's colour mapping; nil selects the default.
	colorMap map[string]string
	// defaultTab is the default tab stop in pixels.
	defaultTab float64
	fonts      *wordFonts
	// evenOdd is w:evenAndOddHeaders: even pages have their own header and
	// footer.
	evenOdd bool
	// hf holds the header and footer parts (render_headers.go).
	hf *wordHF
	// notes is the footnote and endnote state (render_notes.go).
	notes *wordNotes
}

// wordIssueKind says how a feature the profile does not draw exactly is
// handled in best-effort mode.
type wordIssueKind uint8

const (
	// wordLeaveOut drops the feature (or the content carrying it).
	wordLeaveOut wordIssueKind = iota + 1
	// wordApproximate draws the content with the feature approximated.
	wordApproximate
)

// wordIssue is one feature the profile could not draw exactly.
type wordIssue struct {
	kind wordIssueKind
	what string
}

// leaveOut reports content that is not drawn. Strict mode fails; best-effort
// mode reports it once and returns nil.
func (r *wordRenderer) leaveOut(what string) error {
	return r.report(wordIssue{wordLeaveOut, what})
}

// approximate reports content that is drawn approximately.
func (r *wordRenderer) approximate(what string) error {
	return r.report(wordIssue{wordApproximate, what})
}

func (r *wordRenderer) report(i wordIssue) error {
	if !r.lenient {
		return fmt.Errorf("%w: docx: %s", render.ErrUnsupported, i.what)
	}
	var err error
	switch i.kind {
	case wordApproximate:
		err = fmt.Errorf("docx: %s: %w", i.what, render.ErrApproximated)
	default:
		err = fmt.Errorf("docx: %s left out: %w", i.what, render.ErrUnsupported)
	}
	key := err.Error()
	if r.reported[key] {
		return nil
	}
	if len(r.reported) >= 256 {
		// Bound the report set; the distinct kinds are a fixed small list,
		// so this only holds against a pathological source.
		return nil
	}
	r.reported[key] = true
	r.opts.Warn(err)
	return nil
}

// issues reports a list of issues, returning the first strict failure.
func (r *wordRenderer) issues(list []wordIssue) error {
	for _, i := range list {
		if err := r.report(i); err != nil {
			return err
		}
	}
	return nil
}

// addIssue appends an issue unless an identical one is present. The list is
// bounded by the number of distinct feature names the profile knows.
func wordAddIssue(list []wordIssue, i wordIssue) []wordIssue {
	for _, e := range list {
		if e == i {
			return list
		}
	}
	return append(list, i)
}

// charge accounts n nodes against the layout budget.
func (r *wordRenderer) charge(n int) error {
	if n > r.budget.Nodes {
		return render.ErrLimit
	}
	r.budget.Nodes -= n
	return nil
}

// chargeText accounts n bytes of emitted text.
func (r *wordRenderer) chargeText(n int) error {
	if n > r.maxTextBytes-r.textBytes {
		return render.ErrLimit
	}
	r.textBytes += n
	return nil
}
