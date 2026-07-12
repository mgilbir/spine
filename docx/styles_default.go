package docx

import (
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// defaultStyles builds the minimal styles part written for documents created
// with Create(): document defaults (Calibri 11pt) plus the Normal style and
// Heading1-9 that AddHeading references. Without it, a created document
// referenced "Heading1" etc. with no styles.xml in the package, so headings
// rendered as plain text. Modeled on Word's defaults, compact.
func defaultStyles() *oxml.CT_Styles {
	styles := &oxml.CT_Styles{
		DocDefaults: &oxml.CT_DocDefaults{
			RPrDefault: &oxml.CT_RPrDefault{
				RPr: &oxml.CT_RPr{
					RFonts: &oxml.CT_Fonts{Ascii: "Calibri", HAnsi: "Calibri", EastAsia: "Calibri", Cs: "Calibri"},
					Sz:     &oxml.CT_HpsMeasure{Val: "22"},
					SzCs:   &oxml.CT_HpsMeasure{Val: "22"},
				},
			},
			PPrDefault: &oxml.CT_PPrDefault{},
		},
	}

	styles.Style = append(styles.Style, &oxml.CT_Style{
		Type:    "paragraph",
		StyleId: "Normal",
		Default: "1",
		Name:    &oxml.CT_String{Val: "Normal"},
		QFormat: &oxml.CT_OnOff{},
	})

	// Heading sizes in half-points: 16pt, 13pt, 12pt, then 11pt (body size).
	sizes := [9]string{"32", "26", "24", "22", "22", "22", "22", "22", "22"}
	for level := 1; level <= 9; level++ {
		n := strconv.Itoa(level)
		sz := sizes[level-1]
		styles.Style = append(styles.Style, &oxml.CT_Style{
			Type:    "paragraph",
			StyleId: "Heading" + n,
			Name:    &oxml.CT_String{Val: "heading " + n},
			BasedOn: &oxml.CT_String{Val: "Normal"},
			Next:    &oxml.CT_String{Val: "Normal"},
			QFormat: &oxml.CT_OnOff{},
			PPr: &oxml.CT_PPr{
				KeepNext:   &oxml.CT_OnOff{},
				KeepLines:  &oxml.CT_OnOff{},
				Spacing:    &oxml.CT_Spacing{Before: "240"},
				OutlineLvl: &oxml.CT_DecimalNumber{Val: level - 1},
			},
			RPr: &oxml.CT_RPr{
				B:    &oxml.CT_OnOff{},
				BCs:  &oxml.CT_OnOff{},
				Sz:   &oxml.CT_HpsMeasure{Val: sz},
				SzCs: &oxml.CT_HpsMeasure{Val: sz},
			},
		})
	}
	return styles
}
