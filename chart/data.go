package chart

import "fmt"

// embedBuilder builds a chart's embedded .xlsx workbook. It is registered by the
// xlsx package from an init (see xlsx.init -> RegisterEmbedder) to avoid a
// chart -> xlsx import cycle: the xlsx integration imports chart for its
// AddChart/Charts methods, so chart must not import xlsx. Building an .xlsx
// inherently needs the xlsx package, so any program that embeds a chart
// workbook already links it and the builder is installed.
var embedBuilder func(*Chart) ([]byte, DataLayout, error)

// RegisterEmbedder installs the embedded-workbook builder. The xlsx package
// calls it from an init; application code does not use it directly.
func RegisterEmbedder(fn func(*Chart) ([]byte, DataLayout, error)) {
	embedBuilder = fn
}

// EmbeddedWorkbook builds a minimal .xlsx workbook (as bytes) holding the
// chart's data laid out to match its c:f references, and returns it together
// with the DataLayout describing the cell ranges. This is what docx and pptx
// charts embed so Office can edit the data; xlsx charts reference the host
// sheet directly and do not need it.
//
// The layout places categories (or scatter X) in column A and each series in a
// subsequent column, with the series name in row 1 and its values in rows
// 2..N+1 — the same convention MarshalChartXML builds its references from, so
// the returned layout's references equal those in the emitted chart.xml.
//
// The builder lives in the xlsx package and is installed via RegisterEmbedder;
// EmbeddedWorkbook returns an error if the xlsx package has not been linked.
func (c *Chart) EmbeddedWorkbook() ([]byte, DataLayout, error) {
	if embedBuilder == nil {
		return nil, c.Layout(), fmt.Errorf("chart: EmbeddedWorkbook requires the xlsx package to be imported")
	}
	return embedBuilder(c)
}
