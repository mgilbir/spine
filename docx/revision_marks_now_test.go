package docx

import (
	"strings"
	"testing"
	"time"
)

// The four "now" revision authoring entry points — Run.MarkInserted,
// Run.MarkDeleted, Paragraph.AddMoveFromRun and Paragraph.AddMoveToRun — are
// one-line delegations to their *WithDate twins. That is exactly the shape that
// rots silently: MarkInserted delegating to MarkDeletedWithDate, or
// AddMoveToRun passing "moveFrom", produces markup that still parses, still
// round-trips, and is the wrong tracked change.
//
// Each case below therefore authors through the now-variant, saves, reopens,
// and reads the revision back: the kind, the author, the affected text, and a
// timestamp that has to be recent and expressed in UTC.

// revisionMarkCase is one now-variant entry point.
type revisionMarkCase struct {
	name string
	// author applies the entry point to a paragraph that already holds text.
	author func(p *Paragraph, who string)
	// wantKind is the revision type Document.Revisions must report.
	wantKind RevisionType
	// wantText is the text the revision must carry.
	wantText string
	// wantXML must appear in word/document.xml.
	wantXML []string
	// notXML must not — the twin's markup, so a delegation to the wrong
	// variant fails even if the reader were also wrong.
	notXML []string
	// moveName is the expected MoveName, "" for non-move revisions.
	moveName string
}

const revisionAuthor = "Ada L"

func revisionMarkCases() []revisionMarkCase {
	return []revisionMarkCase{
		{
			name: "MarkInserted",
			author: func(p *Paragraph, who string) {
				p.Runs()[0].MarkInserted(who)
			},
			wantKind: RevisionInsertion,
			wantText: "subject",
			wantXML:  []string{"<w:ins ", "<w:t"},
			// An insertion must not convert the run's text to deletion text.
			notXML: []string{"<w:del ", "<w:delText"},
		},
		{
			name: "MarkDeleted",
			author: func(p *Paragraph, who string) {
				p.Runs()[0].MarkDeleted(who)
			},
			wantKind: RevisionDeletion,
			wantText: "subject",
			// A deletion has to rewrite w:t as w:delText; a w:t left inside a
			// w:del is schema-invalid and Word drops the text.
			wantXML: []string{"<w:del ", "<w:delText"},
			notXML:  []string{"<w:ins "},
		},
		{
			name: "AddMoveFromRun",
			author: func(p *Paragraph, who string) {
				p.AddMoveFromRun(who, "move-1", "relocated")
			},
			wantKind: RevisionMoveFrom,
			wantText: "relocated",
			wantXML: []string{
				"<w:moveFromRangeStart ", "<w:moveFrom ", "<w:moveFromRangeEnd ",
				`w:name="move-1"`,
			},
			notXML:   []string{"<w:moveTo "},
			moveName: "move-1",
		},
		{
			name: "AddMoveToRun",
			author: func(p *Paragraph, who string) {
				p.AddMoveToRun(who, "move-1", "relocated")
			},
			wantKind: RevisionMoveTo,
			wantText: "relocated",
			wantXML: []string{
				"<w:moveToRangeStart ", "<w:moveTo ", "<w:moveToRangeEnd ",
				`w:name="move-1"`,
			},
			notXML:   []string{"<w:moveFrom "},
			moveName: "move-1",
		},
	}
}

func TestRevisionMarks_NowVariants(t *testing.T) {
	for _, c := range revisionMarkCases() {
		t.Run(c.name, func(t *testing.T) {
			// Bracket the authoring so the recorded timestamp can be checked
			// against a real interval rather than merely "not zero".
			before := time.Now().UTC().Add(-2 * time.Second)
			doc := Create()
			p := doc.AddParagraph()
			p.AddRun().SetText("subject")
			c.author(p, revisionAuthor)
			after := time.Now().UTC().Add(2 * time.Second)

			data, err := doc.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			parts, _ := docParts(t, data)
			main := parts["word/document.xml"]
			for _, want := range c.wantXML {
				if !strings.Contains(main, want) {
					t.Errorf("word/document.xml lacks %s:\n%s", want, main)
				}
			}
			for _, not := range c.notXML {
				if strings.Contains(main, not) {
					t.Errorf("word/document.xml unexpectedly contains %s (delegated to the wrong twin?):\n%s", not, main)
				}
			}

			// The reopen is the assertion that matters: it proves the revision
			// is in the part in a form the reader recognizes, not just that the
			// right substring was emitted.
			revs := saveAndReopen(t, doc).Revisions()
			var got *Revision
			for _, r := range revs {
				if r.Type() == c.wantKind {
					got = r
					break
				}
			}
			if got == nil {
				kinds := make([]string, 0, len(revs))
				for _, r := range revs {
					kinds = append(kinds, string(r.Type()))
				}
				t.Fatalf("no %s revision after reopen; got %v", c.wantKind, kinds)
			}
			if got.Author() != revisionAuthor {
				t.Errorf("Author() = %q, want %q", got.Author(), revisionAuthor)
			}
			if got.Text() != c.wantText {
				t.Errorf("Text() = %q, want %q", got.Text(), c.wantText)
			}
			if got.MoveName() != c.moveName {
				t.Errorf("MoveName() = %q, want %q", got.MoveName(), c.moveName)
			}

			// The now-variants must stamp the current time, in UTC. A zero
			// time, a local-zone time, or a date forwarded from the wrong
			// argument all fail here.
			ds := got.Date()
			if ds == "" {
				t.Fatal("Date() is empty: the now-variant recorded no timestamp")
			}
			if !strings.HasSuffix(ds, "Z") {
				t.Errorf("Date() = %q, want a UTC (Z-suffixed) timestamp", ds)
			}
			when, err := time.Parse(time.RFC3339, ds)
			if err != nil {
				t.Fatalf("Date() = %q is not RFC3339: %v", ds, err)
			}
			if when.Before(before) || when.After(after) {
				t.Errorf("Date() = %v, want a time within [%v, %v] — the now-variant did not use the current time",
					when, before, after)
			}
		})
	}
}

// TestFormatRevisionDate_NormalizesToUTC drives the helper directly with a
// zoned time. The end-to-end tests above cannot see this: they call time.Now(),
// so on a machine whose local zone is UTC a missing .UTC() conversion is
// invisible. Word reads w:date as an absolute instant, so a timestamp written
// with a local wall clock and a Z suffix is simply the wrong time.
func TestFormatRevisionDate_NormalizesToUTC(t *testing.T) {
	plusFive := time.FixedZone("UTC+5", 5*60*60)
	minusThree := time.FixedZone("UTC-3", -3*60*60)
	const want = "2021-03-04T05:06:07Z"

	for _, in := range []time.Time{
		time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC),
		time.Date(2021, 3, 4, 10, 6, 7, 0, plusFive),
		time.Date(2021, 3, 4, 2, 6, 7, 0, minusThree),
	} {
		if got := formatRevisionDate(in); got != want {
			t.Errorf("formatRevisionDate(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestRevisionMarks_MatchTheirWithDateTwin pins the delegation itself: authoring
// with the now-variant and with the explicit-date variant must produce the same
// markup once the timestamp is held equal. It compares the whole main part with
// the two dates normalized away, so a delegation that swapped the author and
// name arguments, or dropped one, shows up as a diff.
func TestRevisionMarks_MatchTheirWithDateTwin(t *testing.T) {
	fixed := time.Date(2021, 3, 4, 5, 6, 7, 0, time.UTC)

	cases := []struct {
		name          string
		now, withDate func(p *Paragraph)
	}{
		{
			name:     "MarkInserted",
			now:      func(p *Paragraph) { p.Runs()[0].MarkInserted(revisionAuthor) },
			withDate: func(p *Paragraph) { p.Runs()[0].MarkInsertedWithDate(revisionAuthor, fixed) },
		},
		{
			name:     "MarkDeleted",
			now:      func(p *Paragraph) { p.Runs()[0].MarkDeleted(revisionAuthor) },
			withDate: func(p *Paragraph) { p.Runs()[0].MarkDeletedWithDate(revisionAuthor, fixed) },
		},
		{
			name:     "AddMoveFromRun",
			now:      func(p *Paragraph) { p.AddMoveFromRun(revisionAuthor, "mv", "relocated") },
			withDate: func(p *Paragraph) { p.AddMoveFromRunWithDate(revisionAuthor, "mv", "relocated", fixed) },
		},
		{
			name:     "AddMoveToRun",
			now:      func(p *Paragraph) { p.AddMoveToRun(revisionAuthor, "mv", "relocated") },
			withDate: func(p *Paragraph) { p.AddMoveToRunWithDate(revisionAuthor, "mv", "relocated", fixed) },
		},
	}

	build := func(t *testing.T, apply func(p *Paragraph)) string {
		t.Helper()
		doc := Create()
		p := doc.AddParagraph()
		p.AddRun().SetText("subject")
		apply(p)
		data, err := doc.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		parts, _ := docParts(t, data)
		return stripRevisionDates(parts["word/document.xml"])
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotNow := build(t, c.now)
			gotFixed := build(t, c.withDate)
			if gotNow != gotFixed {
				t.Errorf("the now-variant and the with-date variant do not produce the same markup:\n now:   %s\n fixed: %s",
					gotNow, gotFixed)
			}
			// Guard the normalizer: if it stopped removing dates, the two
			// would differ and the comparison above would be vacuous the other
			// way round.
			if strings.Contains(gotFixed, "2021-03-04") {
				t.Fatal("stripRevisionDates left a timestamp behind; the comparison above is not meaningful")
			}
		})
	}
}

// stripRevisionDates blanks every w:date attribute value so two documents
// authored at different instants can be compared.
func stripRevisionDates(part string) string {
	var b strings.Builder
	rest := part
	for {
		i := strings.Index(rest, ` w:date="`)
		if i < 0 {
			b.WriteString(rest)
			return b.String()
		}
		b.WriteString(rest[:i])
		b.WriteString(` w:date="*"`)
		rest = rest[i+len(` w:date="`):]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			return b.String()
		}
		rest = rest[j+1:]
	}
}

func TestStripRevisionDates(t *testing.T) {
	const in = `<w:ins w:id="1" w:author="A" w:date="2021-03-04T05:06:07Z"><w:r/></w:ins>`
	const want = `<w:ins w:id="1" w:author="A" w:date="*"><w:r/></w:ins>`
	if got := stripRevisionDates(in); got != want {
		t.Errorf("stripRevisionDates =\n %s\nwant\n %s", got, want)
	}
	if got := stripRevisionDates("<w:r/>"); got != "<w:r/>" {
		t.Errorf("stripRevisionDates on a date-free part changed it: %s", got)
	}
}

// TestMarkDeleted_ConvertsTextToDelText: accepting a deletion has to remove the
// text, and rejecting it has to restore it. That only works if MarkDeleted
// rewrote w:t as w:delText — a w:t left inside a w:del is invalid and Word
// silently drops the run.
func TestMarkDeleted_ConvertsTextToDelText(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddRun().SetText("doomed")
	p.AddRun().SetText(" kept")
	p.Runs()[0].MarkDeleted(revisionAuthor)

	reopened := saveAndReopen(t, doc)
	revs := reopened.Revisions()
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	if revs[0].Type() != RevisionDeletion {
		t.Fatalf("revision type = %q, want %q", revs[0].Type(), RevisionDeletion)
	}
	if err := revs[0].Accept(); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	after := saveAndReopen(t, reopened)
	if got := after.Paragraphs()[0].Text(); got != " kept" {
		t.Errorf("after accepting the deletion the paragraph reads %q, want %q", got, " kept")
	}
}

// TestMarkInserted_RejectRemovesTheRun is the mirror: an insertion authored
// with MarkInserted must be rejectable, which requires the run to be inside a
// w:ins the transformer recognizes.
func TestMarkInserted_RejectRemovesTheRun(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddRun().SetText("added")
	p.AddRun().SetText(" kept")
	p.Runs()[0].MarkInserted(revisionAuthor)

	reopened := saveAndReopen(t, doc)
	revs := reopened.Revisions()
	if len(revs) != 1 {
		t.Fatalf("got %d revisions, want 1", len(revs))
	}
	if err := revs[0].Reject(); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	after := saveAndReopen(t, reopened)
	if got := after.Paragraphs()[0].Text(); got != " kept" {
		t.Errorf("after rejecting the insertion the paragraph reads %q, want %q", got, " kept")
	}
}
