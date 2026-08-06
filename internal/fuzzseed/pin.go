// Pinning the values a save generates fresh on every run.
//
// A fuzz fixture has to be byte-identical from one build to the next. The
// corpus accumulated against it is a set of mutations of specific bytes, and a
// crasher is a promise that replaying those bytes reproduces the failure — a
// fixture that moves turns every stored entry into a description of a package
// that no longer exists.
//
// Most of the drift is timestamps, which PinTimestamps handles at the source by
// assigning the core properties before the save. The rest is generated inside
// the writer with no API to reach it: comment GUIDs, comment dates, paragraph
// ids, and pptx's change ids are minted from crypto/rand and time.Now() as the
// part is written. xlsx's comments fixture dealt with that by replacing the
// three comment parts wholesale with fixed content, which works but hand-writes
// XML that has to stay consistent with what the writer produces.
//
// PinGenerated takes the other route: it rewrites the volatile *values* in the
// bytes the writer produced, leaving the structure exactly as written. Values
// are remapped consistently across the whole package, so a GUID that appears as
// an author id in one part and as an authorId reference in another still
// matches after pinning — which is the property hand-written parts are most
// likely to get wrong.
package fuzzseed

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
)

var (
	// An ISO-8601 instant, as core properties and comment dates carry it. The
	// trailing zone is optional and the fractional part is not always present:
	// core.xml writes 2026-08-06T10:27:53Z while a pptx modern comment writes
	// 2026-08-06T10:27:53.963. Requiring the Z left the latter drifting.
	pinTimestamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?`)
	// A brace-wrapped GUID, as pptx comment authors and comments carry it.
	pinGUID = regexp.MustCompile(`\{[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\}`)
	// A WordprocessingML paragraph id: eight hex digits in an attribute.
	pinParaID = regexp.MustCompile(`(paraId(?:Parent)?=")([0-9A-Fa-f]{8})(")`)
	// pptx's change id, a decimal on a slide monitor entry.
	pinChangeID = regexp.MustCompile(`(cId=")(\d+)(")`)
)

// PinGenerated rewrites every value a save generates fresh — timestamps, GUIDs,
// paragraph ids and change ids — to a fixed value, consistently across the
// package, and returns the rebuilt archive.
//
// Equal values map to equal replacements and distinct values to distinct ones,
// so cross-part references survive: a pptx comment's authorId still names an
// entry in authors.xml, and a commentsExtended paraId still names a paragraph
// in comments.xml.
func PinGenerated(pkg []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil, fmt.Errorf("pinning generated values: %w", err)
	}

	// One pass to collect, so a value is numbered by where it first appears in
	// the package rather than by which part happens to be rewritten first.
	bodies := make([][]byte, len(zr.File))
	names := make([]string, len(zr.File))
	guids := map[string]string{}
	paraIDs := map[string]string{}
	changeIDs := map[string]string{}
	for i, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("pinning generated values: %s: %w", f.Name, err)
		}
		body, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("pinning generated values: %s: %w", f.Name, err)
		}
		names[i], bodies[i] = f.Name, body
		for _, g := range pinGUID.FindAll(body, -1) {
			assign(guids, string(g))
		}
		for _, m := range pinParaID.FindAllSubmatch(body, -1) {
			assign(paraIDs, string(m[2]))
		}
		for _, m := range pinChangeID.FindAllSubmatch(body, -1) {
			assign(changeIDs, string(m[2]))
		}
	}

	out := make([][2]string, 0, len(bodies))
	for i, body := range bodies {
		// The replacement keeps each instant's own shape: a value written with a
		// zone keeps one and a value written without stays local, so the pinned
		// fixture still looks like something the writer would produce.
		body = pinTimestamp.ReplaceAllFunc(body, func(ts []byte) []byte {
			if bytes.HasSuffix(ts, []byte("Z")) || bytes.ContainsAny(ts[len(ts)-6:], "+-") {
				return []byte(FixtureModified.UTC().Format("2006-01-02T15:04:05Z"))
			}
			return []byte(FixtureModified.UTC().Format("2006-01-02T15:04:05"))
		})
		body = pinGUID.ReplaceAllFunc(body, func(g []byte) []byte {
			return []byte(fmt.Sprintf("{%08s-0000-0000-0000-000000000000}", guids[string(g)]))
		})
		body = pinParaID.ReplaceAllFunc(body, func(m []byte) []byte {
			p := pinParaID.FindSubmatch(m)
			return append(append(append([]byte{}, p[1]...),
				[]byte(fmt.Sprintf("%08s", paraIDs[string(p[2])]))...), p[3]...)
		})
		body = pinChangeID.ReplaceAllFunc(body, func(m []byte) []byte {
			p := pinChangeID.FindSubmatch(m)
			return append(append(append([]byte{}, p[1]...),
				[]byte(changeIDs[string(p[2])])...), p[3]...)
		})
		out = append(out, [2]string{names[i], string(body)})
	}
	pinned := BuildZip(out)
	if pinned == nil {
		return nil, fmt.Errorf("pinning generated values: rebuilding the archive produced nothing")
	}
	return pinned, nil
}

// assign gives value the next free replacement, keyed by first appearance.
// Sorting the existing replacements keeps the numbering a pure function of the
// insertion order rather than of Go's map iteration.
func assign(m map[string]string, value string) {
	if _, ok := m[value]; ok {
		return
	}
	m[value] = fmt.Sprintf("%d", len(m)+1)
}
