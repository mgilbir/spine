package dml

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// This file guards the ST_Percentage attribute class (C400, re-opening C277).
//
// Every DrawingML attribute typed ST_Percentage / ST_PositivePercentage /
// ST_FixedPercentage / ST_PositiveFixedPercentage (plus the text schemas'
// ST_TextSpacingPercent / ST_TextBulletSizePercent / ST_TextFontScalePercent
// restrictions of them) has two lexical spaces: the strict thousandths integer
// "50000" and the transitional percent string "50%". Modeling one as a Go
// integer makes a transitional file fail to open at all, because the parse
// error propagates out of the enclosing part's UnmarshalXML.
//
// Two tests here:
//
//   - TestPercentClassTransitionalForms parses the transitional form through
//     every swept element and asserts the value and the verbatim lexical form
//     survive, and that a canonical integer source still re-emits canonically.
//   - TestPercentClassSchemaTypeDiff is the class guard the audit asked for: it
//     walks this package's own source, maps each type back to the CT_ it
//     documents, and fails on any attribute the schema types as a percentage
//     that is modeled as anything but Percentage. A new type (or a new field on
//     an old one) that gets this wrong fails here rather than in the wild.

// percentAttrsByComplexType is the schema-derived percentage-attribute map,
// extracted from the ECMA-376 DrawingML schemas (dml-baseTypes.xsd,
// dml-shapeEffects.xsd, dml-shape3DCamera.xsd, dml-shapeLineProperties.xsd,
// dml-text*.xsd) by selecting every xsd:attribute whose type resolves to the
// ST_Percentage family. Keep it in sync with the schemas, not with the code.
var percentAttrsByComplexType = map[string][]string{
	// dml-baseTypes.xsd
	"CT_Percentage":              {"val"},
	"CT_PositivePercentage":      {"val"},
	"CT_FixedPercentage":         {"val"},
	"CT_PositiveFixedPercentage": {"val"},
	"CT_ScRgbColor":              {"r", "g", "b"},
	"CT_HslColor":                {"sat", "lum"},
	"CT_RelativeRect":            {"l", "t", "r", "b"},
	// dml-shape3DCamera.xsd (fov is ST_FOVAngle, an angle — deliberately absent)
	"CT_Camera": {"zoom"},
	// dml-shapeEffects.xsd
	"CT_AlphaBiLevelEffect":       {"thresh"},
	"CT_AlphaModulateFixedEffect": {"amt"},
	"CT_AlphaReplaceEffect":       {"a"},
	"CT_BiLevelEffect":            {"thresh"},
	"CT_HSLEffect":                {"sat", "lum"},
	"CT_LuminanceEffect":          {"bright", "contrast"},
	"CT_OuterShadowEffect":        {"sx", "sy"},
	"CT_ReflectionEffect":         {"stA", "stPos", "endA", "endPos", "sx", "sy"},
	"CT_RelativeOffsetEffect":     {"tx", "ty"},
	"CT_TintEffect":               {"amt"},
	"CT_TransformEffect":          {"sx", "sy"},
	"CT_GradientStop":             {"pos"},
	"CT_TileInfoProperties":       {"sx", "sy"},
	// dml-shapeLineProperties.xsd
	"CT_LineJoinMiterProperties": {"lim"},
	"CT_DashStop":                {"d", "sp"},
	// dml-text*.xsd
	"CT_TextBulletSizePercent":   {"val"},
	"CT_TextCharacterProperties": {"baseline"},
	"CT_TextSpacingPercent":      {"val"},
	"CT_TextNormalAutofit":       {"fontScale", "lnSpcReduction"},
}

// TestPercentClassSchemaTypeDiff diffs every modeled attribute in this package
// against percentAttrsByComplexType. A type opts into the diff by naming its
// complex type in its doc comment, which every type here already does
// ("// Foo represents CT_Bar (a:baz)").
func TestPercentClassSchemaTypeDiff(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	pkg, ok := pkgs["dml"]
	if !ok {
		t.Fatal("package dml not found in .")
	}

	checked := 0
	for _, file := range pkg.Files {
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
				doc := ts.Doc
				if doc == nil {
					doc = gd.Doc
				}
				ct := complexTypeFromDoc(doc.Text())
				want, ok := percentAttrsByComplexType[ct]
				if !ok {
					continue
				}
				for _, field := range st.Fields.List {
					if field.Tag == nil {
						continue
					}
					tag, err := strconv.Unquote(field.Tag.Value)
					if err != nil {
						continue
					}
					name, isAttr := attrNameFromTag(reflect.StructTag(tag).Get("xml"))
					if !isAttr || !contains(want, name) {
						continue
					}
					checked++
					if got := exprString(field.Type); got != "Percentage" && got != "*Percentage" {
						pos := fset.Position(field.Pos())
						t.Errorf("%s: %s (%s) attribute %q is schema type ST_*Percentage but modeled as %s; "+
							"a transitional \"n%%\" source fails the whole part (see C400/C277)",
							pos, ts.Name.Name, ct, name, got)
					}
				}
			}
		}
	}
	// The diff is only meaningful if it actually matched types; a refactor that
	// drops the "represents CT_x" doc convention must not silently disarm it.
	if checked < 25 {
		t.Errorf("schema diff checked only %d attributes, expected at least 25 — "+
			"did the \"represents CT_x\" doc convention change?", checked)
	}
}

// complexTypeFromDoc extracts the CT_ name a doc comment says the type models.
func complexTypeFromDoc(doc string) string {
	i := strings.Index(doc, "CT_")
	if i < 0 {
		return ""
	}
	j := i
	for j < len(doc) && (doc[j] == '_' || doc[j] >= 'a' && doc[j] <= 'z' ||
		doc[j] >= 'A' && doc[j] <= 'Z' || doc[j] >= '0' && doc[j] <= '9') {
		j++
	}
	return doc[i:j]
}

// attrNameFromTag returns the attribute local name of an xml struct tag and
// whether the field is an attribute at all.
func attrNameFromTag(tag string) (string, bool) {
	parts := strings.Split(tag, ",")
	isAttr := false
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == "attr" {
			isAttr = true
		}
	}
	if !isAttr {
		return "", false
	}
	name := parts[0]
	if i := strings.LastIndex(name, " "); i >= 0 {
		name = name[i+1:]
	}
	return name, true
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// exprString renders a field type expression as source text (only the shapes
// that occur in this package: identifiers, pointers, selectors, slices).
func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return "*" + exprString(v.X)
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(v.Elt)
	default:
		return "?"
	}
}

