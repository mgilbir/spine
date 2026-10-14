// Package renderfonts maps -font FAMILY[:style]=FILE[#FACE] flags to font
// faces for the renderers: the resolver spine-render and spine serve both
// use, the bounded reading of font files, and the faces charts are given.
package renderfonts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// Budget bounds the font files read, and the fonts copied out of font
// collections for charts.
const Budget = 32 << 20

// Flags is a repeatable -font flag value collecting the raw mapping strings.
type Flags []string

func (f *Flags) String() string     { return strings.Join(*f, ";") }
func (f *Flags) Set(s string) error { *f = append(*f, s); return nil }

// File is a mapped font's family and file, which charts are drawn with too.
// A face of a font collection is not a file of its own, and aster takes one
// font per file: it is copied out into a font of its own, from Face.
type File struct {
	Family string
	Data   []byte
	Face   *shape.Face // the face of a collection; nil for a font file
}

// Program is the font bytes charts are given.
func (f File) Program() []byte {
	if f.Face != nil {
		return f.Face.Program()
	}
	return f.Data
}

// SplitIndex splits the face index off a font location FILE or FILE#INDEX.
// The index is the digits after the last '#', which is the file's own where
// nothing or something else follows it; FILE#0 names a file whose name ends
// in a '#' and digits.
func SplitIndex(location string) (path string, index int, indexed bool, err error) {
	i := strings.LastIndex(location, "#")
	if i < 0 || i == len(location)-1 || strings.Trim(location[i+1:], "0123456789") != "" {
		return location, 0, false, nil
	}
	index, err = strconv.Atoi(location[i+1:])
	if err != nil || index > 1<<16 || i == 0 {
		return "", 0, false, fmt.Errorf("invalid font face index in %q", location)
	}
	return location[:i], index, true, nil
}

// IsCollection reports whether a font file is a TrueType or OpenType
// collection, bare or WOFF 2 wrapped.
func IsCollection(data []byte) bool {
	return bytes.HasPrefix(data, []byte("ttcf")) || (bytes.HasPrefix(data, []byte("wOF2")) && len(data) >= 8 && string(data[4:8]) == "ttcf")
}

// LoadFace loads a face of a font file: the face at index of a collection,
// or the font of a file that is none. A collection of several faces needs its
// index, so that the face is never a guess.
func LoadFace(data []byte, index int, indexed bool) (*shape.Face, error) {
	if !indexed {
		if faces, err := shape.CollectionFaces(data); err == nil && len(faces) > 1 {
			const shown = 8
			var list []string
			for _, f := range faces[:min(len(faces), shown)] {
				name := strings.TrimSpace(f.Family + " " + f.Subfamily)
				if f.Name != "" {
					name += " (" + f.Name + ")"
				}
				list = append(list, fmt.Sprintf("#%d %s", f.Index, name))
			}
			more := ""
			if len(faces) > shown {
				more = fmt.Sprintf("; and %d more", len(faces)-shown)
			}
			return nil, fmt.Errorf("the file holds %d faces; choose one with FILE#INDEX: %s%s", len(faces), strings.Join(list, "; "), more)
		}
	}
	return shape.LoadCollection(data, index)
}

// ReadFile reads a font file within the budget that is left.
func ReadFile(path string, left int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, left+1))
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > left {
		return nil, fmt.Errorf("font files exceed %d MiB budget", Budget>>20)
	}
	return data, nil
}

// Resolver builds the render.FontResolver for the given raw font mapping
// strings (the -font flag values, FAMILY[:style]=FILE[#FACE]) and the
// -fallback-noto choice, along with the font files charts are given.
func Resolver(specs []string, fallbackNoto bool) (render.FontResolver, []File, error) {
	if len(specs) > 32 {
		return nil, nil, fmt.Errorf("at most 32 font mappings are allowed")
	}
	var files []File
	faces := make(map[render.FontRequest]*shape.Face)
	// Each font file is read, and counted, once, however many of its faces
	// are mapped.
	read := make(map[string][]byte)
	var total int64
	for _, spec := range specs {
		key, location, ok := strings.Cut(spec, "=")
		if !ok || key == "" || location == "" {
			return nil, nil, fmt.Errorf("invalid font mapping %q", spec)
		}
		r := render.FontRequest{Family: key}
		if family, style, found := strings.Cut(key, ":"); found {
			r.Family = family
			switch style {
			case "regular":
			case "bold":
				r.Bold = true
			case "italic":
				r.Italic = true
			case "bolditalic":
				r.Bold = true
				r.Italic = true
			default:
				return nil, nil, fmt.Errorf("invalid font style %q", style)
			}
		}
		if r.Family == "" || faces[r] != nil {
			return nil, nil, fmt.Errorf("empty or duplicate font family %q", key)
		}
		path, index, indexed, err := SplitIndex(location)
		if err != nil {
			return nil, nil, err
		}
		data, ok := read[filepath.Clean(path)]
		if !ok {
			if data, err = ReadFile(path, Budget-total); err != nil {
				return nil, nil, err
			}
			total += int64(len(data))
			read[filepath.Clean(path)] = data
		}
		face, err := LoadFace(data, index, indexed)
		if err != nil {
			return nil, nil, fmt.Errorf("font %q: %w", key, err)
		}
		faces[r] = face
		file := File{Family: r.Family, Data: data}
		if IsCollection(data) {
			file.Face = face
		}
		files = append(files, file)
	}
	var fallback *shape.Face
	if fallbackNoto {
		var err error
		fallback, err = notosans.Face()
		if err != nil {
			return nil, nil, err
		}
	}
	return func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if face := faces[r]; face != nil {
			return face, nil
		}
		if fallback != nil && !r.Bold && !r.Italic {
			return fallback, nil
		}
		return nil, fmt.Errorf("unresolved font %+v; provide -font or explicitly choose -fallback-noto for regular fonts", r)
	}, files, nil
}
