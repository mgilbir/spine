package pptxrender

import (
	"fmt"
	"strings"
	"unicode/utf8"

	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

// renderClass is the class of font DrawingML picks for a character: a:latin
// for Latin, Greek and Cyrillic text, ASCII and symbols, a:ea for East Asian
// characters and a:cs for the complex scripts, here Hebrew, Arabic and their
// neighbours.
type renderClass int

const (
	classLatin renderClass = iota
	classEastAsian
	classComplex
)

// renderClassOf is the font class of a character, which depends on the
// character alone and not on its neighbours. The joiners and direction marks
// have no class of their own and are set in that of the text they are among;
// see renderPieces.
func renderClassOf(c rune) renderClass {
	switch {
	case core.IsEastAsian(c):
		return classEastAsian
	case core.IsComplex(c):
		return classComplex
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
// font. A character in a run with no font for its class is drawn in the Latin
// font by best effort.
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
	// In a language of a complex script, ASCII digits and spaces take the
	// complex-script font, as measured in PowerPoint's own export; elsewhere
	// they take the Latin one. How other ASCII punctuation and symbols are
	// drawn there was not measured, so such text fails, and best effort
	// draws it with the Latin font.
	if rs.complexLang {
		for _, c := range text {
			if c > ' ' && c < 0x7F && (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				if err := colors.approximate(fmt.Errorf("%w: ASCII punctuation in a complex-script language", render.ErrUnsupported)); err != nil {
					return nil, err
				}
				break
			}
		}
	}
	classOf := func(c rune) renderClass {
		if rs.complexLang && (c == ' ' || c >= '0' && c <= '9') {
			return classComplex
		}
		return renderClassOf(c)
	}
	var pieces []renderPiece
	start, class := 0, classOf(firstClassed(text))
	flush := func(end int) error {
		if end == start {
			return nil
		}
		font, err := rs.classFont(class, text[start:end], colors)
		if err != nil {
			return err
		}
		pieces = append(pieces, renderPiece{text: text[start:end], font: font, class: class})
		return nil
	}
	for i, c := range text {
		if core.IsFormat(c) {
			continue
		}
		if k := classOf(c); k != class {
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

// firstClassed is the first character of text that has a class of its own.
func firstClassed(text string) rune {
	for _, c := range text {
		if !core.IsFormat(c) {
			return c
		}
	}
	return 0
}

// classFont is the font that draws a stretch of text of one class in a run. A
// problem with the slot of a class matters only to text of that class.
func (rs renderRunStyle) classFont(class renderClass, text string, colors *renderColors) (string, error) {
	var (
		missing string
		name    string
		fault   error
	)
	switch class {
	case classEastAsian:
		missing, name, fault = "East Asian text without an East Asian font", rs.ea, rs.eaFault
	case classComplex:
		missing, name, fault = "complex-script text without a complex-script font", rs.cs, rs.csFault
		if name == "" && fault == nil && rs.csList != nil {
			// A theme font the theme leaves empty is found by the script of
			// the text, when the language gave none.
			v, err := renderScriptFont(rs.csList, "cs", renderComplexScript(text))
			if err != nil {
				fault = err
			}
			name = v
		}
	default:
		return rs.font, nil
	}
	switch {
	case fault != nil:
		if err := colors.approximate(fault); err != nil {
			return "", err
		}
	case name != "":
		return name, nil
	default:
		if err := colors.approximate(fmt.Errorf("%w: %s", render.ErrUnsupported, missing)); err != nil {
			return "", err
		}
	}
	return rs.font, nil
}

// renderComplexScript is the ISO 15924 script of the first complex-script
// character of text, as a theme's font list names it.
func renderComplexScript(text string) string {
	for _, c := range text {
		switch {
		case c >= 0x590 && c <= 0x5FF, c >= 0xFB1D && c <= 0xFB4F:
			return "Hebr"
		case c >= 0x700 && c <= 0x74F:
			return "Syrc"
		case c >= 0x780 && c <= 0x7BF:
			return "Thaa"
		case c >= 0x7C0 && c <= 0x7FF:
			return "Nkoo"
		case core.IsComplex(c):
			return "Arab"
		}
	}
	return ""
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

// renderLineHasArabic reports whether a line holds Arabic script, which low
// kashida justification elongates.
func renderLineHasArabic(line renderLine) bool {
	for _, sg := range line.Segments {
		for i := 0; i < len(sg.Text); {
			c, n := utf8.DecodeRuneInString(sg.Text[i:])
			if core.IsComplex(c) && renderComplexScript(sg.Text[i:i+n]) == "Arab" {
				return true
			}
			i += n
		}
	}
	return false
}
