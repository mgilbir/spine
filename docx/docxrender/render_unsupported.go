package docxrender

import "github.com/mgilbir/spine/common/xml"

// Placeholders for the element kinds the foundation profile does not draw. In
// strict mode meeting one fails with render.ErrUnsupported; in best-effort mode
// the element is left out, with one warning per kind, and the rest of the page
// still draws.
//
// A translator that adds support for a kind removes its entry here and
// registers its own function (see render_flow.go).

const (
	nsMC   = xml.NSMarkupCompatibility
	nsMath = xml.NSMath
)

// wordLeftOutBlocks are body-level elements.
var wordLeftOutBlocks = []struct{ key, what string }{
	{"tbl", "tables"},
	{"commentRangeStart", "comments"},
	{"commentRangeEnd", "comments"},
	{"altChunk", "embedded content"},
	{"{" + nsMC + "}AlternateContent", "alternate content (drawings and text boxes)"},
	{"{" + nsMath + "}oMathPara", "equations"},
	{"{" + nsMath + "}oMath", "equations"},
	{"ins", "tracked block changes"},
	{"del", "tracked block changes"},
	{"moveFrom", "tracked block changes"},
	{"moveTo", "tracked block changes"},
}

// wordLeftOutInlines are paragraph-level elements.
var wordLeftOutInlines = []struct{ key, what string }{
	{"commentRangeStart", "comments"},
	{"commentRangeEnd", "comments"},
	{"dir", "bidirectional overrides"},
	{"bdo", "bidirectional overrides"},
	{"subDoc", "sub-documents"},
	{"contentPart", "embedded content"},
	{"{" + nsMC + "}AlternateContent", "alternate content (drawings and text boxes)"},
	{"{" + nsMath + "}oMathPara", "equations"},
	{"{" + nsMath + "}oMath", "equations"},
}

// wordLeftOutRuns are w:r children.
var wordLeftOutRuns = []struct{ key, what string }{
	{"drawing", "drawings and images"},
	{"pict", "drawings and images"},
	{"object", "embedded objects"},
	{"{" + nsMC + "}AlternateContent", "alternate content (drawings and text boxes)"},
	{"commentReference", "comments"},
	{"annotationRef", "comments"},
	{"sym", "symbol characters"},
	{"ptab", "absolute position tabs"},
	{"dayShort", "date placeholders"},
	{"dayLong", "date placeholders"},
	{"monthShort", "date placeholders"},
	{"monthLong", "date placeholders"},
	{"yearShort", "date placeholders"},
	{"yearLong", "date placeholders"},
	{"ruby", "ruby text"},
	{"contentPart", "embedded content"},
}

func init() {
	for _, e := range wordLeftOutBlocks {
		what := e.what
		wordRegisterBlock(e.key, func(f *wordFlow, _ *wordNode) error { return f.r.leaveOut(what) })
	}
	for _, e := range wordLeftOutInlines {
		what := e.what
		wordRegisterInline(e.key, func(p *wordPara, _ *wordNode) error { return p.r.leaveOut(what) })
	}
	for _, e := range wordLeftOutRuns {
		what := e.what
		wordRegisterRun(e.key, func(rn *wordRun, _ *wordNode) error {
			if !rn.p.f.visible() {
				return nil
			}
			return rn.p.r.leaveOut(what)
		})
	}
}
