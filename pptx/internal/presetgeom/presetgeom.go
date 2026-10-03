// Package presetgeom holds the DrawingML preset shape geometries of ECMA-376
// Part 1, 5th edition (2016), from presetShapeDefinitions.xml in the
// standard's OfficeOpenXML-DrawingMLGeometries.zip. The embedded copy keeps
// only each shape's adjust values, guides, text rectangle and paths; handles
// and connection sites are dropped.
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

var (
	once    sync.Once
	shapes  map[string]*dml.CustGeom
	loadErr error
)

func load() {
	shapes = map[string]*dml.CustGeom{}
	d := xml.NewDecoder(bytes.NewReader(definitions))
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				loadErr = err
			}
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth != 2 {
				continue
			}
			var g dml.CustGeom
			if err := d.DecodeElement(&g, &t); err != nil {
				loadErr = fmt.Errorf("presetgeom: %s: %w", t.Name.Local, err)
				return
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
