package docx

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A docx package is written back part by part, and most parts are only written
// at all when the session flagged them: a header or footer round-trips as
// preserved raw bytes unless markHdrFtrModified named it, and styles.xml,
// numbering.xml, settings.xml, comments.xml, footnotes.xml, endnotes.xml,
// people.xml and bibliography/sources.xml are each gated on their own
// *Modified field. A mutator that edits one of those models without raising
// the flag therefore does not fail — it succeeds in memory and is silently
// discarded at save. That is tension T2 in the 2026-07-27 audit, the
// most-repeated bug class in this codebase: C266 found it on the property
// setters, C406 found it again on a dozen feature mutators after they were
// added, and this sweep found it a third time on AddMath, AddMathPara,
// ClearBorders and ClearShading.
//
// Fixing the known offenders does not stop the fourth round. The failure is
// silent by construction, so nothing but a guard notices when the *next*
// mutator forgets, and a hand-maintained table of today's mutators only moves
// the omission from the code to the table. This guard instead derives both
// halves from the source: which functions write into a flag-gated model, and
// which functions reach that model's flag. It needs no entry per mutator, so a
// mutator added tomorrow is covered the day it is written.
//
// The exemption list below is the guard's only manual input, and every entry
// carries the reason it is safe — the convention captureExemptAttrs
// established in pptx/internal/oxml.

// mutationDomain names one flag-gated model group.
type mutationDomain string

const (
	domainHdrFtr    mutationDomain = "header/footer part"
	domainStyles    mutationDomain = "styles.xml"
	domainNumbering mutationDomain = "numbering.xml"
	domainSettings  mutationDomain = "settings.xml"
	domainComments  mutationDomain = "comments.xml"
	domainCommentEx mutationDomain = "commentsExtended.xml"
	domainPeople    mutationDomain = "people.xml"
	domainFootnotes mutationDomain = "footnotes.xml"
	domainEndnotes  mutationDomain = "endnotes.xml"
	domainSources   mutationDomain = "bibliography/sources.xml"
	// domainMainPart is document.xml. Unlike the others it is not gated on a
	// flag for *persistence* — it is regenerated whenever the body was
	// materialized, which is why the handles that reach it used to be a
	// domain-level exemption here. It is gated on one for the document's
	// modification time: a mutator that edits the body without recording the
	// edit persists it and leaves dcterms:modified stale (see modified.go).
	domainMainPart mutationDomain = "main document part"
)

// fieldRef identifies a struct field by its declaring package-local type.
type fieldRef struct{ typ, field string }

// domainFields maps the struct fields that hold a flag-gated model. Reaching a
// model through one of these fields and writing into it is what the guard
// looks for; the fields themselves are handles, so assigning the field (an
// ensureX creating an empty model) is not a content edit and does not count.
var domainFields = map[fieldRef]mutationDomain{
	// Header/footer story handles. Paragraph and Run carry hfPart, so their
	// touch() resolves to the owning part; Hyperlink and InlineImage delegate
	// to the paragraph/run they hang off. A Paragraph in the main document
	// part makes touch() a no-op, so routing every write through it is free.
	{"Paragraph", "p"}:         domainHdrFtr,
	{"Run", "r"}:               domainHdrFtr,
	{"Hyperlink", "h"}:         domainHdrFtr,
	{"InlineImage", "drawing"}: domainHdrFtr,
	{"Header", "hdr"}:          domainHdrFtr,
	{"Footer", "ftr"}:          domainHdrFtr,
	{"hdrFtrPart", "hdr"}:      domainHdrFtr,
	{"hdrFtrPart", "ftr"}:      domainHdrFtr,

	// Handles onto one element *inside* a metadata part. Editing one of these
	// is editing the part, so the same flag applies — Style.modified() and
	// ListLevel.modified() are the funnels.
	{"Style", "s"}:                 domainStyles,
	{"ListLevel", "lvl"}:           domainNumbering,
	{"ListDefinition", "abstract"}: domainNumbering,
	{"ListDefinition", "levels"}:   domainNumbering,
	{"Comment", "c"}:               domainComments,
	{"Footnote", "note"}:           domainFootnotes,

	// Metadata parts, each gated on its own Document flag.
	{"Document", "styles"}:           domainStyles,
	{"Document", "numbering"}:        domainNumbering,
	{"Document", "settings"}:         domainSettings,
	{"Document", "comments"}:         domainComments,
	{"Document", "commentsExtended"}: domainCommentEx,
	{"Document", "people"}:           domainPeople,
	{"Document", "footnotes"}:        domainFootnotes,
	{"Document", "endnotes"}:         domainEndnotes,
	{"Document", "sources"}:          domainSources,

	// The main document part. Paragraph.p and Run.r reach it too, but they are
	// already listed above: their touch() raises the header/footer flag and
	// records the edit in one call, so one entry covers both domains.
	{"Document", "docModel"}:    domainMainPart,
	{"Table", "tbl"}:            domainMainPart,
	{"TableRow", "tr"}:          domainMainPart,
	{"TableCell", "tc"}:         domainMainPart,
	{"Section", "sectPr"}:       domainMainPart,
	{"ContentControl", "block"}: domainMainPart,
	{"ContentControl", "run"}:   domainMainPart,
}

// domainFlagSetters maps a domain to the Document method a mutator must call to
// make its edit survive the save.
//
// These used to be plain `d.stylesModified = true` assignments, and the guard
// looked for the assignment. They are methods now because raising the flag is
// no longer the whole job: the setter also records the edit for
// dcterms:modified (mutate.go, modified.go), and a mutator that assigned the
// field would regenerate the part correctly while leaving the document's
// modification time stale. Crediting the call and not the field is what keeps
// the next mutator from finding out the hard way.
var domainFlagSetters = map[mutationDomain]string{
	domainStyles:    "markStylesModified",
	domainNumbering: "markNumberingModified",
	domainSettings:  "markSettingsModified",
	domainComments:  "markCommentsModified",
	domainCommentEx: "markCommentsExtModified",
	domainPeople:    "markPeopleModified",
	domainFootnotes: "markFootnotesModified",
	domainEndnotes:  "markEndnotesModified",
	domainSources:   "markSourcesModified",
	domainMainPart:  "markEdited",
}

// hdrFtrFlagCalls are the calls that flag a header/footer part. markHdrFtrModified
// is the primitive; the rest are the funnels that reach it, and are only
// credited when called on a receiver that owns a header/footer story (Style
// has an ensurePPr of its own that flags styles.xml, not a header).
var hdrFtrFlagCalls = map[string]bool{
	"markHdrFtrModified": true,
	"touch":              true,
	"mut":                true,
	"mutParagraph":       true,
	"markModified":       true,
}

// domainFlagFuncs are the functions that raise a domain's flag: the per-part
// setters from domainFlagSetters, plus the two that are a flag rather than
// setting one — the header/footer flag is a set of part names, and
// Revision.markModified fires a per-part notifier closure.
var domainFlagFuncs = func() map[string]mutationDomain {
	m := map[string]mutationDomain{
		"Document.markHdrFtrModified": domainHdrFtr,
		"Revision.markModified":       domainHdrFtr,
	}
	for domain, setter := range domainFlagSetters {
		m["Document."+setter] = domain
	}
	return m
}()

// mutationFlagExempt lists the functions allowed to write into a flag-gated
// model without reaching its flag, each with the reason. Entries are
// "Type.Method" (or a bare function name), and an entry that stops matching a
// real function fails the guard, so a rename or a deletion cannot leave a stale
// excuse behind.
// It holds one entry, and that is the point: every write into a flag-gated
// model in this package reaches its flag. An entry here should be rare and
// argued.
//
// One exemption is still domain-level rather than function-level, and is
// deliberately absent from domainFields above: Document.Properties, custom
// properties, VBA, glossary, frameset and custom-XML edits are gated on flags
// of their own (customPropertiesModified is computed by comparing against an
// open-time snapshot) and are not reached through a model field, so there is no
// selector chain to track.
var mutationFlagExempt = map[string]string{
	// Materializes an empty w:sectPr (and w:body) for a caller that asked for
	// the default section, which is as often a read as an edit —
	// DefaultSection().PageSize() must not bump the document's modification
	// time. Every Section *mutator* records the edit itself, so the only
	// unrecorded change is the empty sectPr a malformed body gains, which the
	// save would have had to write in any case.
	"Document.DefaultSection": "materializes the body's sectPr on a read path; " +
		"the Section setters record the edit themselves",
}

// --- the guard ---------------------------------------------------------------

// TestMutationsFlagTheirPart fails when a function writes into a flag-gated
// model without reaching that model's modification flag — the T2 failure. It
// is derived from the source rather than from a list of mutators, so a newly
// added mutator that forgets to flag fails it on the day it is written.
func TestMutationsFlagTheirPart(t *testing.T) {
	pkg := loadMutationPackage(t)

	usedExempt := map[string]bool{}
	var problems []string
	for _, name := range pkg.sortedFuncs() {
		f := pkg.funcs[name]
		for _, d := range sortedDomains(f.writes) {
			if pkg.flags(name, d) {
				continue
			}
			if _, ok := mutationFlagExempt[name]; ok {
				usedExempt[name] = true
				continue
			}
			consequence := "the edit is applied in memory and dropped at save"
			if d == domainMainPart {
				// The main part is written from the model either way; what is
				// lost here is the record that the document changed.
				consequence = "the edit is written out but dcterms:modified is left stale"
			}
			problems = append(problems, fmt.Sprintf(
				"%s (%s) writes into the %s model but never reaches its modification flag: %s. %s",
				name, pkg.pos(name), d, consequence, remedyFor(d)))
		}
	}
	for _, p := range problems {
		t.Error(p)
	}

	// A stale exemption is an unaudited excuse: fail when one no longer names a
	// function that needs it.
	for name := range mutationFlagExempt {
		if _, ok := pkg.funcs[name]; !ok {
			t.Errorf("mutationFlagExempt names %q, which no longer exists — drop the entry", name)
			continue
		}
		if !usedExempt[name] {
			t.Errorf("mutationFlagExempt names %q, which no longer needs an exemption — drop the entry", name)
		}
	}
}

// TestMutationFlagGuardSeesWrites pins the guard's own detection. A guard that
// silently stopped classifying anything as a write would keep passing forever
// while covering nothing, so these landmark mutators — one per detection
// mechanism — must stay classified.
func TestMutationFlagGuardSeesWrites(t *testing.T) {
	pkg := loadMutationPackage(t)
	want := map[string]mutationDomain{
		// Direct field assignment through the receiver's model.
		"Paragraph.SetStyle": domainHdrFtr,
		"Run.SetColor":       domainHdrFtr,
		// A mutating internal/oxml method called on the model.
		"Paragraph.Clear":    domainHdrFtr,
		"Run.AddSymbol":      domainHdrFtr,
		"Document.Protect":   domainSettings,
		"Document.AddSource": domainSources,
		// A chain through a handle field (StyleManager.document.styles).
		"StyleManager.AddStyle": domainStyles,
		"ListLevel.SetStart":    domainNumbering,
		// A write through a local aliased to the model.
		"Document.addCommentModel": domainComments,
		// The main document part, reached through each of its handles.
		"Section.SetPageSize":     domainMainPart,
		"Table.SetStyle":          domainMainPart,
		"TableCell.SetShading":    domainMainPart,
		"ContentControl.SetValue": domainMainPart,
		"Document.AddParagraph":   domainMainPart,
	}
	for name, domain := range want {
		f, ok := pkg.funcs[name]
		if !ok {
			t.Errorf("%s no longer exists; update the landmark list", name)
			continue
		}
		if !f.writes[domain] {
			t.Errorf("%s is no longer detected as writing into the %s model — "+
				"the guard's write detection has regressed and it is now checking less than it reports",
				name, domain)
		}
	}
	if n := len(pkg.funcs); n < 200 {
		t.Errorf("parsed only %d docx functions; the guard is not seeing the package", n)
	}
}

// remedyFor names the call the mutator is missing.
func remedyFor(d mutationDomain) string {
	if d == domainHdrFtr {
		return "Reach the model through Paragraph.mut()/Run.mut() (see mutate.go) instead of the p/r field."
	}
	if setter, ok := domainFlagSetters[d]; ok {
		if d == domainMainPart {
			return "Call d." + setter + "() (or the handle's touch()) — see modified.go."
		}
		return "Call d." + setter + "() (see mutate.go)."
	}
	return ""
}

func sortedDomains(m map[mutationDomain]bool) []mutationDomain {
	out := make([]mutationDomain, 0, len(m))
	for d := range m {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// --- package model -----------------------------------------------------------

type mutationFunc struct {
	name     string
	recvType string
	pos      string
	writes   map[mutationDomain]bool // domains this function writes into
	setsFlag map[mutationDomain]bool // domains this function flags directly
	calls    map[string]bool         // resolved callee names
	loose    map[string]bool         // unresolved callee method names
	// resultType names the package-local type of a single-result function, so
	// `run := p.AddRun()` binds run to Run.
	resultType string
	// returnsDomain is set when the function hands back a flag-gated model —
	// ensureSettings, Style.ensureRPr, ListLevel.ensureInd. Without it the
	// `s := d.ensureSettings(); s.SetChild(…)` idiom would hide the write
	// behind a call the chain walk cannot see through.
	returnsDomain mutationDomain
}

type mutationPackage struct {
	fset    *token.FileSet
	funcs   map[string]*mutationFunc
	byName  map[string][]string // bare method name -> qualified names
	fields  map[string]map[string]string
	flagged map[string]map[mutationDomain]bool
	// modelReturns is the fixpoint of "this function hands back a flag-gated
	// model", consulted while resolving selector chains rooted at a call.
	modelReturns map[string]mutationDomain
}

func (p *mutationPackage) pos(name string) string { return p.funcs[name].pos }

func (p *mutationPackage) sortedFuncs() []string {
	out := make([]string, 0, len(p.funcs))
	for n := range p.funcs {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// flags reports whether name reaches domain d's flag, directly or through a
// call. The transitive closure is computed once, lazily.
func (p *mutationPackage) flags(name string, d mutationDomain) bool {
	if p.flagged == nil {
		p.computeFlagClosure()
	}
	return p.flagged[name][d]
}

func (p *mutationPackage) computeFlagClosure() {
	p.flagged = make(map[string]map[mutationDomain]bool, len(p.funcs))
	for name, f := range p.funcs {
		set := make(map[mutationDomain]bool, len(f.setsFlag))
		for d := range f.setsFlag {
			set[d] = true
		}
		p.flagged[name] = set
	}
	for changed := true; changed; {
		changed = false
		for name, f := range p.funcs {
			add := func(callee string) {
				for d := range p.flagged[callee] {
					if !p.flagged[name][d] {
						p.flagged[name][d] = true
						changed = true
					}
				}
			}
			for c := range f.calls {
				add(c)
			}
			// A call whose receiver type could not be resolved credits every
			// method of that name. This only ever grants a flag the guard would
			// otherwise demand, so it costs precision, not soundness.
			for m := range f.loose {
				for _, cand := range p.byName[m] {
					add(cand)
				}
			}
		}
	}
}

// loadMutationPackage parses the docx package's non-test sources and derives,
// for every function, the flag-gated models it writes into and the flags it
// reaches.
func loadMutationPackage(t *testing.T) *mutationPackage {
	t.Helper()
	files, fset := parseGoDir(t, ".")
	oxmlMutators := oxmlMutatingMethods(t)

	p := &mutationPackage{
		fset:         fset,
		funcs:        map[string]*mutationFunc{},
		byName:       map[string][]string{},
		fields:       collectStructFields(files),
		modelReturns: map[string]mutationDomain{},
	}
	var decls []*ast.FuncDecl
	for _, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			decls = append(decls, fd)
		}
	}
	// Analysis is run to a fixpoint: which functions hand back a flag-gated
	// model is discovered by the same walk that consumes it, so one pass would
	// miss `s := d.ensureSettings()` in every function analyzed before
	// ensureSettings itself.
	for pass := 0; ; pass++ {
		p.funcs = map[string]*mutationFunc{}
		p.byName = map[string][]string{}
		for _, fd := range decls {
			f := p.analyzeFunc(fd, oxmlMutators)
			p.funcs[f.name] = f
			p.byName[fd.Name.Name] = append(p.byName[fd.Name.Name], f.name)
		}
		grew := false
		for name, f := range p.funcs {
			if f.returnsDomain != "" && p.modelReturns[name] == "" {
				p.modelReturns[name] = f.returnsDomain
				grew = true
			}
		}
		if !grew {
			break
		}
		if pass > 20 {
			t.Fatal("model-return fixpoint did not converge")
		}
	}
	if len(p.funcs) == 0 {
		t.Fatal("docx package parsed to zero functions")
	}
	return p
}

// analyzeFunc walks one function body, resolving every selector chain against
// the package's struct fields to decide whether it reaches — and writes into —
// a flag-gated model.
func (p *mutationPackage) analyzeFunc(fd *ast.FuncDecl, oxmlMutators map[string]bool) *mutationFunc {
	recvType, recvName := "", ""
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		recvType = baseTypeName(fd.Recv.List[0].Type)
		if len(fd.Recv.List[0].Names) > 0 {
			recvName = fd.Recv.List[0].Names[0].Name
		}
	}
	name := fd.Name.Name
	if recvType != "" {
		name = recvType + "." + name
	}
	f := &mutationFunc{
		name: name, recvType: recvType,
		pos:      strings.TrimPrefix(p.fset.Position(fd.Pos()).String(), "./"),
		writes:   map[mutationDomain]bool{},
		setsFlag: map[mutationDomain]bool{},
		calls:    map[string]bool{},
		loose:    map[string]bool{},
	}
	if fd.Type.Results != nil && len(fd.Type.Results.List) == 1 && len(fd.Type.Results.List[0].Names) == 0 {
		f.resultType = baseTypeName(fd.Type.Results.List[0].Type)
	}
	if d, ok := domainFlagFuncs[name]; ok {
		f.setsFlag[d] = true
	}

	// Local variable environment: idents bound to a package-local type, and
	// idents bound to a model living in a flag-gated domain.
	localType := map[string]string{}
	localDomain := map[string]mutationDomain{}
	if recvName != "" && recvName != "_" {
		localType[recvName] = recvType
	}
	for _, param := range fd.Type.Params.List {
		if tn := baseTypeName(param.Type); tn != "" {
			for _, id := range param.Names {
				localType[id.Name] = tn
			}
		}
	}

	// resolve walks a selector chain, returning the domain of the model it
	// reaches and how many selectors follow the domain field.
	resolve := func(e ast.Expr) (mutationDomain, int, bool) {
		var path []string
		for {
			switch t := e.(type) {
			case *ast.SelectorExpr:
				path = append([]string{t.Sel.Name}, path...)
				e = t.X
			case *ast.IndexExpr:
				e = t.X
			case *ast.ParenExpr:
				e = t.X
			case *ast.StarExpr:
				e = t.X
			case *ast.CallExpr:
				// A chain rooted at a helper that hands back a flag-gated model
				// (`d.ensureSettings().SetChild(…)`).
				if d, ok := p.modelReturns[calleeName(t, localType, recvName, recvType)]; ok {
					return d, len(path), true
				}
				return "", 0, false
			case *ast.Ident:
				if d, ok := localDomain[t.Name]; ok {
					return d, len(path), true
				}
				typ := localType[t.Name]
				for i, seg := range path {
					if d, ok := domainFields[fieldRef{typ, seg}]; ok {
						return d, len(path) - i - 1, true
					}
					next, ok := p.fields[typ][seg]
					if !ok {
						return "", 0, false
					}
					typ = next
				}
				return "", 0, false
			default:
				return "", 0, false
			}
		}
	}

	// bindLocals records assignments that alias a package-local handle or a
	// flag-gated model into a local, so later writes through the local are seen.
	bindLocals := func(lhs, rhs []ast.Expr, define bool) {
		if len(lhs) != len(rhs) {
			return
		}
		for i, l := range lhs {
			id, ok := l.(*ast.Ident)
			if !ok || id.Name == "_" {
				continue
			}
			if d, _, ok := resolve(rhs[i]); ok {
				localDomain[id.Name] = d
				continue
			}
			if tn := exprTypeName(rhs[i], p, localType); tn != "" {
				localType[id.Name] = tn
			} else if define {
				delete(localType, id.Name)
				delete(localDomain, id.Name)
			}
		}
	}

	noteWrite := func(e ast.Expr, minDepth int) {
		if d, depth, ok := resolve(e); ok && depth >= minDepth {
			f.writes[d] = true
		}
	}

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for _, l := range s.Lhs {
				// depth >= 1: writing *into* the model. Assigning the handle
				// field itself (ensureX creating an empty model) is not an edit.
				noteWrite(l, 1)
			}
			bindLocals(s.Lhs, s.Rhs, s.Tok == token.DEFINE)
		case *ast.ValueSpec:
			var lhs []ast.Expr
			for _, id := range s.Names {
				lhs = append(lhs, id)
			}
			bindLocals(lhs, s.Values, true)
		case *ast.IncDecStmt:
			noteWrite(s.X, 1)
		case *ast.ReturnStmt:
			// Handing a flag-gated model back makes this function a funnel: the
			// caller's write through the result is a write into that model.
			if len(s.Results) == 1 {
				if d, _, ok := resolve(s.Results[0]); ok {
					f.returnsDomain = d
				}
			}
		case *ast.CallExpr:
			sel, ok := s.Fun.(*ast.SelectorExpr)
			if !ok {
				if id, ok := s.Fun.(*ast.Ident); ok {
					f.calls[id.Name] = true
				}
				return true
			}
			f.noteCall(p, sel, localType, recvName, recvType)
			// A mutating oxml method called on a flag-gated model edits it.
			if oxmlMutators[sel.Sel.Name] {
				noteWrite(sel.X, 0)
			}
		}
		return true
	})
	return f
}

// noteCall records the callee of a method call, resolving the receiver type
// where possible so that same-named methods on different types stay distinct.
func (f *mutationFunc) noteCall(p *mutationPackage, sel *ast.SelectorExpr, localType map[string]string, recvName, recvType string) {
	if id, ok := sel.X.(*ast.Ident); ok {
		if id.Name == recvName && recvType != "" {
			f.calls[recvType+"."+sel.Sel.Name] = true
			return
		}
		if tn, ok := localType[id.Name]; ok && tn != "" {
			f.calls[tn+"."+sel.Sel.Name] = true
			return
		}
	}
	// Unresolved receiver: credited against every method of that name, except
	// the header/footer funnels, which are only credited on a receiver type
	// that actually owns a header/footer story.
	f.loose[sel.Sel.Name] = true
	if hdrFtrFlagCalls[sel.Sel.Name] {
		f.setsFlag[domainHdrFtr] = true
	}
}

// calleeName names the function a call resolves to, or "" when the receiver's
// type is unknown. It mirrors noteCall's resolution so the two agree.
func calleeName(call *ast.CallExpr, localType map[string]string, recvName, recvType string) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		id, ok := fn.X.(*ast.Ident)
		if !ok {
			return ""
		}
		if id.Name == recvName && recvType != "" {
			return recvType + "." + fn.Sel.Name
		}
		if tn, ok := localType[id.Name]; ok && tn != "" {
			return tn + "." + fn.Sel.Name
		}
	}
	return ""
}

// --- helpers -----------------------------------------------------------------

// collectStructFields maps each package-local struct type to its fields whose
// type is itself a package-local named type, so selector chains through handle
// types (StyleManager.document, Run.paragraph, …) can be walked.
func collectStructFields(files []*ast.File) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				fields := map[string]string{}
				for _, fld := range st.Fields.List {
					tn := baseTypeName(fld.Type)
					for _, nm := range fld.Names {
						fields[nm.Name] = tn
					}
				}
				out[ts.Name.Name] = fields
			}
		}
	}
	return out
}

// exprTypeName names the package-local type an expression evaluates to, for
// the handful of forms that matter: &T{…}, a call to a package function or
// method whose single result is a package-local type.
func exprTypeName(e ast.Expr, p *mutationPackage, localType map[string]string) string {
	switch t := e.(type) {
	case *ast.UnaryExpr:
		if t.Op == token.AND {
			return exprTypeName(t.X, p, localType)
		}
	case *ast.CompositeLit:
		return baseTypeName(t.Type)
	case *ast.CallExpr:
		sel, ok := t.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		var candidates []string
		if id, ok := sel.X.(*ast.Ident); ok {
			if tn, ok := localType[id.Name]; ok && tn != "" {
				candidates = []string{tn + "." + sel.Sel.Name}
			}
		}
		if candidates == nil {
			candidates = p.byName[sel.Sel.Name]
		}
		if len(candidates) != 1 {
			return ""
		}
		return p.resultTypeName(candidates[0])
	}
	return ""
}

// resultTypeName is the package-local type of a function's single result.
func (p *mutationPackage) resultTypeName(name string) string {
	f, ok := p.funcs[name]
	if !ok || f.resultType == "" {
		return ""
	}
	return f.resultType
}

// baseTypeName strips pointers/slices and returns the identifier of a type
// expression, or "" when the type is not a bare package-local name.
func baseTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return baseTypeName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// parseGoDir parses every non-test .go file in dir.
func parseGoDir(t *testing.T, dir string) ([]*ast.File, *token.FileSet) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files = append(files, f)
	}
	return files, fset
}

// oxmlMutatingMethods returns the names of the internal/oxml methods that
// mutate their receiver, computed from that package's source: a call to one of
// them on a flag-gated model is a write. Deriving the set keeps the guard
// correct as the model grows new Append*/Insert*/Remove* helpers.
func oxmlMutatingMethods(t *testing.T) map[string]bool {
	t.Helper()
	files, _ := parseGoDir(t, filepath.Join("internal", "oxml"))

	// backfillChildOrder materializes a lazy index of the children a paragraph
	// already has. It writes to the receiver but changes no content, and the
	// read accessors that resolve a run's position call it — treating it as a
	// mutation would classify HasDirectChildRun and DirectChildRunIndex as
	// writes and make every range reader look like an unflagged mutator.
	derivedState := map[string]bool{"backfillChildOrder": true}

	type m struct {
		recv    string
		mutates bool
		calls   map[string]bool
	}
	methods := map[string]*m{}
	names := map[string][]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Recv == nil || len(fd.Recv.List) == 0 {
				continue
			}
			// A value receiver cannot mutate the model the caller holds.
			if _, ptr := fd.Recv.List[0].Type.(*ast.StarExpr); !ptr {
				continue
			}
			if derivedState[fd.Name.Name] {
				continue
			}
			recvName := ""
			if len(fd.Recv.List[0].Names) > 0 {
				recvName = fd.Recv.List[0].Names[0].Name
			}
			key := baseTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
			e := &m{recv: recvName, calls: map[string]bool{}}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.AssignStmt:
					for _, l := range s.Lhs {
						if rootIdent(l) == recvName && recvName != "" && l != nil {
							if _, isIdent := l.(*ast.Ident); !isIdent {
								e.mutates = true
							}
						}
					}
				case *ast.IncDecStmt:
					if rootIdent(s.X) == recvName && recvName != "" {
						e.mutates = true
					}
				case *ast.CallExpr:
					if sel, ok := s.Fun.(*ast.SelectorExpr); ok && rootIdent(sel.X) == recvName {
						e.calls[sel.Sel.Name] = true
					}
				}
				return true
			})
			methods[key] = e
			names[fd.Name.Name] = append(names[fd.Name.Name], key)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, e := range methods {
			if e.mutates {
				continue
			}
			for c := range e.calls {
				for _, cand := range names[c] {
					if methods[cand].mutates {
						e.mutates = true
						changed = true
					}
				}
			}
		}
	}
	out := map[string]bool{}
	for key, e := range methods {
		if e.mutates {
			out[key[strings.Index(key, ".")+1:]] = true
		}
	}
	if len(out) == 0 {
		t.Fatal("internal/oxml yielded no mutating methods")
	}
	return out
}

// rootIdent returns the identifier a selector/index chain is rooted at.
func rootIdent(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.SelectorExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
		}
	}
}
