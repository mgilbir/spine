// Example: Create a PowerPoint presentation with transitions and shapes
//
// This example demonstrates slide transitions, auto shapes with fills,
// lines, and shadows, gradient fills, and color helpers using the spine
// PPTX library.
//
// Run with: go run ./examples/create_presentation
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/pptx"
)

func main() {
	p := pptx.CreateWidescreen()

	p.Properties.Title = "Spine Demo Presentation"
	p.Properties.Creator = "Spine Library"
	p.Properties.Subject = "Demonstration of the spine PPTX library"

	// ── Slide 1: Title ───────────────────────────────────────────────

	slide1 := p.AddSlide()
	slide1.SetName("Title Slide")
	slide1.SetTransition(pptx.Transition{
		Type:           pptx.TransitionFade,
		Duration:       0.5,
		AdvanceOnClick: true,
		AdvanceAfter:   5.0,
	})

	// Background accent shape with gradient fill
	bgShape := pptx.NewAutoShape(pptx.PresetRect)
	bgShape.SetPosition(0, 0)
	bgShape.SetSize(dml.Inches(13.33), dml.Inches(7.5))
	bgShape.SetFill(dml.NewGradientFill(270,
		dml.GradientStop{Position: 0, Color: dml.NewRGB(31, 78, 121).ToColor()},
		dml.GradientStop{Position: 1, Color: dml.NewRGB(68, 114, 196).ToColor()},
	))
	bgShape.SetLine(dml.Line{Width: 0}) // no outline
	if err := slide1.AddShape(bgShape); err != nil {
		log.Fatalf("Failed to add shape: %v", err)
	}

	// Title text
	titleBox := slide1.AddTextBox()
	titleBox.SetPosition(dml.Inches(0.5), dml.Inches(2))
	titleBox.SetSize(dml.Inches(12.33), dml.Inches(1.5))
	tf := titleBox.TextFrame()
	titlePara := tf.AddParagraph()
	titlePara.SetAlignment(enum.TextAlignCenter)
	titleRun := titlePara.AddRun()
	titleRun.SetText("Welcome to Spine")
	titleRun.SetFontSize(44)
	titleRun.SetBold(true)
	titleRun.SetColor(dml.ColorWhite)

	// Subtitle
	subtitleBox := slide1.AddTextBox()
	subtitleBox.SetPosition(dml.Inches(0.5), dml.Inches(4))
	subtitleBox.SetSize(dml.Inches(12.33), dml.Inches(1))
	stf := subtitleBox.TextFrame()
	subPara := stf.AddParagraph()
	subPara.SetAlignment(enum.TextAlignCenter)
	subRun := subPara.AddRun()
	subRun.SetText("A Go library for creating Office documents")
	subRun.SetFontSize(24)
	subRun.SetColor(dml.NewRGB(200, 220, 240).ToColor())

	// ── Slide 2: Features ────────────────────────────────────────────

	slide2 := p.AddSlide()
	slide2.SetName("Features")
	slide2.SetTransition(pptx.Transition{
		Type:           pptx.TransitionPush,
		Duration:       0.5,
		AdvanceOnClick: true,
	})

	// Title
	titleBox2 := slide2.AddTextBox()
	titleBox2.SetPosition(dml.Inches(0.5), dml.Inches(0.5))
	titleBox2.SetSize(dml.Inches(12.33), dml.Inches(1))
	titleBox2.SetText("Features")

	// Feature list
	contentBox := slide2.AddTextBox()
	contentBox.SetPosition(dml.Inches(0.5), dml.Inches(2))
	contentBox.SetSize(dml.Inches(7), dml.Inches(4))

	ctf := contentBox.TextFrame()
	features := []string{
		"Create presentations, documents, and spreadsheets",
		"Read and modify existing Office files",
		"Byte-identical round-trip preservation",
		"Based on the Open Packaging Conventions standard",
		"Pure Go with no external dependencies",
	}
	for i, feature := range features {
		para := ctf.AddParagraph()
		para.SetLevel(0)
		run := para.AddRun()
		run.SetText(feature)
		if i == 0 {
			run.SetBold(true)
		}
	}

	// Decorative shape with solid fill, line, and shadow
	accentShape := pptx.NewAutoShape(pptx.PresetRoundRect)
	accentShape.SetName("Accent Shape")
	accentShape.SetPosition(dml.Inches(8.5), dml.Inches(2.5))
	accentShape.SetSize(dml.Inches(4), dml.Inches(2.5))
	accentShape.SetFill(dml.NewSolidFill(
		dml.NewRGB(68, 114, 196).ToColor(),
	))
	accentShape.SetLine(dml.Line{
		Width: 1.5,
		Color: dml.NewRGB(31, 78, 121).ToColor(),
		Dash:  dml.DashSolid,
	})
	accentShape.SetShadow(dml.Shadow{
		Color:    dml.NewRGB(0, 0, 0).ToColor().WithAlpha(40),
		BlurRad:  6,
		Distance: 3,
		Angle:    315,
	})
	// Add text to the shape
	stf2 := accentShape.TextFrame()
	stf2.SetAnchor(enum.TextAnchorMiddle)
	shapePara := stf2.AddParagraph()
	shapePara.SetAlignment(enum.TextAlignCenter)
	shapeRun := shapePara.AddRun()
	shapeRun.SetText("Styled Shape")
	shapeRun.SetFontSize(20)
	shapeRun.SetBold(true)
	shapeRun.SetColor(dml.ColorWhite)
	if err := slide2.AddShape(accentShape); err != nil {
		log.Fatalf("Failed to add shape: %v", err)
	}

	// ── Slide 3: Comparison Table ────────────────────────────────────

	slide3 := p.AddSlide()
	slide3.SetName("Comparison")
	slide3.SetTransition(pptx.Transition{
		Type:           pptx.TransitionWipe,
		Duration:       0.8,
		AdvanceOnClick: true,
	})

	titleBox3 := slide3.AddTextBox()
	titleBox3.SetPosition(dml.Inches(0.5), dml.Inches(0.5))
	titleBox3.SetSize(dml.Inches(12.33), dml.Inches(1))
	titleBox3.SetText("Format Comparison")

	table := slide3.AddTable(4, 3)
	table.SetPosition(dml.Inches(1), dml.Inches(2))
	table.SetSize(dml.Inches(11), dml.Inches(3))

	// Headers
	table.Cell(0, 0).SetText("Format")
	table.Cell(0, 1).SetText("Extension")
	table.Cell(0, 2).SetText("Status")

	tableData := [][]string{
		{"PowerPoint", ".pptx", "Full support"},
		{"Word", ".docx", "Full support"},
		{"Excel", ".xlsx", "Full support"},
	}
	for i, row := range tableData {
		for j, cell := range row {
			table.Cell(i+1, j).SetText(cell)
		}
	}

	// ── Save ─────────────────────────────────────────────────────────

	outputPath := "output.pptx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}

	if err := p.Save(outputPath); err != nil {
		log.Fatalf("Failed to save presentation: %v", err)
	}

	fmt.Printf("Presentation saved to: %s\n", outputPath)
	fmt.Printf("Slides: %d\n", p.SlideCount())
}
