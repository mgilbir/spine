package maporder

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// modulePath is this module's import path; it is how a resolved object is
// recognised as belonging to the repository rather than to the standard library.
const modulePath = "github.com/mgilbir/spine"

// Roots are the directories, relative to the repository root, whose packages
// are analysed. They are the ones that serialize: every OOXML part this library
// writes is built in one of them.
var Roots = []string{"chart", "common", "docx", "opc", "pptx", "xlsx"}

// Package is one analysed Go package: its AST, its type information and the
// file set the two share.
type Package struct {
	ImportPath string
	Dir        string
	Files      []*ast.File
	Info       *types.Info
	Pkg        *types.Package
}

// Program is every analysed package plus the shared file set.
type Program struct {
	Fset     *token.FileSet
	Packages []*Package
}

// Load parses and type-checks every non-test package under Roots.
//
// It is deliberately standard-library only: this module has no third-party
// requirements and golang.org/x/tools/go/packages would be one. The source
// importer type-checks dependencies from source as it goes, which is slower
// than reading export data but needs no build step, and one importer is shared
// across all packages so each dependency is checked once.
func Load(repoRoot string, dirs []string) (*Program, error) {
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)
	prog := &Program{Fset: fset}
	for _, dir := range dirs {
		rel, err := filepath.Rel(repoRoot, dir)
		if err != nil {
			return nil, err
		}
		importPath := modulePath + "/" + filepath.ToSlash(rel)
		files, err := parseDir(fset, dir)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			continue
		}
		info := &types.Info{
			Types:      map[ast.Expr]types.TypeAndValue{},
			Defs:       map[*ast.Ident]types.Object{},
			Uses:       map[*ast.Ident]types.Object{},
			Selections: map[*ast.SelectorExpr]*types.Selection{},
		}
		conf := types.Config{
			Importer: imp,
			// Type errors are swallowed rather than fatal: a package that does
			// not fully resolve still yields usable types for the expressions
			// that do, and the caller asserts a floor on how much was resolved
			// (see TestGuardSeesEnough) so a silently empty result cannot pass
			// for a clean sweep.
			Error: func(error) {},
		}
		tpkg, _ := conf.Check(importPath, fset, files, info)
		if tpkg == nil {
			return nil, fmt.Errorf("type-checking %s produced no package", importPath)
		}
		prog.Packages = append(prog.Packages, &Package{
			ImportPath: importPath,
			Dir:        dir,
			Files:      files,
			Info:       info,
			Pkg:        tpkg,
		})
	}
	if len(prog.Packages) == 0 {
		return nil, fmt.Errorf("no packages found under %s", repoRoot)
	}
	return prog, nil
}

// PackageDirs returns every directory under Roots that holds non-test Go
// sources, in sorted order.
func PackageDirs(repoRoot string) ([]string, error) {
	var dirs []string
	for _, root := range Roots {
		err := filepath.WalkDir(filepath.Join(repoRoot, root), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				return nil
			}
			switch d.Name() {
			case "testdata", "internal_testdata":
				return filepath.SkipDir
			}
			names, err := filepath.Glob(filepath.Join(path, "*.go"))
			if err != nil {
				return err
			}
			for _, n := range names {
				if !strings.HasSuffix(n, "_test.go") {
					dirs = append(dirs, path)
					break
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// parseDir parses the non-test Go files of one directory.
func parseDir(fset *token.FileSet, dir string) ([]*ast.File, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var files []*ast.File
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", name, err)
		}
		files = append(files, f)
	}
	return files, nil
}
