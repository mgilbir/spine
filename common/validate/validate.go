// Package validate provides a small structured-validation vocabulary shared by
// the docx, xlsx, and pptx packages. Each format exposes a Validate method that
// walks its in-memory model (without saving or re-parsing) and returns a Report
// of findings; the Save paths run Validate first and refuse to write when any
// error-severity finding is present, so a structurally corrupt package is never
// produced.
//
// Findings are structured rather than a single joined string: each carries a
// stable machine Code, a Severity (error or warning), the OPC Part it concerns,
// and a human Detail. This gives callers programmatic triage (filter by Code or
// Severity) while still rendering usefully when returned as a plain error.
package validate

import (
	"sort"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// Severity classifies a validation finding.
type Severity int

const (
	// SeverityError marks a finding that makes the package structurally
	// invalid. Save refuses to write when any error-severity finding is
	// present.
	SeverityError Severity = iota

	// SeverityWarning marks a spec-questionable but commonly Office-tolerated
	// condition. Warnings are reported but never block a Save.
	SeverityWarning
)

// String renders the severity as "error" or "warning".
func (s Severity) String() string {
	switch s {
	case SeverityWarning:
		return "warning"
	default:
		return "error"
	}
}

// Error is a single structured validation finding. It implements the error
// interface so an individual finding can be returned or wrapped directly.
type Error struct {
	// Severity is error or warning.
	Severity Severity
	// Code is a stable machine-readable identifier for the check that
	// produced this finding (e.g. "dangling-rel"). One check == one Code.
	Code string
	// Part is the OPC part name the finding concerns, or "" when it is
	// package-wide.
	Part string
	// Detail is a human-readable explanation.
	Detail string
}

// Error renders the finding as "code [severity] part: detail".
func (e Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Code)
	b.WriteString(" [")
	b.WriteString(e.Severity.String())
	b.WriteString("]")
	if e.Part != "" {
		b.WriteString(" ")
		b.WriteString(e.Part)
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

// Report is an ordered set of findings. It implements error so a Report can be
// funnelled straight through a Save error path; its Error method joins only the
// error-severity findings (warnings never make Save fail, so they do not belong
// in the returned error string).
type Report []Error

// HasErrors reports whether the Report contains any error-severity finding.
func (r Report) HasErrors() bool {
	for _, e := range r {
		if e.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Errors returns only the error-severity findings.
func (r Report) Errors() Report { return r.filter(SeverityError) }

// Warnings returns only the warning-severity findings.
func (r Report) Warnings() Report { return r.filter(SeverityWarning) }

func (r Report) filter(s Severity) Report {
	var out Report
	for _, e := range r {
		if e.Severity == s {
			out = append(out, e)
		}
	}
	return out
}

// Error joins the error-severity findings into one string. A Report with no
// error-severity findings still renders (listing any warnings) so it is never
// mistaken for a fatal condition when logged.
func (r Report) Error() string {
	var parts []string
	for _, e := range r {
		if e.Severity == SeverityError {
			parts = append(parts, e.Error())
		}
	}
	if len(parts) == 0 {
		for _, e := range r {
			parts = append(parts, e.Error())
		}
	}
	if len(parts) == 0 {
		return "validation passed"
	}
	if len(parts) == 1 {
		return "validation failed: " + parts[0]
	}
	return "validation failed (" + itoa(len(parts)) + " findings): " + strings.Join(parts, "; ")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// Collector accumulates findings during a Validate pass.
type Collector struct {
	findings []Error
}

// New returns an empty Collector.
func New() *Collector { return &Collector{} }

// Add records a finding.
func (c *Collector) Add(sev Severity, code, part, detail string) {
	c.findings = append(c.findings, Error{Severity: sev, Code: code, Part: part, Detail: detail})
}

// Errorf records an error-severity finding.
func (c *Collector) Errorf(code, part, detail string) {
	c.Add(SeverityError, code, part, detail)
}

// Warnf records a warning-severity finding.
func (c *Collector) Warnf(code, part, detail string) {
	c.Add(SeverityWarning, code, part, detail)
}

// Report returns the accumulated findings.
func (c *Collector) Report() Report { return Report(c.findings) }

// --- Shared OPC-level checks -------------------------------------------------
//
// These operate on data every format already holds after Open: the set of part
// names that will be written, the per-part relationship lists, and a content
// type lookup. They are intentionally conservative — each errs toward silence
// when a fact is uncertain — so that a real, Office-accepted package never
// trips them (soundness over completeness).

// Codes for the shared checks.
const (
	CodeRelTargetMissing   = "rel-target-missing"
	CodeContentTypeMissing = "content-type-missing"
	CodeDuplicatePart      = "duplicate-part-name"
	CodeDanglingRel        = "dangling-rel"
)

// CheckDuplicateParts reports part names that collide case-insensitively (OPC
// part names compare case-insensitively, so two such parts are one part with an
// ambiguous content stream).
func CheckDuplicateParts(c *Collector, parts []string) {
	seen := make(map[string]string, len(parts))
	reported := make(map[string]bool)
	for _, p := range parts {
		key := strings.ToLower(p)
		if first, ok := seen[key]; ok {
			if !reported[key] {
				c.Errorf(CodeDuplicatePart, p, "part name collides case-insensitively with "+first)
				reported[key] = true
			}
			continue
		}
		seen[key] = p
	}
}

// CheckContentTypes reports parts without a resolvable content type (neither a
// Default by extension nor an Override) — the class that produces
// "[trash]"/unreadable parts. It is warning severity, not error: OOXML packages
// in the wild (and this library's own round-trip preservation) legitimately
// carry parts with no declared content type, and the final content type may be
// assigned by the writer after this pass. The package pseudo-part
// [Content_Types].xml is exempt (it has no content type of its own).
func CheckContentTypes(c *Collector, parts []string, contentType func(partName string) string) {
	for _, p := range parts {
		if isContentTypesPart(p) {
			continue
		}
		if strings.TrimSpace(contentType(p)) == "" {
			c.Warnf(CodeContentTypeMissing, p, "part has no content type (no Default extension mapping or Override)")
		}
	}
}

func isContentTypesPart(name string) bool {
	return name == "/[Content_Types].xml" || name == "[Content_Types].xml"
}

// CheckRelationshipTargets reports declared internal relationships whose target
// part is absent from the package. External targets are skipped. exists must
// report whether a resolved part name is present; it should be over-inclusive
// (treating an uncertain name as present) so the check never yields a false
// positive.
//
// It is warning severity, not error: real Office-accepted packages routinely
// carry relationships whose internal target part is missing (broken diagram/VML
// rels, placeholder "NULL" targets, unused entries) and the consumer simply
// ignores them. The corruption direction that actually breaks a document — a
// part that REFERENCES an r:id with no declared relationship — is the
// dangling-rel check each format runs against its typed references.
func CheckRelationshipTargets(c *Collector, relsByPart map[string][]*opc.Relationship, exists func(partName string) bool) {
	// Deterministic order for stable output.
	sources := make([]string, 0, len(relsByPart))
	for src := range relsByPart {
		sources = append(sources, src)
	}
	sort.Strings(sources)
	for _, src := range sources {
		for _, rel := range relsByPart[src] {
			if rel == nil || rel.IsExternal() || rel.Target == "" {
				continue
			}
			resolved := opc.ResolvePartName(src, rel.Target)
			if resolved == "" || !exists(resolved) {
				c.Warnf(CodeRelTargetMissing, src,
					"relationship "+rel.ID+" targets missing part "+resolved+" ("+rel.Target+")")
			}
		}
	}
}

// RelByID reports whether a relationship with the given id is present in rels.
// Formats use it for dangling-reference checks: a typed r:id-style reference in
// a part must resolve to a declared relationship on that part.
func RelByID(rels []*opc.Relationship, id string) bool {
	for _, rel := range rels {
		if rel != nil && rel.ID == id {
			return true
		}
	}
	return false
}
