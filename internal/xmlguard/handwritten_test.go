// Package xmlguard_test holds the guard for XML that is written without going
// through the Builder.
//
// Every rule about names — must be a QName, must be in a namespace something
// binds, must not be pasted together out of a prefix and whatever the decoder
// reported — lives in common/xml and is enforced when a name is written through
// it. Code that builds a tag out of string pieces is subject to none of that, by
// construction: no test of the Builder can reach it, because it never calls the
// Builder.
//
// Two shipped defects came from exactly this in one week:
//
//   - docx.preserveNoteExtraChildren wrote "<w:" followed by the local name the
//     decoder reported. A source element written <:pos/> is reported with the
//     local name ":pos", so the output was <w::pos/> — two colons, not a QName,
//     and the emitted part no longer parsed.
//   - opc.rebuildScalarElement returned "<" + "vt:"+name.Local + ">" + …, which
//     moved a custom property value out of whatever namespace the source used
//     and into docPropsVTypes.
//
// Both were found by hand. This is the check that would have found them.
package xmlguard_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// allowed lists files that build XML by hand for a reason, with the reason. A
// new entry is a decision to write a name outside every rule that governs names,
// so it should be rare and it should be argued here.
//
// Prefer the site-level form to this one: a `//xmlguard:allow <reason>` comment
// on the line or immediately above it exempts one construct, where a file entry
// exempts everything anyone adds to that file afterwards.
var allowed = map[string]string{
	// Builds a synthetic wrapper element that is handed straight to a decoder
	// and never written to a part, so nothing it produces reaches a package.
	// The element name is a constant supplied by the caller, not from input.
	"docx/math.go": "synthesizes a parse-only wrapper; the name is a caller constant",
}

// suppression is the site-level exemption, which must carry a reason.
const suppression = "//xmlguard:allow"

// skipDirs are trees the rule does not govern: common/xml is the writer whose
// rules these are, and it necessarily assembles names from pieces.
var skipDirs = map[string]bool{
	"common/xml": true,
	"testdata":   true,
	".git":       true,
	"tools":      true,
}

// tagOpener matches a string literal that opens an element: "<", "</", or a
// literal that already carries a prefix such as "<w:" or "</vt:". Anything
// concatenated onto one of these is being used as an element name.
var tagOpener = regexp.MustCompile(`^</?([A-Za-z_][A-Za-z0-9_.\-]*:)?$`)

func TestNoXMLNamesBuiltFromStringPieces(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	var findings []string
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if d.IsDir() {
			if skipDirs[rel] || strings.HasPrefix(rel, ".") && rel != "." {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if _, ok := allowed[filepath.ToSlash(rel)]; ok {
			return nil
		}
		files++
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			return nil // not our business; the build catches it
		}
		findings = append(findings, inspectFile(fset, f, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// The scan must actually have scanned something: a broken walk that finds
	// nothing looks identical to a clean tree.
	if files < 50 {
		t.Fatalf("only %d source files scanned; the walk is broken, not the tree", files)
	}

	for _, f := range findings {
		t.Errorf("%s\n\tan XML name is assembled from string pieces here, so none of the "+
			"rules in common/xml apply to it. Write it through the Builder, copy the "+
			"source bytes verbatim, or add the file to `allowed` with a reason.", f)
	}
}

// inspectFile reports both shapes the defects took, skipping any construct
// carrying a site-level exemption and reporting each position once.
func inspectFile(fset *token.FileSet, f *ast.File, rel string) []string {
	exempt := suppressedLines(fset, f)
	seen := map[int]bool{}
	var out []string
	pos := func(n ast.Node) string {
		line := fset.Position(n.Pos()).Line
		if exempt[line] || seen[line] {
			return ""
		}
		seen[line] = true
		return rel + ":" + strconv.Itoa(line)
	}
	add := func(n ast.Node) {
		if p := pos(n); p != "" {
			out = append(out, p)
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		// Shape 1: a concatenation that contains both a "<" literal and a ">"
		// literal with something non-literal between them — i.e. a whole tag
		// built around a computed name. `"<" + trimFloat(x)` is not this: it
		// has no closing piece and is a label, not markup.
		if be, ok := n.(*ast.BinaryExpr); ok && be.Op == token.ADD {
			parts := flattenConcat(be)
			if len(parts) > 1 && concatBuildsATag(parts) {
				add(be)
			}
			return true
		}
		// Shape 2: WriteString("<w:") immediately followed by WriteString(expr),
		// which is the same thing spelled across two statements.
		if blk, ok := n.(*ast.BlockStmt); ok {
			for i := 0; i+1 < len(blk.List); i++ {
				lit, ok1 := writeStringArg(blk.List[i])
				next, ok2 := writeStringArg(blk.List[i+1])
				if !ok1 || !ok2 {
					continue
				}
				s, isLit := stringLit(lit)
				if !isLit || !tagOpener.MatchString(s) {
					continue
				}
				if _, nextIsLit := stringLit(next); !nextIsLit {
					add(blk.List[i])
				}
			}
		}
		return true
	})
	return out
}

// concatBuildsATag reports whether the operands look like an element tag built
// around a computed name: an opener literal, a closer literal, and at least one
// operand that is not a literal.
func concatBuildsATag(parts []ast.Expr) bool {
	var opener, closer, computed bool
	for _, p := range parts {
		s, isLit := stringLit(p)
		if !isLit {
			computed = true
			continue
		}
		if tagOpener.MatchString(s) {
			opener = true
		}
		if strings.Contains(s, ">") {
			closer = true
		}
	}
	return opener && closer && computed
}

// suppressedLines returns the lines an //xmlguard:allow comment exempts: the
// line the comment sits on, and the line after it. A bare marker with no reason
// exempts nothing, so the exemption cannot be used without saying why.
func suppressedLines(fset *token.FileSet, f *ast.File) map[int]bool {
	return markedLines(fset, f, suppression)
}

// markedLines returns the lines a `marker <reason>` comment exempts: every line
// of the comment group, plus the statement it sits above. A marker with no
// reason after it exempts nothing, so the check cannot be silenced silently.
func markedLines(fset *token.FileSet, f *ast.File, marker string) map[int]bool {
	out := map[int]bool{}
	for _, group := range f.Comments {
		marked, reasoned := false, false
		for _, c := range group.List {
			idx := strings.Index(c.Text, marker)
			if idx < 0 {
				continue
			}
			marked = true
			if strings.TrimSpace(c.Text[idx+len(marker):]) != "" {
				reasoned = true
			}
		}
		if !marked || !reasoned {
			continue // absent, or present with no reason, which exempts nothing
		}
		// Every line of the group, plus the statement it sits above. A reason
		// long enough to be worth reading usually wraps, so keying on the first
		// line would exempt the middle of the comment and not the code.
		first := fset.Position(group.Pos()).Line
		last := fset.Position(group.End()).Line
		for l := first; l <= last+1; l++ {
			out[l] = true
		}
	}
	return out
}

func flattenConcat(e ast.Expr) []ast.Expr {
	be, ok := e.(*ast.BinaryExpr)
	if !ok || be.Op != token.ADD {
		return []ast.Expr{e}
	}
	return append(flattenConcat(be.X), flattenConcat(be.Y)...)
}

// writeStringArg returns the sole argument of a `.WriteString(x)` call statement.
func writeStringArg(s ast.Stmt) (ast.Expr, bool) {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil, false
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "WriteString" {
		return nil, false
	}
	return call.Args[0], true
}

func stringLit(e ast.Expr) (string, bool) {
	bl, ok := e.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the test directory")
		}
		dir = parent
	}
}
