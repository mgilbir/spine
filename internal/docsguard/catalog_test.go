// Package docsguard_test holds guards that pin prose in the repository's
// documentation to the code it describes.
//
// It exists because of C446: README.md and docs/troubleshooting.md both
// described a dangling `numPr` as an error-severity finding that blocks saves,
// while docx/validate.go had deliberately made it a warning (Word opens such
// documents; blocking the save would reject a file Word accepts). The severity
// was not wrong in one place and right in the other — the catalog was simply
// written twice, by hand, and the severities are actively tuned as wild files
// are surveyed. That guarantees recurrence.
//
// So the README table is now the single source of truth for the severities,
// and this test derives the real catalog from the validators and compares.
// Adding a check, or retuning one from error to warning, fails here until the
// table is updated in the same commit.
package docsguard_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// repoRoot is this package's directory, two levels below the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

// severities is the set of severities a finding code is emitted with.
type severities struct {
	err, warn bool
	sites     []string
}

func (s severities) String() string {
	switch {
	case s.err && s.warn:
		return "error and warning"
	case s.err:
		return "error"
	case s.warn:
		return "warning"
	}
	return "none"
}

// codeShape is what a validation code looks like: lowercase, dash-separated.
// It is also what distinguishes a Collector call from fmt.Errorf, whose first
// argument is a format string literal rather than a code constant.
var codeShape = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// parseGoDir parses every non-test .go file in dir. (parser.ParseDir is
// deprecated and x/tools is not a dependency of this zero-dependency module, so
// the glob is done by hand.)
func parseGoDir(t *testing.T, dir string) (*token.FileSet, []*ast.File) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", p, err)
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		t.Fatalf("no Go files found in %s", dir)
	}
	return fset, files
}

// stringConsts returns the string-valued constants declared in a directory's
// (non-test) Go files, keyed by identifier.
func stringConsts(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	_, files := parseGoDir(t, dir)
	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, n := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					bl, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || bl.Kind != token.STRING {
						continue
					}
					if v, err := strconv.Unquote(bl.Value); err == nil {
						out[n.Name] = v
					}
				}
			}
		}
	}
	return out
}

// collectCatalog walks the validator sources and returns every finding code
// with the severities it is actually emitted at.
//
// A Collector call is recognized structurally: a selector call to Errorf or
// Warnf with exactly three arguments whose first is an identifier or a
// qualified identifier resolving to a code-shaped string constant. fmt.Errorf
// never matches (its first argument is a format-string literal).
func collectCatalog(t *testing.T, root string) map[string]*severities {
	t.Helper()

	shared := stringConsts(t, filepath.Join(root, "common", "validate"))
	catalog := map[string]*severities{}

	dirs := []string{"docx", "xlsx", "pptx", filepath.Join("common", "validate")}
	for _, rel := range dirs {
		dir := filepath.Join(root, rel)
		local := stringConsts(t, dir)
		fset, files := parseGoDir(t, dir)
		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok || len(ce.Args) != 3 {
					return true
				}
				sel, ok := ce.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				var isErr bool
				switch sel.Sel.Name {
				case "Errorf":
					isErr = true
				case "Warnf":
					isErr = false
				default:
					return true
				}
				var code string
				switch a := ce.Args[0].(type) {
				case *ast.Ident:
					code = local[a.Name]
				case *ast.SelectorExpr:
					code = shared[a.Sel.Name]
				default:
					return true
				}
				if !codeShape.MatchString(code) {
					return true
				}
				s := catalog[code]
				if s == nil {
					s = &severities{}
					catalog[code] = s
				}
				if isErr {
					s.err = true
				} else {
					s.warn = true
				}
				pos := fset.Position(ce.Pos())
				s.sites = append(s.sites, rel+"/"+filepath.Base(pos.Filename)+":"+strconv.Itoa(pos.Line))
				return true
			})
		}
	}
	if len(catalog) == 0 {
		t.Fatal("found no validation findings at all; the extraction is broken, not the docs")
	}
	return catalog
}

var (
	catalogBegin = "<!-- validation-catalog:begin -->"
	catalogEnd   = "<!-- validation-catalog:end -->"
	rowRE        = regexp.MustCompile("^\\| `([a-z0-9-]+)` \\| ([^|]+?) \\|")
)

// readDocumentedCatalog parses the README's marked validation table.
func readDocumentedCatalog(t *testing.T, root string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	i := strings.Index(text, catalogBegin)
	j := strings.Index(text, catalogEnd)
	if i < 0 || j < 0 || j < i {
		t.Fatalf("README.md has no %s ... %s block; the validation catalog must stay machine-checkable",
			catalogBegin, catalogEnd)
	}
	out := map[string]string{}
	for _, line := range strings.Split(text[i:j], "\n") {
		m := rowRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		if _, dup := out[m[1]]; dup {
			t.Errorf("README validation catalog lists %q twice", m[1])
		}
		out[m[1]] = strings.TrimSpace(m[2])
	}
	return out
}

// TestValidationCatalogMatchesCode is the C446 guard: every finding the
// validators emit must appear in the README catalog with the severity it is
// really emitted at, and the catalog may not invent findings.
func TestValidationCatalogMatchesCode(t *testing.T) {
	root := repoRoot(t)
	actual := collectCatalog(t, root)
	documented := readDocumentedCatalog(t, root)

	var codes []string
	for code := range actual {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	for _, code := range codes {
		got := actual[code]
		want, ok := documented[code]
		if !ok {
			t.Errorf("validation finding %q (%s, emitted at %s) is not in the README catalog. "+
				"An undocumented check is one nobody can triage; add a row.",
				code, got, strings.Join(got.sites, ", "))
			continue
		}
		if want != got.String() {
			t.Errorf("validation finding %q: README says %q, the code emits it as %q (at %s). "+
				"The code is authoritative — fix the README row, or the severity, but do not "+
				"let them disagree.", code, want, got.String(), strings.Join(got.sites, ", "))
		}
	}

	for code := range documented {
		if _, ok := actual[code]; !ok {
			t.Errorf("README catalog documents validation finding %q, which no validator emits "+
				"(stale row, or the check was removed).", code)
		}
	}
}

// TestTroubleshootingDoesNotContradictSeverities keeps the second copy of the
// story honest. docs/troubleshooting.md explains the save-refusal gate in
// prose; it must not name a warning-severity finding as something that blocks a
// save. numbering-missing is exactly the case that went wrong (C446).
func TestTroubleshootingDoesNotContradictSeverities(t *testing.T) {
	root := repoRoot(t)
	actual := collectCatalog(t, root)

	data, err := os.ReadFile(filepath.Join(root, "docs", "troubleshooting.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The section that describes why a save was refused: from its heading to
	// the next one.
	text := string(data)
	start := strings.Index(text, "## Save refused with a validation error")
	if start < 0 {
		t.Fatal("docs/troubleshooting.md no longer has the save-refusal section")
	}
	section := text[start:]
	if next := strings.Index(section[3:], "\n## "); next >= 0 {
		section = section[:next+3]
	}

	// Phrases the guide uses for particular findings, mapped to the finding.
	// A phrase for a warning-only finding may appear here only in a sentence
	// that says so; a phrase for an error-severity finding may not be hedged
	// into a warning.
	phrases := map[string]string{
		"numPr":              "numbering-missing",
		"duplicate shape id": "shape-id-dup",
		"overlapping merged": "merge-overlap",
		"duplicate `sheetId": "sheet-id-dup",
	}
	for phrase, code := range phrases {
		s, ok := actual[code]
		if !ok {
			if strings.Contains(section, phrase) {
				t.Errorf("troubleshooting names %q but no validator emits %q", phrase, code)
			}
			continue
		}
		for _, sentence := range sentences(section) {
			if !strings.Contains(sentence, phrase) {
				continue
			}
			mentionsWarning := strings.Contains(strings.ToLower(sentence), "warning")
			switch {
			case !s.err && !mentionsWarning:
				t.Errorf("docs/troubleshooting.md's save-refusal section cites %q as a "+
					"save-refusing condition, but %q is warning-only (%s) and never blocks a "+
					"save. Say so in the same sentence, or drop the example.\n\tsentence: %s",
					phrase, code, strings.Join(s.sites, ", "), strings.TrimSpace(sentence))
			case s.err && !s.warn && mentionsWarning:
				t.Errorf("docs/troubleshooting.md describes %q as a warning, but %q is "+
					"error-severity (%s) and does refuse the save.\n\tsentence: %s",
					phrase, code, strings.Join(s.sites, ", "), strings.TrimSpace(sentence))
			}
		}
	}
}

// sentences splits markdown prose into rough sentences. It is deliberately
// crude — enough to attribute a phrase to the claim around it.
func sentences(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] != '.' || i+1 >= len(text) || text[i+1] != ' ' {
			continue
		}
		out = append(out, text[start:i+1])
		start = i + 1
	}
	return append(out, text[start:])
}
