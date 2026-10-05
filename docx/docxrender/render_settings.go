package docxrender

import (
	"fmt"

	"github.com/mgilbir/spine/render"
)

// wordClrSlot maps a w:clrSchemeMapping value to the theme colour scheme slot.
var wordClrSlot = map[string]string{
	"light1": "lt1", "dark1": "dk1", "light2": "lt2", "dark2": "dk2",
	"accent1": "accent1", "accent2": "accent2", "accent3": "accent3",
	"accent4": "accent4", "accent5": "accent5", "accent6": "accent6",
	"hyperlink": "hlink", "followedHyperlink": "folHlink",
}

// wordClrNames are the w:clrSchemeMapping attributes, in document order.
var wordClrNames = []string{"bg1", "tx1", "bg2", "tx2", "accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hyperlink", "followedHyperlink"}

// clrMap is the document's colour mapping, or nil for the default one.
func (r *wordRenderer) clrMap() map[string]string { return r.colorMap }

// wordSettingsLayout lists the settings that change how a page is laid out and
// that the profile does not model: the page is drawn without them.
var wordSettingsLayout = map[string]string{
	"mirrorMargins":           "mirrored margins",
	"gutterAtTop":             "gutter at top",
	"bookFoldPrinting":        "book fold printing",
	"bookFoldRevPrinting":     "book fold printing",
	"autoHyphenation":         "automatic hyphenation",
	"noPunctuationKerning":    "punctuation kerning",
	"printTwoOnOne":           "two pages per sheet",
	"displayBackgroundShape":  "page background",
	"alignBordersAndEdges":    "page border alignment",
	"doNotHyphenateCaps":      "",
	"consecutiveHyphenLimit":  "",
	"hyphenationZone":         "",
	"strictFirstAndLastChars": "",
}

// loadSettings reads the settings part. A nil root is a document without one.
func (r *wordRenderer) loadSettings(root *wordNode) error {
	r.defaultTab = 720.0 / 15
	r.compat = 12
	if root == nil {
		return nil
	}
	for _, c := range root.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "defaultTabStop":
			v, err := wordLength(c.val(), "w:defaultTabStop")
			if err != nil {
				return err
			}
			if v <= 0 {
				return fmt.Errorf("%w: w:defaultTabStop", render.ErrInvalid)
			}
			r.defaultTab = v
		case "footnotePr", "endnotePr":
			var err error
			if c.name == "endnotePr" {
				r.notes.end, err = parseNotePr(c, r.notes.end, true)
			} else {
				r.notes.foot, err = parseNotePr(c, r.notes.foot, false)
			}
			if err != nil {
				return err
			}
		case "evenAndOddHeaders":
			on, valid := c.on()
			if !valid {
				return fmt.Errorf("%w: w:evenAndOddHeaders", render.ErrInvalid)
			}
			r.evenOdd = on
		case "compat":
			for _, s := range c.children {
				if s.is("compatSetting") && s.attrOr("name") == "compatibilityMode" {
					v, ok := wordRenderInt(s.attrOr("val"))
					if !ok || v < 0 || v > 100 {
						return fmt.Errorf("%w: w:compatibilityMode", render.ErrInvalid)
					}
					r.compat = v
				}
			}
		case "clrSchemeMapping":
			m := map[string]string{}
			for _, name := range wordClrNames {
				v, ok := c.attr(name)
				if !ok {
					continue
				}
				slot, known := wordClrSlot[v]
				if !known {
					return fmt.Errorf("%w: w:clrSchemeMapping", render.ErrInvalid)
				}
				key := name
				switch name {
				case "hyperlink":
					key = "hlink"
				case "followedHyperlink":
					key = "folHlink"
				}
				m[key] = slot
			}
			r.colorMap = m
		default:
			what, ok := wordSettingsLayout[c.name]
			if !ok {
				break
			}
			on := true
			if c.name != "hyphenationZone" && c.name != "consecutiveHyphenLimit" {
				var valid bool
				if on, valid = c.on(); !valid {
					return fmt.Errorf("%w: w:%s", render.ErrInvalid, c.name)
				}
			}
			if on && what != "" {
				if err := r.approximate("setting " + what); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
