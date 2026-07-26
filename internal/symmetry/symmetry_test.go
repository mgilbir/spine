// Package symmetry_test enforces cross-format API symmetry for the user-facing
// capability types shared by the docx, xlsx, and pptx packages.
//
// A core project principle is that a capability (comments, hyperlinks, images)
// should expose the same method set across all three formats, with differences
// only where a format genuinely requires them. This test turns that principle
// into a regression guard:
//
//   - For each capability it declares the shared method set (name + signature).
//     Every format's capability type must expose each shared method with a
//     matching signature; a self-referential return (e.g. Replies, Parent,
//     Reply) is matched against that format's own type.
//   - Every remaining exported method that a format declares directly on its
//     capability type (methods promoted from an embedded type, such as
//     pptx.Picture's BaseShape, are excluded) must be listed in that
//     capability's allow-list of documented format-only differences.
//
// The second rule is what makes the guard bite: if someone later adds a method
// to one format's capability type without either adding it to the other two
// (making it shared) or documenting it as format-specific, this test fails and
// points at the unclassified method.
package symmetry_test

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

var (
	tString  = reflect.TypeOf("")
	tBool    = reflect.TypeOf(true)
	tInt64   = reflect.TypeOf(int64(0))
	tFloat64 = reflect.TypeOf(float64(0))
	tBytes   = reflect.TypeOf([]byte(nil))
	tTime    = reflect.TypeOf(time.Time{})
)

// retSpec describes one expected return value of a shared method. Exactly one
// field is meaningful: typ for a concrete type, self for a pointer to the
// capability type itself, selfSlice for a slice of such pointers.
type retSpec struct {
	typ       reflect.Type
	self      bool
	selfSlice bool
}

func ret(t reflect.Type) retSpec { return retSpec{typ: t} }
func retSelf() retSpec           { return retSpec{self: true} }
func retSelfSlice() retSpec      { return retSpec{selfSlice: true} }

// methodSpec is a shared method's name and signature (excluding the receiver).
// A nil entry in args marks the capability type itself as the parameter.
type methodSpec struct {
	name string
	args []reflect.Type
	rets []retSpec
}

// capability bundles a shared method set with, per format, the pointer type of
// the capability handle and the allow-list of that format's documented
// format-only methods.
type capability struct {
	name   string
	shared []methodSpec
	// formats maps a format label to its capability handle pointer type.
	formats map[string]reflect.Type
	// allow maps a format label to the set of exported methods declared
	// directly on that format's type that are intentionally format-specific.
	allow map[string]map[string]bool
}

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// capabilities is the authoritative table of cross-format capabilities.
var capabilities = []capability{
	{
		name: "Comment",
		shared: []methodSpec{
			{name: "ID", rets: []retSpec{ret(tString)}},
			{name: "Author", rets: []retSpec{ret(tString)}},
			{name: "Text", rets: []retSpec{ret(tString)}},
			{name: "Date", rets: []retSpec{ret(tTime)}},
			{name: "Resolved", rets: []retSpec{ret(tBool)}},
			{name: "Replies", rets: []retSpec{retSelfSlice()}},
			{name: "Parent", rets: []retSpec{retSelf()}},
			{name: "Reply", args: []reflect.Type{tString, tString}, rets: []retSpec{retSelf()}},
			{name: "Resolve"},
			{name: "SetResolved", args: []reflect.Type{tBool}},
		},
		formats: map[string]reflect.Type{
			"docx": reflect.TypeOf(&docx.Comment{}),
			"xlsx": reflect.TypeOf(&xlsx.Comment{}),
			"pptx": reflect.TypeOf(&pptx.Comment{}),
		},
		allow: map[string]map[string]bool{
			// docx: range-precise anchoring, initials, and body paragraphs.
			"docx": set("AnchorText", "Initials", "Paragraphs", "SetInitials"),
			// xlsx: cell anchor, the threaded-vs-legacy-note distinction, and
			// per-run rich text on legacy notes.
			"xlsx": set("Ref", "Threaded", "RichText", "SetRichText"),
			// pptx: slide/point/shape anchoring.
			"pptx": set("AnchorShapeID", "Position", "Slide"),
		},
	},
	{
		name: "Hyperlink",
		shared: []methodSpec{
			{name: "URL", rets: []retSpec{ret(tString)}},
			{name: "Anchor", rets: []retSpec{ret(tString)}},
			{name: "Tooltip", rets: []retSpec{ret(tString)}},
			{name: "SetTooltip", args: []reflect.Type{tString}},
		},
		formats: map[string]reflect.Type{
			"docx": reflect.TypeOf(&docx.Hyperlink{}),
			"xlsx": reflect.TypeOf(&xlsx.Hyperlink{}),
			"pptx": reflect.TypeOf(&pptx.Hyperlink{}),
		},
		allow: map[string]map[string]bool{
			// docx: a hyperlink wraps display runs / text.
			"docx": set("Runs", "Text"),
			// xlsx: cell anchor.
			"xlsx": set("Ref"),
			"pptx": set(),
		},
	},
	{
		name: "Image",
		shared: []methodSpec{
			{name: "Width", rets: []retSpec{ret(tFloat64)}},
			{name: "Height", rets: []retSpec{ret(tFloat64)}},
			{name: "WidthEMU", rets: []retSpec{ret(tInt64)}},
			{name: "HeightEMU", rets: []retSpec{ret(tInt64)}},
			{name: "AltText", rets: []retSpec{ret(tString)}},
			{name: "Data", rets: []retSpec{ret(tBytes)}},
			{name: "ContentType", rets: []retSpec{ret(tString)}},
			{name: "PartName", rets: []retSpec{ret(tString)}},
		},
		formats: map[string]reflect.Type{
			"docx": reflect.TypeOf(&docx.InlineImage{}),
			"xlsx": reflect.TypeOf(&xlsx.Image{}),
			"pptx": reflect.TypeOf(&pptx.Picture{}),
		},
		allow: map[string]map[string]bool{
			// docx: inline-vs-floating flag and the mutation setters.
			"docx": set("Floating", "SetAltText", "SetSize"),
			// xlsx: top-left cell anchor; SVGData returns the original SVG bytes
			// of an image added as SVG (Data returns the raster fallback).
			"xlsx": set("AnchorCell", "SVGData"),
			// pptx: a Picture is also a shape, so it carries the picture-specific
			// image/crop/hyperlink surface (shape geometry is promoted from the
			// embedded BaseShape and excluded by directMethods).
			"pptx": set(
				"ShapeType", "ImagePath", "SetImagePath", "ImageData", "SetImageData",
				"SetSVGImageData", "SetSVGData", "SetImage", "Description", "SetDescription",
				"SetHyperlink", "SetActionHyperlink", "SetHyperlinkToSlide",
				"CropLeft", "SetCropLeft", "CropRight", "SetCropRight",
				"CropTop", "SetCropTop", "CropBottom", "SetCropBottom", "SetCrop",
			),
		},
	},
}

// TestSharedMethodsExist asserts every shared method is present on every
// format's capability type with a matching signature.
func TestSharedMethodsExist(t *testing.T) {
	for _, capab := range capabilities {
		for format, ct := range capab.formats {
			for _, spec := range capab.shared {
				m, ok := ct.MethodByName(spec.name)
				if !ok {
					t.Errorf("%s: %s.%s is missing (shared method)", capab.name, format, spec.name)
					continue
				}
				if err := checkSignature(m.Type, spec, ct); err != nil {
					t.Errorf("%s: %s.%s signature mismatch: %v", capab.name, format, spec.name, err)
				}
			}
		}
	}
}

// TestNoUnclassifiedMethods asserts every exported method declared directly on
// a format's capability type is either a shared method or a documented
// format-only method in the allow-list. A method that is neither fails the test
// — that is the guard against adding a shared-looking method to one format
// without extending the others (or documenting the difference).
func TestNoUnclassifiedMethods(t *testing.T) {
	for _, capab := range capabilities {
		shared := make(map[string]bool, len(capab.shared))
		for _, s := range capab.shared {
			shared[s.name] = true
		}
		for format, ct := range capab.formats {
			allow := capab.allow[format]
			for name := range directMethods(ct) {
				if shared[name] || allow[name] {
					continue
				}
				t.Errorf("%s: %s.%s is neither a shared method nor a documented "+
					"format-only method. Either add it to the other formats (make it "+
					"shared) or record it in the %q allow-list.", capab.name, format, name, format)
			}
		}
	}
}

// TestAllowListsAreAccurate keeps the allow-lists honest: every name listed as a
// format-only method must actually exist on that format's type, so stale
// entries cannot silently mask a later regression.
func TestAllowListsAreAccurate(t *testing.T) {
	for _, capab := range capabilities {
		for format, ct := range capab.formats {
			direct := directMethods(ct)
			names := make([]string, 0, len(capab.allow[format]))
			for n := range capab.allow[format] {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				if !direct[n] {
					t.Errorf("%s: %q allow-list names %q, which is not a direct exported "+
						"method of that format's type (stale entry).", capab.name, format, n)
				}
			}
		}
	}
}

// checkSignature verifies a method's func type matches the spec. ft includes the
// receiver as In(0); self is the capability pointer type used to resolve
// self-referential returns.
func checkSignature(ft reflect.Type, spec methodSpec, self reflect.Type) error {
	if got := ft.NumIn() - 1; got != len(spec.args) {
		return fmt.Errorf("arg count: got %d, want %d", got, len(spec.args))
	}
	for i, want := range spec.args {
		if got := ft.In(i + 1); got != want {
			return fmt.Errorf("arg %d: got %s, want %s", i, got, want)
		}
	}
	if ft.NumOut() != len(spec.rets) {
		return fmt.Errorf("return count: got %d, want %d", ft.NumOut(), len(spec.rets))
	}
	for i, want := range spec.rets {
		got := ft.Out(i)
		switch {
		case want.selfSlice:
			if got.Kind() != reflect.Slice || got.Elem() != self {
				return fmt.Errorf("return %d: got %s, want []%s", i, got, self)
			}
		case want.self:
			if got != self {
				return fmt.Errorf("return %d: got %s, want %s", i, got, self)
			}
		default:
			if got != want.typ {
				return fmt.Errorf("return %d: got %s, want %s", i, got, want.typ)
			}
		}
	}
	return nil
}

// directMethods returns the exported methods declared directly on the pointer
// type pt, excluding any promoted from an embedded (anonymous) field — those are
// inherited base-type behavior, not part of the capability's own surface.
func directMethods(pt reflect.Type) map[string]bool {
	inherited := map[string]bool{}
	if elem := pt.Elem(); elem.Kind() == reflect.Struct {
		for i := 0; i < elem.NumField(); i++ {
			f := elem.Field(i)
			if !f.Anonymous {
				continue
			}
			for _, et := range []reflect.Type{f.Type, reflect.PointerTo(f.Type)} {
				for j := 0; j < et.NumMethod(); j++ {
					inherited[et.Method(j).Name] = true
				}
			}
		}
	}
	out := map[string]bool{}
	for i := 0; i < pt.NumMethod(); i++ {
		if m := pt.Method(i); !inherited[m.Name] {
			out[m.Name] = true
		}
	}
	return out
}
