package xml_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This guard enforces one rule across the whole module: no float destined for
// XML output is formatted with an exponent-capable verb.
//
// Go's 'g' switches to E-notation at magnitudes at or above 1e6 and below 1e-4,
// which ordinary document data reaches — a sparkline's manual axis bound, a
// pivot field's grouping interval, a chart axis scale. Office writes plain
// decimal in every one of those positions, so E-notation both marks the output
// as non-Office and, for a value that was *parsed* at that magnitude, re-emits
// it in a spelling its own source never used. That is byte drift, and it was
// found three separate times in three packages (C531, C556, C559) before the
// rule was written down. xmlb.FormatFloat is the single policy; see its doc.
//
// The check is syntactic on purpose. It needs no type information because both
// patterns it looks for are float-only by construction: FormatFloat's second
// argument is its verb, and %g/%e/%E apply to no other kind.

// floatFmtExempt lists call sites that format a float with an exponent-capable
// verb for a reason, each with that reason. An entry matching no finding fails
// the test, so the list cannot rot into a pile of stale excuses.
//
// Keys are "<path>:<func>" — deliberately not line numbers, which would rot on
// the first unrelated edit above them.
//
// It is currently empty, and that is the point: every float in the module
// reaches XML through xmlb.FormatFloat or a lexical type that replays its
// source spelling. A new entry here should be rare and argued.
var floatFmtExempt = map[string]string{}

// exponentVerbs are the strconv verbs that can produce E-notation.
var exponentVerbs = map[string]bool{"g": true, "G": true, "e": true, "E": true}

type floatFmtFinding struct {
	key  string
	pos  string
	what string
}

func TestNoExponentFloatFormatting(t *testing.T) {
	root := moduleRoot(t)

	var findings []floatFmtFinding
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		// Tests format floats for human-readable failure messages, which has
		// nothing to do with what gets written into a part.
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		findings = append(findings, scanFileForExponentFormatting(t, path, filepath.ToSlash(rel))...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// The guard is only meaningful if it looked at a real amount of code.
	// A broken walk that visits nothing would otherwise pass silently.
	if len(scannedFiles) < 200 {
		t.Fatalf("guard scanned only %d files; the walk is broken, not the code clean", len(scannedFiles))
	}

	used := make(map[string]bool)
	var live []floatFmtFinding
	for _, f := range findings {
		if _, ok := floatFmtExempt[f.key]; ok {
			used[f.key] = true
			continue
		}
		live = append(live, f)
	}

	sort.Slice(live, func(i, j int) bool { return live[i].pos < live[j].pos })
	for _, f := range live {
		t.Errorf("%s: %s\n\tUse xmlb.FormatFloat, or a lexical type if the source spelling must survive.\n\tIf this value never reaches XML, add %q to floatFmtExempt with the reason.",
			f.pos, f.what, f.key)
	}

	for key := range floatFmtExempt {
		if !used[key] {
			t.Errorf("floatFmtExempt has a stale entry %q: no finding matches it, so remove it", key)
		}
	}
}

// scannedFiles counts what the walk actually parsed, so an empty result can be
// distinguished from a walk that silently visited nothing.
var scannedFiles []string

func scanFileForExponentFormatting(t *testing.T, path, rel string) []floatFmtFinding {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		// Not every .go file in a repo has to parse (build-tagged stubs,
		// generated scratch); a parse failure is not this guard's business.
		return nil
	}
	scannedFiles = append(scannedFiles, rel)
	return findExponentFormatting(fset, file, rel)
}

// findExponentFormatting reports exponent-capable float formatting in one file.
// It is separated from the walk so the guard can be tested against source it
// constructs itself.
func findExponentFormatting(fset *token.FileSet, file *ast.File, rel string) []floatFmtFinding {
	var out []floatFmtFinding

	// enclosing tracks the function a node sits in, for a line-independent key.
	var stack []string
	enclosing := func() string {
		if len(stack) == 0 {
			return "<file>"
		}
		return stack[len(stack)-1]
	}

	report := func(pos token.Pos, what string) {
		out = append(out, floatFmtFinding{
			key:  rel + ":" + enclosing(),
			pos:  fmt.Sprintf("%s:%d", rel, fset.Position(pos).Line),
			what: what,
		})
	}

	// ast.Inspect enters its callback with nil on the way back up, which makes
	// maintaining a scope stack inside a single callback error-prone. Recursing
	// explicitly at FuncDecl keeps push and pop adjacent.
	var walk func(ast.Node)
	walk = func(root ast.Node) {
		ast.Inspect(root, func(n ast.Node) bool {
			switch node := n.(type) {
			case nil:
				return false
			case *ast.FuncDecl:
				name := node.Name.Name
				if node.Recv != nil && len(node.Recv.List) > 0 {
					name = receiverTypeName(node.Recv.List[0].Type) + "." + name
				}
				stack = append(stack, name)
				if node.Body != nil {
					walk(node.Body)
				}
				stack = stack[:len(stack)-1]
				return false
			case *ast.CallExpr:
				inspectCall(node, report)
			}
			return true
		})
	}
	walk(file)
	return out
}

// inspectCall reports the two shapes that can put E-notation in an XML part.
func inspectCall(node *ast.CallExpr, report func(token.Pos, string)) {
	sel, ok := node.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return
	}
	switch {
	case pkg.Name == "strconv" && sel.Sel.Name == "FormatFloat" && len(node.Args) >= 2:
		if v, ok := charLiteral(node.Args[1]); ok && exponentVerbs[v] {
			report(node.Pos(), fmt.Sprintf("strconv.FormatFloat with %q verb", v))
		}
	case pkg.Name == "fmt":
		if i, ok := formatArgIndex(sel.Sel.Name); ok && len(node.Args) > i {
			if lit, ok := stringLiteral(node.Args[i]); ok {
				if verb := exponentVerbIn(lit); verb != "" {
					report(node.Pos(), fmt.Sprintf("fmt.%s with %s verb", sel.Sel.Name, verb))
				}
			}
		}
	}
}

// formatArgIndex returns the position of the format string in a fmt function's
// argument list, and whether the function takes one at all.
func formatArgIndex(name string) (int, bool) {
	switch name {
	case "Sprintf", "Printf", "Errorf":
		return 0, true
	case "Fprintf":
		return 1, true
	}
	return 0, false
}

// exponentVerbIn reports the first exponent-capable verb in a format string,
// skipping "%%" so a literal percent is not mistaken for one.
func exponentVerbIn(format string) string {
	for i := 0; i < len(format)-1; i++ {
		if format[i] != '%' {
			continue
		}
		if format[i+1] == '%' {
			i++
			continue
		}
		// Step over flags, width and precision to reach the verb.
		j := i + 1
		for j < len(format) && strings.ContainsRune("+-# 0123456789.*", rune(format[j])) {
			j++
		}
		if j < len(format) && exponentVerbs[string(format[j])] {
			return "%" + string(format[j])
		}
		i = j
	}
	return ""
}

func charLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.CHAR {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}

func receiverTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverTypeName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverTypeName(t.X)
	case *ast.IndexListExpr:
		return receiverTypeName(t.X)
	}
	return "?"
}

func skipDir(name string) bool {
	switch name {
	case ".git", ".claude", "testdata", "vendor", "node_modules":
		return true
	}
	return false
}

func moduleRoot(t *testing.T) string {
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
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// TestFloatFormatGuardFires is the positive control: the guard must actually
// report the patterns it claims to catch. Without it a refactor that broke the
// detection would leave a test that passes on everything.
func TestFloatFormatGuardFires(t *testing.T) {
	const src = `package p

import (
	"fmt"
	"strconv"
)

func viaStrconv(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func viaSprintf(f float64) string { return fmt.Sprintf("%g", f) }

func viaWidth(f float64) string { return fmt.Sprintf("%8.3E", f) }

func clean(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func alsoClean() string { return fmt.Sprintf("100%% of %s", "it") }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "planted.go", src, 0)
	if err != nil {
		t.Fatalf("parsing planted source: %v", err)
	}
	got := findExponentFormatting(fset, file, "planted.go")

	want := map[string]bool{
		"planted.go:viaStrconv": true,
		"planted.go:viaSprintf": true,
		"planted.go:viaWidth":   true,
	}
	for _, f := range got {
		if !want[f.key] {
			t.Errorf("guard flagged %s, which is clean", f.key)
			continue
		}
		delete(want, f.key)
	}
	for key := range want {
		t.Errorf("guard missed the planted violation in %s", key)
	}
}

// TestExponentVerbScanner pins the format-string scanner directly, including
// the cases that make a naive strings.Contains wrong.
func TestExponentVerbScanner(t *testing.T) {
	cases := []struct {
		format string
		want   string
	}{
		{"%g", "%g"},
		{"%e", "%e"},
		{"%E", "%E"},
		{"%G", "%G"},
		{"%8.3g", "%g"},
		{"%-+12.4E", "%E"},
		{"value: %d of %g", "%g"},
		{"%f", ""},
		{"%d %s %v", ""},
		{"100%% done", ""},
		{"%%g is a literal", ""},
		{"%w: %s", ""},
		{"", ""},
		{"%", ""},
	}
	for _, tc := range cases {
		if got := exponentVerbIn(tc.format); got != tc.want {
			t.Errorf("exponentVerbIn(%q) = %q, want %q", tc.format, got, tc.want)
		}
	}
}
