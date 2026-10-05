package pptxrender

import (
	"fmt"
	"strings"

	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// renderClass is the class of font DrawingML picks for a character: a:latin
// for Latin, Greek and Cyrillic text, ASCII and symbols, and a:ea for East
// Asian characters.
type renderClass int

const (
	classLatin renderClass = iota
	classEastAsian
)

// renderClassOf is the font class of a character, which depends on the
// character alone and not on its neighbours.
func renderClassOf(c rune) renderClass {
	if core.IsEastAsian(c) {
		return classEastAsian
	}
	return classLatin
}

// renderPiece is a stretch of a run's text drawn in one font.
type renderPiece struct {
	text, font string
	class      renderClass
}

// renderPieces cuts a run's text into stretches of one font class, each with
// the font that draws it. Text without characters is one piece in the Latin
// font. An East Asian character in a run with no East Asian font is drawn in
// the Latin font by best effort.
//
// PowerPoint may draw symbols of ambiguous East Asian width, such as quotes
// and arrows, with the East Asian font in a run whose language is East
// Asian. Which it does is not documented, so such text fails, and best effort
// draws the symbols with the Latin font.
func renderPieces(rs renderRunStyle, text string, colors *renderColors) ([]renderPiece, error) {
	if text == "" {
		return []renderPiece{{font: rs.font}}, nil
	}
	if rs.eastAsian {
		for _, c := range text {
			if c >= 0x80 && core.RepertoireEuropean.Allows(c) {
				if err := colors.approximate(fmt.Errorf("%w: non-ASCII symbol in an East Asian language", render.ErrUnsupported)); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	var pieces []renderPiece
	start, class := 0, classLatin
	flush := func(end int) error {
		if end == start {
			return nil
		}
		font := rs.font
		if class == classEastAsian {
			switch {
			case rs.eaFault != nil:
				if err := colors.approximate(rs.eaFault); err != nil {
					return err
				}
			case rs.ea != "":
				font = rs.ea
			default:
				if err := colors.approximate(fmt.Errorf("%w: East Asian text without an East Asian font", render.ErrUnsupported)); err != nil {
					return err
				}
			}
		}
		pieces = append(pieces, renderPiece{text: text[start:end], font: font, class: class})
		return nil
	}
	for i, c := range text {
		if k := renderClassOf(c); k != class {
			if err := flush(i); err != nil {
				return nil, err
			}
			start, class = i, k
		}
	}
	if err := flush(len(text)); err != nil {
		return nil, err
	}
	return pieces, nil
}

// renderLineHasEastAsian reports whether a line holds East Asian characters.
func renderLineHasEastAsian(line renderLine) bool {
	for _, sg := range line.Segments {
		if strings.IndexFunc(sg.Text, core.IsEastAsian) >= 0 {
			return true
		}
	}
	return false
}
