package main

// JSON output envelopes. Every stdout object is one of these structs — never a
// map — so field order is stable and output is deterministic.

// sheetInfo is one xlsx sheet as reported by info and ls sheets.
type sheetInfo struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// infoXLSX is the info output for a workbook.
type infoXLSX struct {
	Kind       string      `json:"kind"`
	SheetCount int         `json:"sheet_count"`
	Sheets     []sheetInfo `json:"sheets"`
	Title      string      `json:"title,omitempty"`
	Creator    string      `json:"creator,omitempty"`
}

// slideInfo is one slide as reported by info.
type slideInfo struct {
	Index      int    `json:"index"`
	Name       string `json:"name,omitempty"`
	ShapeCount int    `json:"shape_count"`
}

// infoPPTX is the info output for a presentation.
type infoPPTX struct {
	Kind       string      `json:"kind"`
	SlideCount int         `json:"slide_count"`
	Slides     []slideInfo `json:"slides"`
	Title      string      `json:"title,omitempty"`
	Creator    string      `json:"creator,omitempty"`
}

// slideEntry is one slide as reported by ls slides.
type slideEntry struct {
	Index       int    `json:"index"`
	Name        string `json:"name,omitempty"`
	ShapeCount  int    `json:"shape_count"`
	TextPreview string `json:"text_preview"`
}

// placeholderInfo is reported for placeholder shapes in ls slide/<N>.
type placeholderInfo struct {
	Type string `json:"type"`
	Idx  uint32 `json:"idx"`
}

// shapeEntry is one top-level shape as reported by ls slide/<N>.
type shapeEntry struct {
	ID          uint32           `json:"id"`
	Name        string           `json:"name,omitempty"`
	Type        string           `json:"type"`
	Placeholder *placeholderInfo `json:"placeholder,omitempty"`
	TextPreview string           `json:"text_preview"`
}

// cellEntry is one cell as reported by cells. Value is typed per the spec:
// string, number, bool, string (error), RFC 3339 string (date), or null.
type cellEntry struct {
	Ref     string `json:"ref"`
	Type    string `json:"type"`
	Value   any    `json:"value"`
	Formula string `json:"formula,omitempty"`
}

// cellsOutput is the cells command's envelope.
type cellsOutput struct {
	Sheet string      `json:"sheet"`
	Range string      `json:"range"`
	Cells []cellEntry `json:"cells"`
}

// setXLSXOutput is the set output for an xlsx cell.
type setXLSXOutput struct {
	Selector string `json:"selector"`
	Type     string `json:"type"`
	Value    any    `json:"value"`
}

// setPPTXOutput is the set output for pptx text.
type setPPTXOutput struct {
	Selector string `json:"selector"`
	ShapeID  uint32 `json:"shape_id"`
	Text     string `json:"text"`
}

// textOutput is the text command's --json envelope.
type textOutput struct {
	Text string `json:"text"`
}
