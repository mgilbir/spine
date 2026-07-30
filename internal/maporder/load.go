package maporder

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// modulePath is this module's import path.
const modulePath = "github.com/mgilbir/spine"

// Patterns are the package patterns whose sources are analysed: the six roots
// that serialize. Every OOXML part this library writes is built in one of them.
var Patterns = []string{
	"./chart/...",
	"./common/...",
	"./docx/...",
	"./opc/...",
	"./pptx/...",
	"./xlsx/...",
}

// Package is one analysed Go package: its syntax, its type information and the
// file set they share.
type Package struct {
	ImportPath string
	Files      []*ast.File
	Info       *types.Info
	Pkg        *types.Package
}

// Program is every analysed package plus the shared file set.
type Program struct {
	Fset     *token.FileSet
	Packages []*Package
}

// Load type-checks the packages matching patterns, rooted at dir.
//
// It uses golang.org/x/tools/go/packages rather than go/importer: the module
// resolution, build tags and per-package configuration are handled properly,
// the dependency graph is loaded once from the build cache instead of being
// re-type-checked from source, and — the reason that matters for the call graph
// below — every package in one Load shares object identity, so a call from docx
// into opc resolves to the very *types.Func this analyser indexed from opc's
// own syntax. The source importer gives none of that, and took four times as
// long.
//
// Loading is strict about errors. The classification cannot tell a map from a
// slice without types, so a package that failed to type-check would silently
// report clean — the exact failure a guard must not have.
func Load(dir string, patterns []string) (*Program, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:   dir,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading %v: %w", patterns, err)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched %v in %s", patterns, dir)
	}
	prog := &Program{}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("loading %s: %v (%d errors)", p.PkgPath, p.Errors[0], len(p.Errors))
		}
		if p.TypesInfo == nil || p.Types == nil {
			return nil, fmt.Errorf("loading %s: no type information", p.PkgPath)
		}
		if len(p.Syntax) == 0 {
			return nil, fmt.Errorf("loading %s: no syntax", p.PkgPath)
		}
		if prog.Fset == nil {
			prog.Fset = p.Fset
		}
		prog.Packages = append(prog.Packages, &Package{
			ImportPath: p.PkgPath,
			Files:      p.Syntax,
			Info:       p.TypesInfo,
			Pkg:        p.Types,
		})
	}
	sort.Slice(prog.Packages, func(i, j int) bool {
		return prog.Packages[i].ImportPath < prog.Packages[j].ImportPath
	})
	return prog, nil
}

// ShortPath trims the module prefix from an import path, giving the form the
// exemption keys and landmarks are written in.
func ShortPath(importPath string) string {
	return strings.TrimPrefix(importPath, modulePath+"/")
}
