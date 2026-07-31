package symmetry_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// Cross-format entry-point guard (C572).
//
// The capability table in symmetry_test.go compares handle TYPES method for
// method (a docx.Comment against an xlsx.Comment). That shape only works for
// capabilities that have a per-format handle type. Everything the project has
// shipped since — charts (#104–#111), theme (#118/#122), page/print setup and
// protection (#112–#115) — is reached through methods on the format's own
// container types instead (Document/Sheet/Slide/Workbook/Presentation), so the
// original guard cannot see it, and those APIs drifted apart unobserved:
// AddChart takes different arguments in each format, Theme returns a different
// type in pptx than in docx/xlsx (C571), and protection exists in two formats
// out of three.
//
// This table records the shipped state of each such entry point, per format,
// including deliberate absence. It is a visibility guard, not a convergence
// mandate: a recorded divergence does not fail. What fails is
//
//   - a signature that changes without the table being updated,
//   - a capability appearing in a format the table says does not have it,
//   - and a role whose signatures have CONVERGED while the table still calls
//     it diverged (so the reconciliation gets recorded when it happens).
//
// Adding a cross-format capability therefore means adding a row here, which is
// the point: the next capability cannot ship dark.

// apiPoint is one format's entry point for a capability role. sig is the
// rendered signature excluding the receiver; the empty string means the method
// must NOT exist on that receiver (the capability is absent in that format).
type apiPoint struct {
	format string
	recv   reflect.Type
	method string
	sig    string
}

// absent marks a capability that a format does not offer at all.
func absent(format string, recv reflect.Type, method string) apiPoint {
	return apiPoint{format: format, recv: recv, method: method}
}

// crossFormatRole is one named operation of a capability across the formats.
type crossFormatRole struct {
	capability string
	role       string
	// note records why the formats look the way they do today.
	note string
	// diverged is true when the present signatures are NOT all identical and
	// that is the recorded state of the world.
	diverged bool
	points   []apiPoint
}

var crossFormatRoles = []crossFormatRole{
	{
		capability: "Chart",
		role:       "AddChart",
		note: "Each format anchors a chart its own way: docx inline at document/paragraph " +
			"level with an EMU extent, xlsx by cell anchor string, pptx by explicit EMU " +
			"position and size on a slide. That part of the divergence is real and stays. " +
			"What did NOT have to differ was the position of the shared argument: xlsx took " +
			"the chart last and docx/pptx took it first, so the one thing all three have in " +
			"common sat in a different place in each (C566). The chart is now first " +
			"everywhere and the placement arguments follow it.",
		diverged: true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "AddChart", "func(*chart.Chart, int64, int64) error"},
			{"docx", reflect.TypeOf(&docx.Paragraph{}), "AddChart", "func(*chart.Chart, int64, int64) error"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "AddChart", "func(*chart.Chart, string) error"},
			{"pptx", reflect.TypeOf(&pptx.Slide{}), "AddChart", "func(*chart.Chart, int64, int64, int64, int64) error"},
		},
	},
	{
		capability: "Chart",
		role:       "Charts",
		note:       "The read side did converge: every container returns []*chart.Chart.",
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "Charts", "func() []*chart.Chart"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "Charts", "func() []*chart.Chart"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "Charts", "func() []*chart.Chart"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "Charts", "func() []*chart.Chart"},
			{"pptx", reflect.TypeOf(&pptx.Slide{}), "Charts", "func() []*chart.Chart"},
		},
	},
	{
		capability: "Theme",
		role:       "Theme",
		note: "CONVERGED (C571). This was the largest same-name-different-type collision " +
			"in the public surface: docx and xlsx handed back the shared *dml.ThemeEditor " +
			"while pptx returned a package-private read-only *pptx.Theme, so theme code " +
			"written against a Word document did not even compile against a deck. The " +
			"blocker was C374 — regenerating a pptx theme part from the narrow model would " +
			"have deleted custClrLst and the extLst carrying thm15:themeFamily — and it is " +
			"fixed: dml.Theme models both (plus the extLst of five nested types) and " +
			"ThemeEditor.Marshal replays the source root attrs verbatim. All four accessors " +
			"now return *dml.ThemeEditor, and the divergence flag is off so a regression " +
			"back to a format-local type fails this test.",
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "Theme", "func() *dml.ThemeEditor"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "Theme", "func() *dml.ThemeEditor"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "Theme", "func() *dml.ThemeEditor"},
			{"pptx", reflect.TypeOf(&pptx.SlideMaster{}), "Theme", "func() *dml.ThemeEditor"},
		},
	},
	{
		capability: "Protection",
		role:       "Protection",
		note: "Protection is a two-format capability: docx protects the document, xlsx " +
			"protects a sheet and (separately) the workbook structure, each returning its " +
			"own handle type. pptx models no protection at all — the absence below is the " +
			"record of that, and a pptx protection API appearing without a table row fails.",
		diverged: true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "Protection", "func() *docx.DocumentProtection"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "Protection", "func() *xlsx.SheetProtection"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "Protection", "func() *xlsx.WorkbookProtection"},
			absent("pptx", reflect.TypeOf(&pptx.Presentation{}), "Protection"),
		},
	},
	{
		capability: "Protection",
		role:       "Protect",
		note:       "Same shape as Protection: per-format options struct, nothing shared, absent in pptx.",
		diverged:   true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "Protect", "func(docx.DocumentProtectionOptions)"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "Protect", "func(xlsx.SheetProtectionOptions) error"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "Protect", "func(xlsx.WorkbookProtectionOptions)"},
			absent("pptx", reflect.TypeOf(&pptx.Presentation{}), "Protect"),
		},
	},
	{
		capability: "Page and print setup",
		role:       "PageMargins",
		note: "docx carries margins on a section and returns them unconditionally; xlsx " +
			"carries them on a sheet and reports presence with a second result. pptx has " +
			"no page model (a slide's extent is the presentation's slide size).",
		diverged: true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Section{}), "Margins", "func() docx.PageMargins"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "PageMargins", "func() (xlsx.PageMargins, bool)"},
			absent("pptx", reflect.TypeOf(&pptx.Slide{}), "PageMargins"),
		},
	},
	{
		capability: "Page and print setup",
		role:       "PageSize",
		note: "Three different spellings of one idea: docx returns width/height floats " +
			"from the section, xlsx a whole PageSetup struct with a presence flag, pptx " +
			"separate SlideWidth/SlideHeight accessors on the presentation.",
		diverged: true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Section{}), "PageSize", "func() (float64, float64)"},
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "PageSetup", "func() (xlsx.PageSetup, bool)"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "SlideWidth", "func() int64"},
		},
	},
	{
		capability: "Page and print setup",
		role:       "PrintOptions",
		note:       "Print-time options are an xlsx-only concept; nothing in docx or pptx corresponds.",
		points: []apiPoint{
			{"xlsx", reflect.TypeOf(&xlsx.Sheet{}), "PrintOptions", "func() (xlsx.PrintOptions, bool)"},
			absent("docx", reflect.TypeOf(&docx.Section{}), "PrintOptions"),
			absent("pptx", reflect.TypeOf(&pptx.Slide{}), "PrintOptions"),
		},
	},

	// ---------------------------------------------------------------------
	// Rows added by the C386–C571 cross-format wave. Each records either a
	// convergence (no `diverged` flag, so a regression fails) or a divergence
	// that survives inspection, with the reason.
	// ---------------------------------------------------------------------

	{
		capability: "Encryption",
		role:       "encrypted open",
		note: "CONVERGED (C386). Encryption is format-generic — the same CFB wrapper carries " +
			"any OOXML zip — but only docx had a wrapper, while xlsx.Open and pptx.Open " +
			"pointed callers at opc, whose *opc.Reader no public API could turn into a " +
			"Workbook or Presentation. Every format now opens an encrypted package through " +
			"its ordinary Open/OpenReader with opc.WithPassword, so there is no separate " +
			"encrypted entry point to diverge. The reflection points below cover the save " +
			"half; the open half is exercised by TestEncryptedOpenIsReachableInEveryFormat.",
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "SaveEncrypted", "func(string, string) error"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "SaveEncrypted", "func(string, string) error"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "SaveEncrypted", "func(string, string) error"},
		},
	},
	{
		capability: "Encryption",
		role:       "SaveEncryptedTo",
		note:       "CONVERGED (C386): the in-memory counterpart, same shape in all three.",
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "SaveEncryptedTo", "func(io.Writer, string) error"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "SaveEncryptedTo", "func(io.Writer, string) error"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "SaveEncryptedTo", "func(io.Writer, string) error"},
		},
	},
	{
		capability: "Image alt text",
		role:       "SetAltText",
		note: "PARTIALLY CONVERGED (C442). The read side was harmonized long ago (all three " +
			"picture readers expose AltText). The write side was SetAltText / SetDescription " +
			"/ impossible; docx and pptx now agree on SetAltText (SetDescription survives as " +
			"a deprecated alias). xlsx cannot join: Sheet.AddImage returns only an error, so " +
			"there is no handle, and xlsx.Image is a read view of the saved drawing with no " +
			"owning sheet to write through. Its settable path is the ImageOptions.AltText " +
			"field, guarded by TestAltTextWriteIsReachableInEveryFormat. Giving xlsx a " +
			"mutable image handle is the convergence this row is waiting for.",
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.InlineImage{}), "SetAltText", "func(string)"},
			{"pptx", reflect.TypeOf(&pptx.Picture{}), "SetAltText", "func(string)"},
			absent("xlsx", reflect.TypeOf(&xlsx.Image{}), "SetAltText"),
		},
	},
	{
		capability: "Cross-file composition",
		role:       "Append",
		note: "DIVERGED, and legitimately: the three formats append different things. docx " +
			"appends one body into another (a document has exactly one), so Append needs no " +
			"noun. pptx and xlsx append a collection — slides, sheets — and say so. What was " +
			"NOT legitimate was xlsx having no whole-file merge at all, so 'merge workbook B " +
			"into A' meant looping CopySheetFrom by name; AppendSheetsFrom closes that gap " +
			"(C569). The guards are coherent across all three: nil sentinel plus an explicit " +
			"'cannot append into itself'.",
		diverged: true,
		points: []apiPoint{
			{"docx", reflect.TypeOf(&docx.Document{}), "Append", "func(*docx.Document) error"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "AppendSlidesFrom", "func(*pptx.Presentation) error"},
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "AppendSheetsFrom", "func(*xlsx.Workbook) ([]*xlsx.Sheet, error)"},
		},
	},
	{
		capability: "Removal",
		role:       "Remove<child>",
		note: "CONVERGED on the Remove* prefix (C565). xlsx's DeleteSheet was the lone Delete* " +
			"in a library whose every other removal is Remove* — including RemoveDefinedName " +
			"and RemoveCustomProperty in that same package. RemoveSheet is now the primary " +
			"spelling; DeleteSheet is a deprecated alias for it. The signatures agree " +
			"(index in, error out), which is why this row carries no divergence flag.",
		points: []apiPoint{
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "RemoveSheet", "func(int) error"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "RemoveSlide", "func(int) error"},
		},
	},
	{
		capability: "Lookup by name",
		role:       "<Thing>ByName",
		note: "CONVERGED on the (value, error) shape (C565). xlsx.SheetByName reported a miss " +
			"as ErrSheetNotFound while pptx's GetLayoutByName returned a bare nil, so an " +
			"xlsx user carrying their habits across nil-dereferenced on the first typo. " +
			"LayoutByName is the new spelling; GetLayoutByName is deprecated. docx has no " +
			"by-name collection to look up, so it is absent by construction — recorded here " +
			"rather than left as an unexplained hole. The row stays flagged because the " +
			"rendered signatures cannot match: a sheet lookup returns *xlsx.Sheet and a " +
			"layout lookup *pptx.SlideLayout. The shape — (T, error), miss reported as a " +
			"sentinel error — is what converged, and this table compares text.",
		diverged: true,
		points: []apiPoint{
			{"xlsx", reflect.TypeOf(&xlsx.Workbook{}), "SheetByName", "func(string) (*xlsx.Sheet, error)"},
			{"pptx", reflect.TypeOf(&pptx.Presentation{}), "LayoutByName", "func(string) (*pptx.SlideLayout, error)"},
			{"pptx", reflect.TypeOf(&pptx.SlideMaster{}), "LayoutByName", "func(string) (*pptx.SlideLayout, error)"},
			absent("docx", reflect.TypeOf(&docx.Document{}), "SectionByName"),
		},
	},
}

// TestCrossFormatEntryPoints asserts every recorded entry point still has the
// recorded signature, and that a format the table says lacks a capability still
// lacks it.
func TestCrossFormatEntryPoints(t *testing.T) {
	for _, role := range crossFormatRoles {
		for _, p := range role.points {
			m, ok := p.recv.MethodByName(p.method)
			label := fmt.Sprintf("%s/%s: %s.%s", role.capability, role.role, p.recv, p.method)
			if p.sig == "" {
				if ok {
					t.Errorf("%s now exists (%s), but the cross-format table records this "+
						"capability as absent in %s. Add it to the table — and while you are "+
						"there, reconcile its shape with the other formats.",
						label, renderSig(m.Type), p.format)
				}
				continue
			}
			if !ok {
				t.Errorf("%s is missing; the cross-format table records %s", label, p.sig)
				continue
			}
			if got := renderSig(m.Type); got != p.sig {
				t.Errorf("%s signature changed: got %s, table records %s. Update the table "+
					"(and check whether the other formats should follow).", label, got, p.sig)
			}
		}
	}
}

// TestCrossFormatDivergenceIsRecorded keeps the divergence bookkeeping honest in
// both directions: a role marked diverged whose signatures now agree must be
// re-marked (that is how a reconciliation gets recorded), and a role NOT marked
// diverged must actually be uniform.
func TestCrossFormatDivergenceIsRecorded(t *testing.T) {
	for _, role := range crossFormatRoles {
		sigs := map[string][]string{}
		for _, p := range role.points {
			if p.sig == "" {
				continue
			}
			sigs[p.sig] = append(sigs[p.sig], p.format+"."+p.method)
		}
		if len(sigs) == 0 {
			t.Errorf("%s/%s: every point is marked absent", role.capability, role.role)
			continue
		}
		uniform := len(sigs) == 1
		switch {
		case role.diverged && uniform:
			t.Errorf("%s/%s: the table calls this diverged, but every format now uses %s. "+
				"Drop `diverged: true` so the convergence is recorded and cannot silently "+
				"regress.", role.capability, role.role, firstKey(sigs))
		case !role.diverged && !uniform:
			t.Errorf("%s/%s: signatures differ across formats (%s) but the table does not "+
				"record a divergence. Either converge them or record the divergence with a "+
				"note explaining why the formats differ.",
				role.capability, role.role, describeSigs(sigs))
		}
		if role.diverged && role.note == "" {
			t.Errorf("%s/%s: a recorded divergence needs a note saying why", role.capability, role.role)
		}
	}
}

// renderSig renders a method's signature without its receiver, using the same
// spelling as the table (package-qualified short names).
func renderSig(ft reflect.Type) string {
	var b strings.Builder
	b.WriteString("func(")
	for i := 1; i < ft.NumIn(); i++ { // In(0) is the receiver
		if i > 1 {
			b.WriteString(", ")
		}
		b.WriteString(ft.In(i).String())
	}
	b.WriteString(")")
	switch ft.NumOut() {
	case 0:
	case 1:
		b.WriteString(" " + ft.Out(0).String())
	default:
		var outs []string
		for i := 0; i < ft.NumOut(); i++ {
			outs = append(outs, ft.Out(i).String())
		}
		b.WriteString(" (" + strings.Join(outs, ", ") + ")")
	}
	return b.String()
}

func firstKey(m map[string][]string) string {
	for k := range m {
		return k
	}
	return ""
}

func describeSigs(m map[string][]string) string {
	var parts []string
	for sig, who := range m {
		parts = append(parts, fmt.Sprintf("%s: %s", strings.Join(who, "/"), sig))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
