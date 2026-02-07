// Example: Create a simple Word document
//
// This example demonstrates how to use the spine library to create
// a basic Word document with paragraphs, headings, formatted text,
// and a table.
//
// Run with: go run ./examples/create_document
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/docx"
)

func main() {
	// Create a new document
	doc := docx.Create()

	// Set document properties
	doc.Properties.Title = "Spine Demo Document"
	doc.Properties.Creator = "Spine Library"
	doc.Properties.Subject = "Demonstration of the spine DOCX library"

	// Add a heading
	doc.AddHeading("Welcome to Spine", 1)

	// Add a paragraph
	doc.AddParagraphWithText("Spine is a Go library for reading and writing Microsoft Office documents. It supports PowerPoint (.pptx), Word (.docx), and Excel (.xlsx) formats.")

	// Add a paragraph with formatted text
	doc.AddHeading("Features", 2)

	features := []string{
		"Create Word documents programmatically",
		"Read and modify existing DOCX files",
		"Support for paragraphs, runs, and tables",
		"Byte-identical round-trip preservation",
		"Pure Go implementation with no external dependencies",
	}

	for _, feature := range features {
		p := doc.AddParagraph()
		run := p.AddRun()
		run.SetText("- " + feature)
	}

	// Add a paragraph with rich formatting
	doc.AddHeading("Formatting Example", 2)

	p := doc.AddParagraph()

	boldRun := p.AddRun()
	boldRun.SetText("Bold text")
	boldRun.SetBold(true)

	sepRun := p.AddRun()
	sepRun.SetText(", ")

	italicRun := p.AddRun()
	italicRun.SetText("italic text")
	italicRun.SetItalic(true)

	sepRun2 := p.AddRun()
	sepRun2.SetText(", ")

	colorRun := p.AddRun()
	colorRun.SetText("colored text")
	colorRun.SetColor("FF0000")

	sepRun3 := p.AddRun()
	sepRun3.SetText(", and ")

	sizedRun := p.AddRun()
	sizedRun.SetText("large text")
	sizedRun.SetFontSize(18)
	sizedRun.SetFont("Arial")

	sepRun4 := p.AddRun()
	sepRun4.SetText(".")

	// Add centered paragraph
	centered := doc.AddParagraph()
	centered.SetAlignment(docx.AlignmentCenter)
	cRun := centered.AddRun()
	cRun.SetText("This paragraph is centered.")

	// Save the document
	outputPath := "output.docx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}

	// Ensure output directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}

	err := doc.Save(outputPath)
	if err != nil {
		log.Fatalf("Failed to save document: %v", err)
	}

	fmt.Printf("Document saved to: %s\n", outputPath)
	fmt.Printf("Paragraphs: %d\n", len(doc.Paragraphs()))
	fmt.Printf("Title: %s\n", doc.Properties.Title)
}
