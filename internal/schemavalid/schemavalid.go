// Package schemavalid validates parts this library authors against the OOXML
// XML Schema definitions, using libxml2 through the xmllint binary.
//
// It exists because nothing else here can see a conformance error in authored
// content. The corpus proves *fidelity* — a document read and written back
// comes out byte for byte — which says nothing about a part built from nothing
// by Create and the authoring APIs, because there is no original to compare it
// to. common/validate covers the cross-part semantics a schema cannot express
// (a relationship id that resolves, a sheet the workbook names actually
// existing), and its catalog is hand-written. Neither notices that a child
// element sits in the wrong place.
//
// OOXML content models are xsd:sequence, so order is normative: PowerPoint and
// Word reject a part whose children are transposed with "the file is corrupt",
// while the bytes stay well-formed, the namespaces stay right and every value
// survives a round trip through this library's own model. That is the class
// this package watches, along with missing required attributes, values outside
// an enumeration, and datatypes out of range.
//
// # Which schemas
//
// spec/part4/xsd, the ISO/IEC 29500-4 Transitional set, whose target namespaces
// (schemas.openxmlformats.org/...) are the ones this library writes. The Part 1
// schemas in spec/part1/xsd are the Strict variant (purl.oclc.org/ooxml/...),
// which opc.ErrStrictOOXML exists to reject: validating against those would
// fail on the root element of every document spine produces.
package schemavalid

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SchemaDirs are the vendored schema sets, relative to the repository root.
//
// Part 4 is the Transitional markup itself, whose target namespaces
// (schemas.openxmlformats.org/...) are the ones this library writes. Part 2 is
// the package layer — core properties, content types, relationships — which
// lives in its own standard and its own namespaces, and which nothing else
// checks either.
var SchemaDirs = []string{"spec/part4/xsd", "spec/part2/xsd"}

// skipSchemas are vendored schemas left out of the set, by file name.
//
// A schema that cannot resolve its own references does not merely fail for its
// own part: libxml2 refuses to build the whole set, and every part then fails
// with a schema error dressed up as a conformance error. So the exclusion is
// stated here rather than discovered later.
var skipSchemas = map[string]string{
	// Core properties are defined in terms of Dublin Core (dc:creator,
	// dcterms:created), whose declarations live in two DCMI schemas that are
	// not part of ISO 29500 and are not vendored. docProps/core.xml is left to
	// the fidelity tests.
	"opc-coreProperties.xsd": "references Dublin Core schemas that are not vendored",
}

// Validator holds a prepared schema set. Building it parses 27 schema documents
// and resolves their imports, which is slow enough to be worth doing once per
// test binary rather than per part.
type Validator struct {
	// wrapper is the path of the generated schema document that imports every
	// vendored schema. A multi-file schema set has no single entry point —
	// wml.xsd does not import sml.xsd, and a part may legitimately be rooted in
	// any of them — so one wrapper importing all of them is what lets a single
	// prepared schema validate any part.
	wrapper    string
	backend    backend
	namespaces map[string]bool
}

// backend is how a prepared schema is applied to a document. xmllint is the
// reference implementation; lxml exposes the same libxml2 engine through
// Python, and is accepted so a contributor who has one but not the other still
// runs the checks rather than skipping them.
type backend interface {
	validate(v *Validator, part []byte) error
	name() string
}

// Available reports whether any validator backend is installed, and which.
func Available() (string, bool) {
	if b := pickBackend(); b != nil {
		return b.name(), true
	}
	return "", false
}

// SchemasPresent reports whether the vendored schema set is on disk.
//
// It usually is not. The ISO/IEC 29500 documents are copyrighted and are not
// redistributable, so spec/part1..4 are gitignored and spec/README.md explains
// how to obtain them — which means this suite runs for a developer who has
// bought the standard and skips everywhere else, including CI. The same
// arrangement as the Common Crawl corpus, for the same reason.
func SchemasPresent(repoRoot string) bool {
	for _, rel := range SchemaDirs {
		entries, err := os.ReadDir(filepath.Join(repoRoot, rel))
		if err != nil {
			return false
		}
		found := false
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".xsd") {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func pickBackend() backend {
	if p, err := exec.LookPath("xmllint"); err == nil {
		return xmllintBackend{path: p}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		if err := exec.Command(p, "-c", "import lxml.etree").Run(); err == nil {
			return lxmlBackend{path: p}
		}
	}
	return nil
}

// New prepares a validator over the schema set under repoRoot. The caller is
// responsible for removing the returned temporary state with Close.
func New(repoRoot string) (*Validator, error) {
	b := pickBackend()
	if b == nil {
		return nil, fmt.Errorf("schemavalid: no validator available (install libxml2-utils for xmllint, or python3-lxml)")
	}
	var paths []string
	for _, rel := range SchemaDirs {
		dir := filepath.Join(repoRoot, rel)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("schemavalid: reading %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".xsd") {
				continue
			}
			if _, skip := skipSchemas[e.Name()]; skip {
				continue
			}
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("schemavalid: no schemas under %v", SchemaDirs)
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + "\n")
	seen := map[string]bool{}
	for _, path := range paths {
		ns, err := targetNamespace(path)
		if err != nil {
			return nil, err
		}
		// One import per namespace: two schema documents claiming the same
		// target namespace is a redefinition libxml2 rejects outright.
		if ns == "" || seen[ns] {
			continue
		}
		seen[ns] = true
		fmt.Fprintf(&sb, "  <xs:import namespace=%q schemaLocation=%q/>\n", ns, path)
	}
	sb.WriteString("</xs:schema>\n")

	tmp, err := os.CreateTemp("", "spine-ooxml-*.xsd")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.WriteString(sb.String()); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return &Validator{wrapper: tmp.Name(), backend: b, namespaces: seen}, nil
}

// Backend names the validator implementation in use, for the test log.
func (v *Validator) Backend() string { return v.backend.name() }

// Close removes the generated wrapper schema.
func (v *Validator) Close() error {
	if v == nil || v.wrapper == "" {
		return nil
	}
	return os.Remove(v.wrapper)
}

// Validate checks one XML part against the schema set and returns nil when it
// conforms.
//
// Markup compatibility is applied first, the way a consumer must: an
// mc:AlternateContent collapses to its fallback, and the namespaces the
// producer declared ignorable are dropped. What remains is markup the ISO
// schemas describe and a conforming consumer must understand.
func (v *Validator) Validate(part []byte) error {
	return v.backend.validate(v, StripIgnorable(ResolveAlternateContent(part)))
}

// targetNamespace reads a schema's targetNamespace attribute without parsing
// the whole document.
func targetNamespace(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	const attr = `targetNamespace="`
	i := bytes.Index(data, []byte(attr))
	if i < 0 {
		return "", nil
	}
	rest := data[i+len(attr):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return "", fmt.Errorf("schemavalid: unterminated targetNamespace in %s", path)
	}
	return string(rest[:j]), nil
}

// Describes reports whether the schema set defines the given namespace, which
// is how a caller tells "this part is wrong" from "no schema here describes
// this part". The Microsoft extension parts a document may carry — w15
// commentsExtended and people, the p14/a14 extension parts — are defined by
// [MS-DOCX] and friends, not by ISO 29500, so nothing in this set describes
// their roots.
func (v *Validator) Describes(namespace string) bool { return v.namespaces[namespace] }

// RootNamespace returns the namespace of a part's root element, or "" when the
// part does not parse far enough to have one.
func RootNamespace(part []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(part))
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Space
		}
	}
}
