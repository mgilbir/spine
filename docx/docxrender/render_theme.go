package docxrender

import (
	"math"
	"strconv"
	"strings"
)

// wordRGB is an sRGB colour with eight-bit channels.
type wordRGB struct{ r, g, b uint8 }

// wordTheme holds what the renderer reads from the theme part: the major and
// minor font families (latin, east asian, complex script) and the colour scheme.
type wordTheme struct {
	// fonts[0] is the major font scheme, fonts[1] the minor; each holds the
	// latin, ea and cs typefaces.
	fonts  [2][3]string
	colors map[string]wordRGB
}

// wordRenderTheme reads a parsed theme part. A missing part is a nil result.
func wordRenderTheme(root *wordNode) *wordTheme {
	if root == nil {
		return nil
	}
	t := &wordTheme{colors: map[string]wordRGB{}}
	elements := root.childIn(nsA, "themeElements")
	if scheme := elements.childIn(nsA, "fontScheme"); scheme != nil {
		for i, name := range [2]string{"majorFont", "minorFont"} {
			f := scheme.childIn(nsA, name)
			for j, face := range [3]string{"latin", "ea", "cs"} {
				if c := f.childIn(nsA, face); c != nil {
					for _, a := range c.attrs {
						if a.space == "" && a.name == "typeface" {
							t.fonts[i][j] = a.value
						}
					}
				}
			}
		}
	}
	if scheme := elements.childIn(nsA, "clrScheme"); scheme != nil {
		for _, c := range scheme.children {
			if c.space != nsA || len(c.children) != 1 {
				continue
			}
			v := c.children[0]
			attr := "val"
			if v.name == "sysClr" {
				attr = "lastClr"
			} else if v.name != "srgbClr" {
				continue
			}
			for _, a := range v.attrs {
				if a.space == "" && a.name == attr {
					if rgb, ok := wordParseHex(a.value); ok {
						t.colors[c.name] = rgb
					}
				}
			}
		}
	}
	return t
}

// wordParseHex parses a six-digit hexadecimal colour.
func wordParseHex(s string) (wordRGB, bool) {
	if len(s) != 6 {
		return wordRGB{}, false
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return wordRGB{}, false
	}
	return wordRGB{uint8(n >> 16), uint8(n >> 8), uint8(n)}, true
}

// themeFont resolves a theme font reference such as "minorHAnsi" to a
// typeface. ok is false when the theme part is missing or the reference is not
// one; a theme that names no typeface for a script (which the East Asian and
// complex script slots usually do not) resolves to an empty typeface.
func (t *wordTheme) themeFont(ref string) (string, bool) {
	if t == nil {
		return "", false
	}
	var scheme int
	switch {
	case strings.HasPrefix(ref, "major"):
		scheme = 0
	case strings.HasPrefix(ref, "minor"):
		scheme = 1
	default:
		return "", false
	}
	var slot int
	switch strings.TrimPrefix(strings.TrimPrefix(ref, "major"), "minor") {
	case "Ascii", "HAnsi":
		slot = 0
	case "EastAsia":
		slot = 1
	case "Bidi":
		slot = 2
	default:
		return "", false
	}
	return t.fonts[scheme][slot], true
}

// wordClrMap is the settings part's colour mapping from the document's colour
// names (bg1, tx1...) to the theme's scheme slots.
var wordDefaultClrMap = map[string]string{
	"bg1": "lt1", "tx1": "dk1", "bg2": "lt2", "tx2": "dk2",
	"accent1": "accent1", "accent2": "accent2", "accent3": "accent3",
	"accent4": "accent4", "accent5": "accent5", "accent6": "accent6",
	"hlink": "hlink", "folHlink": "folHlink",
}

// wordThemeColorSlot maps a w:themeColor value to a document colour name.
var wordThemeColorSlot = map[string]string{
	"dark1": "tx1", "light1": "bg1", "dark2": "tx2", "light2": "bg2",
	"text1": "tx1", "background1": "bg1", "text2": "tx2", "background2": "bg2",
	"accent1": "accent1", "accent2": "accent2", "accent3": "accent3",
	"accent4": "accent4", "accent5": "accent5", "accent6": "accent6",
	"hyperlink": "hlink", "followedHyperlink": "folHlink",
}

// themeColor resolves a w:themeColor with optional tint and shade (two-digit
// hexadecimal, 00 to FF). The tint and shade scale the colour's HSL luminance:
// shade multiplies it by shade/255, tint moves it towards white by 1-tint/255.
// A missing theme or unknown name reports ok false.
func (t *wordTheme) themeColor(name, tint, shade string, clrMap map[string]string) (wordRGB, bool) {
	if t == nil {
		return wordRGB{}, false
	}
	slot, ok := wordThemeColorSlot[name]
	if !ok {
		return wordRGB{}, false
	}
	if clrMap == nil {
		clrMap = wordDefaultClrMap
	}
	target, ok := clrMap[slot]
	if !ok {
		target = wordDefaultClrMap[slot]
	}
	rgb, ok := t.colors[target]
	if !ok {
		return wordRGB{}, false
	}
	if shade != "" {
		n, err := strconv.ParseUint(shade, 16, 8)
		if err != nil {
			return wordRGB{}, false
		}
		h, s, l := wordRGBToHSL(rgb)
		rgb = wordHSLToRGB(h, s, l*float64(n)/255)
	}
	if tint != "" {
		n, err := strconv.ParseUint(tint, 16, 8)
		if err != nil {
			return wordRGB{}, false
		}
		h, s, l := wordRGBToHSL(rgb)
		f := float64(n) / 255
		rgb = wordHSLToRGB(h, s, l*f+(1-f))
	}
	return rgb, true
}

func wordRGBToHSL(c wordRGB) (h, s, l float64) {
	r, g, b := float64(c.r)/255, float64(c.g)/255, float64(c.b)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	d := mx - mn
	if d == 0 {
		return 0, 0, l
	}
	s = d / (1 - math.Abs(2*l-1))
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s, l
}

func wordHSLToRGB(h, s, l float64) wordRGB {
	l = math.Max(0, math.Min(1, l))
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	to := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return wordRGB{to(r), to(g), to(b)}
}

// wordHighlight maps w:highlight values to their fixed colours.
var wordHighlight = map[string]wordRGB{
	"yellow": {255, 255, 0}, "green": {0, 255, 0}, "cyan": {0, 255, 255},
	"magenta": {255, 0, 255}, "blue": {0, 0, 255}, "red": {255, 0, 0},
	"darkBlue": {0, 0, 128}, "darkCyan": {0, 128, 128}, "darkGreen": {0, 128, 0},
	"darkMagenta": {128, 0, 128}, "darkRed": {128, 0, 0}, "darkYellow": {128, 128, 0},
	"darkGray": {128, 128, 128}, "lightGray": {192, 192, 192}, "black": {0, 0, 0},
	"white": {255, 255, 255},
}
