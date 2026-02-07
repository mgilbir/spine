// Example: Create a simple PowerPoint presentation
//
// This example demonstrates how to use the spine library to create
// a basic PowerPoint presentation with slides and text.
//
// Run with: go run ./examples/create_presentation
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
)

func main() {
	// Create a new presentation
	p := pptx.CreateWidescreen()

	// Set document properties
	p.Properties.Title = "Spine Demo Presentation"
	p.Properties.Creator = "Spine Library"
	p.Properties.Subject = "Demonstration of the spine PPTX library"
	p.Properties.Keywords = "spine, pptx, golang, presentation"

	// Add first slide (title slide)
	slide1 := p.AddSlide()
	slide1.SetName("Title Slide")

	// Add a text box for the title
	titleBox := slide1.AddTextBox()
	titleBox.SetPosition(dml.Inches(0.5), dml.Inches(2))
	titleBox.SetSize(dml.Inches(12.33), dml.Inches(1.5))
	titleBox.SetText("Welcome to Spine")

	// Add a text box for the subtitle
	subtitleBox := slide1.AddTextBox()
	subtitleBox.SetPosition(dml.Inches(0.5), dml.Inches(4))
	subtitleBox.SetSize(dml.Inches(12.33), dml.Inches(1))
	subtitleBox.SetText("A Go library for creating Office documents")

	// Add second slide (content slide)
	slide2 := p.AddSlide()
	slide2.SetName("Features")

	// Add title
	titleBox2 := slide2.AddTextBox()
	titleBox2.SetPosition(dml.Inches(0.5), dml.Inches(0.5))
	titleBox2.SetSize(dml.Inches(12.33), dml.Inches(1))
	titleBox2.SetText("Features")

	// Add content
	contentBox := slide2.AddTextBox()
	contentBox.SetPosition(dml.Inches(0.5), dml.Inches(2))
	contentBox.SetSize(dml.Inches(12.33), dml.Inches(4))

	tf := contentBox.TextFrame()
	features := []string{
		"Create PowerPoint presentations programmatically",
		"Read and modify existing PPTX files",
		"Support for text, shapes, and tables",
		"Based on the Open Packaging Conventions (OPC) standard",
		"Pure Go implementation with no external dependencies",
	}

	for i, feature := range features {
		para := tf.AddParagraph()
		para.SetLevel(0)
		run := para.AddRun()
		run.SetText("• " + feature)
		if i == 0 {
			run.SetBold(true)
		}
	}

	// Add third slide with a table
	slide3 := p.AddSlide()
	slide3.SetName("Comparison")

	titleBox3 := slide3.AddTextBox()
	titleBox3.SetPosition(dml.Inches(0.5), dml.Inches(0.5))
	titleBox3.SetSize(dml.Inches(12.33), dml.Inches(1))
	titleBox3.SetText("Format Comparison")

	// Add a table
	table := slide3.AddTable(4, 3)
	table.SetPosition(dml.Inches(1), dml.Inches(2))
	table.SetSize(dml.Inches(11), dml.Inches(3))

	// Set headers
	table.Cell(0, 0).SetText("Format")
	table.Cell(0, 1).SetText("Extension")
	table.Cell(0, 2).SetText("Status")

	// Set data rows
	tableData := [][]string{
		{"PowerPoint", ".pptx", "Implemented"},
		{"Excel", ".xlsx", "Placeholder"},
		{"Word", ".docx", "Placeholder"},
	}

	for i, row := range tableData {
		for j, cell := range row {
			table.Cell(i+1, j).SetText(cell)
		}
	}

	// Save the presentation
	outputPath := "output.pptx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	err := p.Save(outputPath)
	if err != nil {
		log.Fatalf("Failed to save presentation: %v", err)
	}

	fmt.Printf("Presentation saved to: %s\n", outputPath)
	fmt.Printf("Slides: %d\n", p.SlideCount())
	fmt.Printf("Title: %s\n", p.Properties.Title)
}
