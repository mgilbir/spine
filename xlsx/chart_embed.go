package xlsx

import "github.com/mgilbir/spine/chart"

// init installs the embedded-workbook builder on the chart package. The chart
// package cannot import xlsx (xlsx imports chart for AddChart/Charts), so
// chart.EmbeddedWorkbook delegates here through a registration hook.
func init() {
	chart.RegisterEmbedder(buildEmbeddedWorkbook)
}

// buildEmbeddedWorkbook builds the minimal .xlsx workbook a docx/pptx chart
// embeds: one worksheet (named by the chart's DataRef, default "Sheet1")
// holding the chart's data in the fixed layout its c:f references point at. It
// backs chart.(*Chart).EmbeddedWorkbook.
func buildEmbeddedWorkbook(c *chart.Chart) ([]byte, chart.DataLayout, error) {
	dl := c.Layout()
	wb := Create()
	sheet := wb.AddSheet(dl.Sheet)
	if err := writeChartData(sheet, c); err != nil {
		return nil, dl, err
	}
	data, err := wb.SaveBytes()
	if err != nil {
		return nil, dl, err
	}
	return data, dl, nil
}
