package docx

import (
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// ListStyle represents a list definition that can be applied to paragraphs.
type ListStyle struct {
	document *Document
	numID    int // the CT_Num.NumId value
}

// AddBulletList creates a new bullet list definition and returns a ListStyle
// that can be applied to paragraphs.
func (d *Document) AddBulletList() *ListStyle {
	d.ensureNumbering()
	absID := d.nextAbstractNumID()
	numID := d.nextNumID()

	bulletChars := []string{"\uF0B7", "o", "\uF0A7"} // disc, circle, diamond
	bulletFonts := []string{"Symbol", "Courier New", "Wingdings"}

	abstract := &oxml.CT_AbstractNum{
		AbstractNumId:  strconv.Itoa(absID),
		MultiLevelType: &oxml.CT_String{Val: "hybridMultilevel"},
	}

	for i := 0; i < 9; i++ {
		idx := i % len(bulletChars)
		indent := 720 * (i + 1) // 720 twips = 0.5 inch
		hanging := 360          // 360 twips = 0.25 inch

		lvl := &oxml.CT_Lvl{
			Ilvl:    strconv.Itoa(i),
			Start:   &oxml.CT_DecimalNumber{Val: 1},
			NumFmt:  &oxml.CT_NumFmt{Val: "bullet"},
			LvlText: &oxml.CT_LvlText{Val: bulletChars[idx]},
			LvlJc:   &oxml.CT_Jc{Val: "left"},
			PPr: &oxml.CT_PPr{
				Ind: &oxml.CT_Ind{
					Left:    strconv.Itoa(indent),
					Hanging: strconv.Itoa(hanging),
				},
			},
			RPr: &oxml.CT_RPr{
				RFonts: &oxml.CT_Fonts{
					Ascii: bulletFonts[idx],
					HAnsi: bulletFonts[idx],
				},
			},
		}
		abstract.Lvl = append(abstract.Lvl, lvl)
	}

	d.numbering.AbstractNum = append(d.numbering.AbstractNum, abstract)
	d.numbering.Num = append(d.numbering.Num, &oxml.CT_Num{
		NumId:         strconv.Itoa(numID),
		AbstractNumId: &oxml.CT_DecimalNumber{Val: absID},
	})
	d.numberingModified = true

	return &ListStyle{document: d, numID: numID}
}

// AddNumberedList creates a new numbered list definition and returns a ListStyle
// that can be applied to paragraphs.
func (d *Document) AddNumberedList() *ListStyle {
	d.ensureNumbering()
	absID := d.nextAbstractNumID()
	numID := d.nextNumID()

	// Numbering formats cycle: decimal, lowerLetter, lowerRoman
	numFmts := []string{"decimal", "lowerLetter", "lowerRoman"}

	abstract := &oxml.CT_AbstractNum{
		AbstractNumId:  strconv.Itoa(absID),
		MultiLevelType: &oxml.CT_String{Val: "hybridMultilevel"},
	}

	for i := 0; i < 9; i++ {
		idx := i % len(numFmts)
		indent := 720 * (i + 1)
		hanging := 360

		// Build level text: "%1.", "%2.", etc.
		lvlText := fmt.Sprintf("%%%d.", i+1)
		switch numFmts[idx] {
		case "lowerLetter":
			lvlText = fmt.Sprintf("%%%d)", i+1)
		case "lowerRoman":
			lvlText = fmt.Sprintf("%%%d.", i+1)
		}

		lvl := &oxml.CT_Lvl{
			Ilvl:    strconv.Itoa(i),
			Start:   &oxml.CT_DecimalNumber{Val: 1},
			NumFmt:  &oxml.CT_NumFmt{Val: numFmts[idx]},
			LvlText: &oxml.CT_LvlText{Val: lvlText},
			LvlJc:   &oxml.CT_Jc{Val: "left"},
			PPr: &oxml.CT_PPr{
				Ind: &oxml.CT_Ind{
					Left:    strconv.Itoa(indent),
					Hanging: strconv.Itoa(hanging),
				},
			},
		}
		abstract.Lvl = append(abstract.Lvl, lvl)
	}

	d.numbering.AbstractNum = append(d.numbering.AbstractNum, abstract)
	d.numbering.Num = append(d.numbering.Num, &oxml.CT_Num{
		NumId:         strconv.Itoa(numID),
		AbstractNumId: &oxml.CT_DecimalNumber{Val: absID},
	})
	d.numberingModified = true

	return &ListStyle{document: d, numID: numID}
}

// SetListStyle applies a list style to the paragraph at the given level (0-based).
func (p *Paragraph) SetListStyle(list *ListStyle, level int) {
	p.ensurePPr()
	p.p.PPr.NumPr = &oxml.CT_NumPr{
		NumId: &oxml.CT_DecimalNumber{Val: list.numID},
		Ilvl:  &oxml.CT_DecimalNumber{Val: level},
	}
}

// RemoveListStyle removes any list style from the paragraph.
func (p *Paragraph) RemoveListStyle() {
	if p.p.PPr != nil {
		p.p.PPr.NumPr = nil
	}
}

// ensureNumbering ensures the numbering definitions part exists on the document.
func (d *Document) ensureNumbering() {
	if d.numbering == nil {
		d.numbering = &oxml.CT_Numbering{}
	}
}

// nextAbstractNumID allocates an abstract numbering ID past both the
// definitions parsed from an opened package (kept raw, IDs surfaced in
// ParsedAbstractNumIDs) and the ones added in this session.
func (d *Document) nextAbstractNumID() int {
	max := -1
	if d.numbering != nil {
		for _, an := range d.numbering.AbstractNum {
			id, err := strconv.Atoi(an.AbstractNumId)
			if err == nil && id > max {
				max = id
			}
		}
		for _, raw := range d.numbering.ParsedAbstractNumIDs {
			id, err := strconv.Atoi(raw)
			if err == nil && id > max {
				max = id
			}
		}
	}
	return max + 1
}

// nextNumID allocates a numbering-instance ID (see nextAbstractNumID).
func (d *Document) nextNumID() int {
	max := 0
	if d.numbering != nil {
		for _, n := range d.numbering.Num {
			id, err := strconv.Atoi(n.NumId)
			if err == nil && id > max {
				max = id
			}
		}
		for _, raw := range d.numbering.ParsedNumIDs {
			id, err := strconv.Atoi(raw)
			if err == nil && id > max {
				max = id
			}
		}
	}
	return max + 1
}
