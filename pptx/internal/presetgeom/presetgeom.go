// Package presetgeom holds the DrawingML preset shape geometries of ECMA-376
// Part 1, 5th edition (2016), from presetShapeDefinitions.xml in the
// standard's OfficeOpenXML-DrawingMLGeometries.zip. The embedded copy keeps
// only each shape's adjust values, guides, text rectangle and paths; handles
// and connection sites are dropped. The preset text warps come from
// presetTextWarpDefinitions.xml in the same archive, embedded as published.
package presetgeom

import (
	"bytes"
	_ "embed"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/mgilbir/spine/common/dml"
)

//go:embed presetShapeDefinitions.xml
var definitions []byte

//go:embed presetTextWarpDefinitions.xml
var warpDefinitions []byte

var (
	once    sync.Once
	shapes  map[string]*dml.CustGeom
	warps   map[string]*dml.CustGeom
	loadErr error
)

func load() {
	if shapes, loadErr = parse(definitions); loadErr != nil {
		return
	}
	warps, loadErr = parse(warpDefinitions)
}

// parse reads a definitions document: one geometry per root child.
func parse(definitions []byte) (map[string]*dml.CustGeom, error) {
	shapes := map[string]*dml.CustGeom{}
	d := xml.NewDecoder(bytes.NewReader(definitions))
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return nil, err
			}
			return shapes, nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth != 2 {
				continue
			}
			var g dml.CustGeom
			if err := d.DecodeElement(&g, &t); err != nil {
				return nil, fmt.Errorf("presetgeom: %s: %w", t.Name.Local, err)
			}
			depth--
			// The standard defines upDownArrow twice, alike; keep the first.
			if _, dup := shapes[t.Name.Local]; !dup {
				shapes[t.Name.Local] = &g
			}
		case xml.EndElement:
			depth--
		}
	}
}

// Lookup returns a preset's geometry, which the caller must not modify, and
// whether the preset exists.
func Lookup(name string) (*dml.CustGeom, bool) {
	once.Do(load)
	if loadErr != nil {
		return nil, false
	}
	g, ok := shapes[name]
	return g, ok
}

// LookupTextWarp returns a preset text warp's geometry, which the caller
// must not modify, and whether the warp exists. Its paths outline where
// text is drawn: in pairs, the top and bottom of a band of lines, or each
// alone, a line the text follows.
func LookupTextWarp(name string) (*dml.CustGeom, bool) {
	once.Do(load)
	if loadErr != nil {
		return nil, false
	}
	g, ok := warps[name]
	return g, ok
}

// Names lists the presets, sorted.
func Names() []string {
	once.Do(load)
	names := make([]string, 0, len(shapes))
	for n := range shapes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Err reports a failure to load the embedded definitions.
func Err() error {
	once.Do(load)
	return loadErr
}
