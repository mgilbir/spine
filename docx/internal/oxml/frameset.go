package oxml

// CT_Frameset represents a frameset definition (w:frameset).
type CT_Frameset struct {
	Sz           *CT_String      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,omitempty"`
	FramesetSplitbar *CT_FramesetSplitbar `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main framesetSplitbar,omitempty"`
	FrameLayout  *CT_String      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main frameLayout,omitempty"`
	Title        *CT_String      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main title,omitempty"`
	Frameset     []*CT_Frameset  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main frameset,omitempty"`
	Frame        []*CT_Frame     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main frame,omitempty"`
}

// CT_Frame represents a single frame in a frameset (w:frame).
type CT_Frame struct {
	Sz            *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,omitempty"`
	Name          *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	Title         *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main title,omitempty"`
	LongDesc      *CT_Rel     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main longDesc,omitempty"`
	SourceFileName *CT_Rel    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sourceFileName,omitempty"`
	MarW          *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marW,omitempty"`
	MarH          *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main marH,omitempty"`
	Scrollbar     *CT_String  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main scrollbar,omitempty"`
	NoResizeAllowed *CT_OnOff `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noResizeAllowed,omitempty"`
	LinkedToFile  *CT_OnOff   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main linkedToFile,omitempty"`
}

// CT_FramesetSplitbar represents frameset splitter bar properties.
type CT_FramesetSplitbar struct {
	W       *CT_TwipsMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,omitempty"`
	Color   *CT_Color        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,omitempty"`
	NoBorder *CT_OnOff       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noBorder,omitempty"`
	FlatBorders *CT_OnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main flatBorders,omitempty"`
}
