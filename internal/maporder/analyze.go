// Package maporder analyses this repository's serialization packages for map
// iteration whose order reaches the output.
//
// Go randomises map iteration order, so a loop that ranges a map and emits as
// it goes produces different bytes on every run. That breaks the byte-identity
// promise this library is built on, and it has happened twice for real:
//
//   - C497: docx.Charts() walked d.headers/d.footers in map order, so the
//     returned slice was ordered differently on each call even though the
//     godoc promised document order. FormFields and Revisions already used
//     sortedKeys for exactly this reason.
//   - C515: pptx embedMediaData/embedAudioPart/embedFontData deduplicated by
//     scanning p.otherParts in map order and returning the first byte-equal
//     part, so with two identical media parts stored under different names the
//     relationship target varied between runs.
//
// Both were found by accident. The established fix in this codebase is
// collect-then-sort:
//
//	names := make([]string, 0, len(m))
//	for name := range m {
//	    names = append(names, name)
//	}
//	sort.Strings(names)
//	for _, name := range names { ... }   // deterministic
//
// A syntactic matcher cannot do this job: `range x` looks the same whether x is
// a map, a slice or a channel, and a purely syntactic sweep of this repository
// produced twelve candidates that were all false positives. So the analysis is
// type-directed, over golang.org/x/tools/go/packages. See Load for why that
// rather than go/importer.
//
// # Classification
//
// Most map ranges are fine, so the analyser classifies rather than reporting
// everything. A map range is order-safe when the key and the value are only
// used in ways whose result does not depend on the order they arrive in:
//
//   - appended to a slice the enclosing function sorts before it is used,
//   - written into another map or set, or deleted from one,
//   - stored at a computed index of a slice or array (position, not sequence),
//   - folded into a commutative aggregate: a min/max guarded by a comparison,
//     a counter, a boolean flag,
//   - used only inside the iteration and discarded.
//
// It is order-dependent — and reported — when the key or the value:
//
//   - is written to a Builder, Writer, Buffer or Collector that was declared
//     outside the loop (kindEmit), or passed to a function that writes to one,
//   - is appended to a slice that nothing ever sorts (kindCollect): the C497
//     shape, where map order becomes the order of a returned slice,
//   - escapes the loop by being returned or assigned outward (kindEscape): the
//     C515 shape, where "the first entry that matches" is whichever one the
//     runtime happened to visit first.
//
// Locally defined closures are inlined, because the collect-then-sort helpers
// in this repository are frequently written as one (`add`, `mark`, `scan`), and
// a summary that stopped at the call would classify them all as opaque. Calls
// to declared functions are resolved through the call graph instead, across
// package boundaries as well as within one — a write hidden a package away in a
// helper that does not own its Builder is the same defect as one written inline.
package maporder

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

// Finding kinds.
const (
	KindEmit    = "emit"
	KindCollect = "collect"
	KindEscape  = "escape"
)

// Finding is one order-dependent use of a map iteration variable.
type Finding struct {
	Key    string // stable identity: "<relpath>:<func>:<range expr>:<kind>"
	Pos    string // file:line:col of the offending statement
	Loop   string // file:line:col of the range statement
	Func   string // enclosing function
	Kind   string
	Detail string
}

// Stats records how much the analyser actually looked at, so a run that
// degrades to seeing nothing cannot pass for a clean sweep.
type Stats struct {
	Packages  int
	Files     int
	Funcs     int
	Exprs     int
	Ranges    int
	MapRanges int
}

// Analysis is the result of one sweep.
type Analysis struct {
	Stats
	// Loops identifies every map range that was classified, as
	// "<relpath>:<func>:<ranged expression>". A guard can assert that a known
	// landmark is still in here, which a Findings-only result cannot show:
	// "nothing reported" and "nothing looked at" are the same picture.
	Loops    []string
	Findings []Finding
	// Prog is the parsed and type-checked source the analysis ran on, kept so a
	// caller can ask further questions of it without paying for a second
	// source-mode type check.
	Prog *Program
}

// sinkTypes are the types whose methods append to a shared output. Writing to
// one from inside a map range makes the output depend on iteration order.
var sinkTypes = map[string]string{
	"bytes.Buffer":       "bytes.Buffer",
	"strings.Builder":    "strings.Builder",
	"bufio.Writer":       "bufio.Writer",
	"archive/zip.Writer": "zip.Writer",
	"github.com/mgilbir/spine/common/xml.Builder": "xmlb.Builder",
	"github.com/mgilbir/spine/opc.Writer":         "opc.Writer",
	// A Collector accumulates findings in the order they are reported, and the
	// validation report is compared and printed in that order.
	"github.com/mgilbir/spine/common/validate.Collector": "validate.Collector",
}

// sortFuncs are the calls that make a collected slice's order deterministic.
var sortFuncs = map[string]bool{
	"sort.Slice": true, "sort.SliceStable": true, "sort.Strings": true,
	"sort.Ints": true, "sort.Float64s": true, "sort.Sort": true, "sort.Stable": true,
	"slices.Sort": true, "slices.SortFunc": true, "slices.SortStableFunc": true,
}

// Analyze loads the packages matching patterns (rooted at dir) and reports
// every order-dependent map range in them. Positions are reported relative to
// dir.
func Analyze(dir string, patterns []string) (*Analysis, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	prog, err := Load(root, patterns)
	if err != nil {
		return nil, err
	}
	a := &analyzer{
		prog:     prog,
		repoRoot: root,
		decls:    map[*types.Func]*funcSite{},
		closures: map[*types.Var]*ast.FuncLit{},
		emitMemo: map[*types.Func]bool{},
	}
	a.index()
	a.run()
	sort.Slice(a.findings, func(i, j int) bool { return a.findings[i].Key < a.findings[j].Key })
	sort.Strings(a.loops)
	return &Analysis{Stats: a.stats, Loops: a.loops, Findings: a.findings, Prog: prog}, nil
}

type funcSite struct {
	pkg  *Package
	decl *ast.FuncDecl
}

type analyzer struct {
	prog     *Program
	repoRoot string
	decls    map[*types.Func]*funcSite
	closures map[*types.Var]*ast.FuncLit
	emitMemo map[*types.Func]bool
	stats    Stats
	loops    []string
	findings []Finding
}

// index records every function declaration and every closure bound to a
// variable, so a call site can be resolved back to a body.
func (a *analyzer) index() {
	a.stats.Packages = len(a.prog.Packages)
	for _, p := range a.prog.Packages {
		a.stats.Files += len(p.Files)
		a.stats.Exprs += len(p.Info.Types)
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch s := n.(type) {
				case *ast.FuncDecl:
					a.stats.Funcs++
					if obj, ok := p.Info.Defs[s.Name].(*types.Func); ok && s.Body != nil {
						a.decls[obj] = &funcSite{pkg: p, decl: s}
					}
				case *ast.AssignStmt:
					if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
						return true
					}
					lit, ok := s.Rhs[0].(*ast.FuncLit)
					if !ok {
						return true
					}
					id, ok := s.Lhs[0].(*ast.Ident)
					if !ok {
						return true
					}
					obj := p.Info.Defs[id]
					if obj == nil {
						obj = p.Info.Uses[id]
					}
					if v, ok := obj.(*types.Var); ok {
						a.closures[v] = lit
					}
				}
				return true
			})
		}
	}
}

// run walks every function body looking for map ranges.
func (a *analyzer) run() {
	for _, p := range a.prog.Packages {
		for _, f := range p.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				rs, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				a.stats.Ranges++
				if !isMapRange(p, rs) {
					return true
				}
				a.stats.MapRanges++
				a.checkLoop(p, enclosingFunc(f, rs), rs)
				return true
			})
		}
	}
}

func isMapRange(p *Package, rs *ast.RangeStmt) bool {
	tv, ok := p.Info.Types[rs.X]
	if !ok || tv.Type == nil {
		return false
	}
	_, isMap := tv.Type.Underlying().(*types.Map)
	return isMap
}

// enclosingFunc finds the top-level declaration containing rs.
func enclosingFunc(f *ast.File, rs *ast.RangeStmt) *ast.FuncDecl {
	var found *ast.FuncDecl
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		if fd.Pos() <= rs.Pos() && rs.End() <= fd.End() {
			found = fd
			break
		}
	}
	return found
}

// posRange is a source span whose declarations count as loop-local.
type posRange struct{ lo, hi token.Pos }

// loopCtx is the state of one map range's classification.
type loopCtx struct {
	a        *analyzer
	pkg      *Package
	fn       *ast.FuncDecl
	rs       *ast.RangeStmt
	taint    map[types.Object]bool
	local    []posRange
	folds    map[types.Object]bool
	collects map[types.Object]token.Pos
	report   bool
	found    []Finding
	depth    int
}

func (a *analyzer) checkLoop(p *Package, fn *ast.FuncDecl, rs *ast.RangeStmt) {
	if fn == nil || rs.Body == nil {
		return
	}
	a.loops = append(a.loops, fmt.Sprintf("%s:%s:%s",
		a.rel(a.prog.Fset.Position(rs.Pos()).Filename), funcName(fn), exprString(a.prog.Fset, rs.X)))
	c := &loopCtx{
		a:     a,
		pkg:   p,
		fn:    fn,
		rs:    rs,
		taint: map[types.Object]bool{},
		folds: map[types.Object]bool{},
		local: []posRange{{rs.Body.Pos(), rs.Body.End()}},
	}
	c.taintIdent(rs.Key)
	c.taintIdent(rs.Value)

	collects := map[types.Object]token.Pos{}
	// Three passes reach a taint fixed point for the chains this code contains
	// (a local assigned from the key, then a second local from the first);
	// findings are kept only from the last pass so they are not duplicated.
	for pass := 0; pass < 3; pass++ {
		c.report = pass == 2
		c.found = nil
		clear(collects)
		c.collects = collects
		c.walk(p, rs.Body)
	}
	for obj, pos := range collects {
		if c.isSorted(obj) {
			continue
		}
		c.add(pos, KindCollect, fmt.Sprintf(
			"appends to %s, which nothing sorts: the slice ends up in map order",
			obj.Name()))
	}
	a.findings = append(a.findings, c.found...)
}

func (c *loopCtx) taintIdent(e ast.Expr) {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}
	if obj := c.objOf(id); obj != nil {
		c.taint[obj] = true
	}
}

func (c *loopCtx) objOf(id *ast.Ident) types.Object {
	if obj, ok := c.pkg.Info.Defs[id]; ok && obj != nil {
		return obj
	}
	return c.pkg.Info.Uses[id]
}

// isLocal reports whether obj was declared inside the loop (or inside a
// closure the loop calls), which makes writing to it harmless.
func (c *loopCtx) isLocal(obj types.Object) bool {
	if obj == nil {
		return true
	}
	for _, r := range c.local {
		if r.lo <= obj.Pos() && obj.Pos() < r.hi {
			return true
		}
	}
	return false
}

// tainted reports whether e reads the loop key or value, directly or through a
// local derived from one.
func (c *loopCtx) tainted(e ast.Expr) bool {
	if e == nil {
		return false
	}
	hit := false
	ast.Inspect(e, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if obj := c.objOf(id); obj != nil && c.taint[obj] {
			hit = true
		}
		return !hit
	})
	return hit
}

func (c *loopCtx) add(pos token.Pos, kind, detail string) {
	if !c.report {
		return
	}
	rel := c.a.rel(c.a.prog.Fset.Position(pos).Filename)
	fnName := funcName(c.fn)
	key := fmt.Sprintf("%s:%s:%s:%s", rel, fnName, exprString(c.a.prog.Fset, c.rs.X), kind)
	for _, f := range c.found {
		if f.Key == key {
			return
		}
	}
	c.found = append(c.found, Finding{
		Key:    key,
		Pos:    c.a.shortPos(pos),
		Loop:   c.a.shortPos(c.rs.Pos()),
		Func:   fnName,
		Kind:   kind,
		Detail: detail,
	})
}

// walk classifies every statement reachable from n.
func (c *loopCtx) walk(p *Package, n ast.Node) {
	prev := c.pkg
	c.pkg = p
	defer func() { c.pkg = prev }()

	ast.Inspect(n, func(node ast.Node) bool {
		switch s := node.(type) {
		case *ast.IfStmt:
			// A min/max fold: `if k < best { best = k }`. The comparison makes
			// the result independent of the order the candidates arrive in, so
			// assignments to the compared variable inside the branch are safe.
			c.noteFolds(s.Cond)
		case *ast.ReturnStmt:
			for _, r := range s.Results {
				if !c.tainted(r) || c.isBool(r) {
					continue
				}
				c.add(r.Pos(), KindEscape, fmt.Sprintf(
					"returns %s, so which entry wins depends on iteration order",
					exprString(c.a.prog.Fset, r)))
			}
		case *ast.AssignStmt:
			c.assign(s)
		case *ast.CallExpr:
			c.call(s)
		case *ast.FuncLit:
			// A literal defined but not called here is not this loop's effect;
			// the ones that are called are inlined from the call site.
			return false
		}
		return true
	})
}

// noteFolds marks variables compared against a tainted value, so a subsequent
// assignment to them reads as a min/max fold rather than as an escape.
func (c *loopCtx) noteFolds(cond ast.Expr) {
	ast.Inspect(cond, func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if !ok {
			return true
		}
		switch b.Op {
		case token.LSS, token.GTR, token.LEQ, token.GEQ:
		default:
			return true
		}
		for _, side := range []ast.Expr{b.X, b.Y} {
			if id, ok := side.(*ast.Ident); ok {
				if obj := c.objOf(id); obj != nil && !c.taint[obj] {
					c.folds[obj] = true
				}
			}
		}
		return true
	})
}

func (c *loopCtx) isBool(e ast.Expr) bool {
	tv, ok := c.pkg.Info.Types[e]
	if !ok || tv.Type == nil {
		return false
	}
	b, ok := tv.Type.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

func (c *loopCtx) assign(s *ast.AssignStmt) {
	for i, lhs := range s.Lhs {
		var rhs ast.Expr
		if len(s.Rhs) == len(s.Lhs) {
			rhs = s.Rhs[i]
		} else if len(s.Rhs) == 1 {
			rhs = s.Rhs[0]
		}
		// Taint propagation: a local that reads the key or value carries it.
		if rhs != nil && c.tainted(rhs) {
			c.taintIdent(lhs)
		}
		if idx, ok := lhs.(*ast.IndexExpr); ok {
			// m[k] = v is order-independent (a set or a map), and s[i] = v
			// stores by position rather than by sequence. Both are safe.
			_ = idx
			continue
		}
		if _, ok := lhs.(*ast.StarExpr); ok {
			continue
		}
		obj := c.rootObj(lhs)
		if c.isLocal(obj) || obj == nil {
			continue
		}
		// x = append(x, ...) collects; the order only matters if nothing sorts x.
		if call, ok := rhs.(*ast.CallExpr); ok && isBuiltin(c.pkg, call, "append") {
			if c.tainted(call) && sameObj(c.rootObj(call.Args[0]), obj) {
				if _, seen := c.collects[obj]; !seen {
					c.collects[obj] = lhs.Pos()
				}
				continue
			}
		}
		if rhs == nil || !c.tainted(rhs) {
			continue
		}
		if c.folds[obj] {
			continue
		}
		c.add(lhs.Pos(), KindEscape, fmt.Sprintf(
			"assigns %s from the loop variable, so its final value depends on iteration order",
			exprString(c.a.prog.Fset, lhs)))
	}
}

func (c *loopCtx) call(call *ast.CallExpr) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if c.sinkWrite(call, sel) {
			return
		}
	}
	// A closure defined in the enclosing function is inlined: its captured
	// variables belong to that function, so an append inside it is the same
	// collect as an append written in the loop body.
	if lit := c.closureFor(call.Fun); lit != nil && c.depth < 4 {
		c.inline(call, lit)
		return
	}
	obj := c.calleeObj(call)
	if obj == nil {
		return
	}
	site := c.a.decls[obj]
	if site == nil {
		return
	}
	if !c.taintedCall(call) {
		return
	}
	if c.a.emitsExternally(obj) {
		c.add(call.Pos(), KindEmit, fmt.Sprintf(
			"passes the loop variable to %s, which writes to an output it does not own",
			obj.Name()))
	}
}

// sinkWrite reports (and returns true) when the call writes to a Builder,
// Writer, Buffer or Collector that lives outside the loop.
func (c *loopCtx) sinkWrite(call *ast.CallExpr, sel *ast.SelectorExpr) bool {
	tv, ok := c.pkg.Info.Types[sel.X]
	if !ok || tv.Type == nil {
		return false
	}
	name, isSink := sinkName(tv.Type)
	if !isSink {
		return false
	}
	if c.isLocal(c.rootObj(sel.X)) {
		return false
	}
	if !isWriteMethod(c.pkg, sel) {
		return false
	}
	c.add(call.Pos(), KindEmit, fmt.Sprintf(
		"calls %s.%s on a %s declared outside the loop: the output lands in map order",
		exprString(c.a.prog.Fset, sel.X), sel.Sel.Name, name))
	return true
}

func (c *loopCtx) taintedCall(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if c.tainted(arg) {
			return true
		}
	}
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && c.tainted(sel.X) {
		return true
	}
	return false
}

// closureFor resolves a call of a variable holding a function literal.
func (c *loopCtx) closureFor(fun ast.Expr) *ast.FuncLit {
	id, ok := fun.(*ast.Ident)
	if !ok {
		return nil
	}
	v, ok := c.objOf(id).(*types.Var)
	if !ok {
		return nil
	}
	return c.a.closures[v]
}

// inline analyses a closure body as though it were written at the call site:
// arguments carry their taint into the parameters, and only the literal's own
// declarations count as local.
func (c *loopCtx) inline(call *ast.CallExpr, lit *ast.FuncLit) {
	c.depth++
	defer func() { c.depth-- }()
	c.local = append(c.local, posRange{lit.Pos(), lit.End()})
	defer func() { c.local = c.local[:len(c.local)-1] }()

	if lit.Type.Params != nil {
		i := 0
		for _, field := range lit.Type.Params.List {
			for _, nm := range field.Names {
				if i < len(call.Args) && c.tainted(call.Args[i]) {
					c.taintIdent(nm)
				}
				i++
			}
		}
	}
	c.walk(c.pkg, lit.Body)
}

func (c *loopCtx) calleeObj(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	default:
		return nil
	}
	fn, _ := c.pkg.Info.Uses[id].(*types.Func)
	return fn
}

// rootObj peels selectors, indexes and dereferences down to the base variable.
func (c *loopCtx) rootObj(e ast.Expr) types.Object {
	for {
		switch t := e.(type) {
		case *ast.Ident:
			return c.objOf(t)
		case *ast.SelectorExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.StarExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.CallExpr:
			// A conversion such as byName(x) keeps x as the root.
			if len(t.Args) == 1 {
				e = t.Args[0]
				continue
			}
			return nil
		default:
			return nil
		}
	}
}

// isSorted reports whether the enclosing function sorts obj.
func (c *loopCtx) isSorted(obj types.Object) bool {
	sorted := false
	ast.Inspect(c.fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || !sortFuncs[id.Name+"."+sel.Sel.Name] {
			return true
		}
		if sameObj(c.rootObj(call.Args[0]), obj) {
			sorted = true
		}
		return !sorted
	})
	return sorted
}

// emitsExternally reports whether fn writes to a sink it did not create — its
// receiver, a parameter, a field or a package-level variable. A function that
// builds its own Builder and returns the bytes does not qualify, which is what
// keeps every Marshal method in the repository from counting as an emit.
func (a *analyzer) emitsExternally(fn *types.Func) bool {
	if v, ok := a.emitMemo[fn]; ok {
		return v
	}
	site := a.decls[fn]
	if site == nil {
		a.emitMemo[fn] = false
		return false
	}
	a.emitMemo[fn] = false // cycle guard: recursion reports "no" until proven
	p := site.pkg
	body := site.decl.Body
	inFn := func(obj types.Object) bool {
		if obj == nil {
			return true
		}
		return body.Pos() <= obj.Pos() && obj.Pos() < body.End() ||
			site.decl.Type.Pos() <= obj.Pos() && obj.Pos() < body.Pos()
	}
	// Parameters and the receiver are not "inside" for this purpose: writing to
	// a *Builder handed in by the caller is exactly the external effect we look
	// for. Only variables declared in the body are local.
	inBody := func(obj types.Object) bool {
		return obj != nil && body.Pos() <= obj.Pos() && obj.Pos() < body.End()
	}
	_ = inFn

	root := func(e ast.Expr) types.Object {
		for {
			switch t := e.(type) {
			case *ast.Ident:
				if o, ok := p.Info.Defs[t]; ok && o != nil {
					return o
				}
				return p.Info.Uses[t]
			case *ast.SelectorExpr:
				e = t.X
			case *ast.IndexExpr:
				e = t.X
			case *ast.StarExpr:
				e = t.X
			case *ast.ParenExpr:
				e = t.X
			default:
				return nil
			}
		}
	}

	emits := false
	ast.Inspect(body, func(n ast.Node) bool {
		if emits {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if tv, ok := p.Info.Types[sel.X]; ok && tv.Type != nil {
				if _, isSink := sinkName(tv.Type); isSink && !inBody(root(sel.X)) && isWriteMethod(p, sel) {
					emits = true
					return false
				}
			}
		}
		// Recursing into a callee only counts when this function hands it
		// something it does not own: a non-local sink argument, or a non-local
		// receiver. Otherwise a locally created Builder would make every caller
		// of a marshal helper look like an emitter.
		var id *ast.Ident
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			id = fun
		case *ast.SelectorExpr:
			id = fun.Sel
			if tv, ok := p.Info.Types[fun.X]; ok && tv.Type != nil {
				if _, isSink := sinkName(tv.Type); isSink && inBody(root(fun.X)) {
					return true
				}
			}
		default:
			return true
		}
		callee, _ := p.Info.Uses[id].(*types.Func)
		if callee == nil || callee == fn {
			return true
		}
		passesForeignSink := false
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if tv, ok := p.Info.Types[sel.X]; ok && tv.Type != nil && !inBody(root(sel.X)) {
				passesForeignSink = true
			}
		}
		for _, arg := range call.Args {
			if tv, ok := p.Info.Types[arg]; ok && tv.Type != nil {
				if _, isSink := sinkName(tv.Type); isSink && !inBody(root(arg)) {
					passesForeignSink = true
				}
			}
		}
		if passesForeignSink && a.emitsExternally(callee) {
			emits = true
			return false
		}
		return true
	})
	a.emitMemo[fn] = emits
	return emits
}

// sinkName reports whether t is (a pointer to) an output sink.
func sinkName(t types.Type) (string, bool) {
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		obj := named.Obj()
		if obj.Pkg() != nil {
			if name, ok := sinkTypes[obj.Pkg().Path()+"."+obj.Name()]; ok {
				return name, true
			}
		}
	}
	// Anything with a Write([]byte) (int, error) method is an output sink too,
	// which covers io.Writer parameters, hash.Hash and *os.File without naming
	// them one by one.
	if hasWriteMethod(t) {
		return types.TypeString(t, func(p *types.Package) string { return p.Name() }), true
	}
	return "", false
}

func hasWriteMethod(t types.Type) bool {
	ms := types.NewMethodSet(types.NewPointer(t))
	if _, ok := t.Underlying().(*types.Interface); ok {
		ms = types.NewMethodSet(t)
	}
	for i := 0; i < ms.Len(); i++ {
		fn, ok := ms.At(i).Obj().(*types.Func)
		if !ok || fn.Name() != "Write" {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 2 {
			continue
		}
		if sl, ok := sig.Params().At(0).Type().(*types.Slice); ok {
			if b, ok := sl.Elem().(*types.Basic); ok && b.Kind() == types.Byte {
				return true
			}
		}
	}
	return false
}

// isWriteMethod distinguishes a method that appends output from one that only
// reads it back: writers return nothing, or return an error.
func isWriteMethod(p *Package, sel *ast.SelectorExpr) bool {
	tv, ok := p.Info.Types[sel]
	if !ok || tv.Type == nil {
		return false
	}
	sig, ok := tv.Type.(*types.Signature)
	if !ok {
		return false
	}
	if sig.Results().Len() == 0 {
		return true
	}
	for i := 0; i < sig.Results().Len(); i++ {
		if named, ok := sig.Results().At(i).Type().(*types.Named); ok {
			if named.Obj().Name() == "error" && named.Obj().Pkg() == nil {
				return true
			}
		}
	}
	return false
}

func isBuiltin(p *Package, call *ast.CallExpr, name string) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok || id.Name != name || len(call.Args) == 0 {
		return false
	}
	_, isBuiltinObj := p.Info.Uses[id].(*types.Builtin)
	return isBuiltinObj
}

func sameObj(a, b types.Object) bool { return a != nil && a == b }

func funcName(fd *ast.FuncDecl) string {
	if fd == nil {
		return "?"
	}
	if fd.Recv != nil && len(fd.Recv.List) == 1 {
		return recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return fd.Name.Name
}

func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}

func exprString(fset *token.FileSet, e ast.Expr) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, e); err != nil {
		return "?"
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}

func (a *analyzer) rel(path string) string {
	r, err := filepath.Rel(a.repoRoot, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(r)
}

func (a *analyzer) shortPos(pos token.Pos) string {
	p := a.prog.Fset.Position(pos)
	return fmt.Sprintf("%s:%d:%d", a.rel(p.Filename), p.Line, p.Column)
}
