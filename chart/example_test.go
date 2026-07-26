package chart_test

import (
	"fmt"

	"github.com/mgilbir/spine/chart"
)

// ExampleNewColumn builds a column chart, serializes it to a DrawingML
// chart.xml with MarshalChartXML, then reads it back with Parse and prints the
// structural facts recovered from the part.
func ExampleNewColumn() {
	c := chart.NewColumn().
		SetTitle("Quarterly Sales").
		SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
	c.AddSeries("North", []float64{10, 20, 30, 40})
	c.AddSeries("South", []float64{5, 15, 25, 35})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		panic(err)
	}

	parsed, err := chart.Parse(xmlBytes)
	if err != nil {
		panic(err)
	}

	fmt.Printf("title=%q categories=%v\n", parsed.Title(), parsed.Categories())
	// Output: title="Quarterly Sales" categories=[Q1 Q2 Q3 Q4]
}
