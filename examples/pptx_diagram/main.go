// Example: Build a "diagram" deck — connected shapes, an edited master/layout,
// and speaker notes — then reopen it and read everything back.
//
// This program is a focused tour of the spine pptx package's structural
// features (as opposed to the visual-effects tour in examples/pptx_deck):
//
//   - connectors (Slide.AddConnector) whose endpoints are BOUND to two shapes,
//     so the line follows the shapes rather than sitting at fixed coordinates;
//   - slide-master and slide-layout editing (SlideMaster.BodyStyle text levels
//     and SlideLayout.SetBackgroundFill) through the typed template APIs;
//   - speaker notes (Slide.SetNotes), which create a notesSlide part on demand;
//   - reading SmartArt (Slide.SmartArt) — the read path is shown even though
//     creating SmartArt from scratch is not supported (see the note below).
//
// After saving it reopens the file and reads the connectors, notes, and layout
// background back out to prove the round trip, then prints a success summary.
//
// ── The shape-id / connector-binding lesson ─────────────────────────────────
//
// A connector binds each end to a shape by that shape's cNvPr id (an unsigned
// integer), NOT by a Go pointer. For a shape you create through this package
// that id is only assigned when the deck is first saved (sequentially, in the
// order shapes were added) — Shape.ID() reports 0 until then.
//
// Connector.Connect is tolerant of this: it remembers the *shape* and resolves
// its id at save time, so you can add two fresh shapes, Connect() them, and Save
// once — the ids materialize and the bindings land correctly. That single-phase
// form works and is fine.
//
// This example deliberately uses the more explicit TWO-PHASE form instead,
// because it makes the "ids materialize at save" rule impossible to get wrong
// and mirrors the pattern you must use for anything that needs a *known* id
// up front (e.g. animations, see examples/pptx_deck):
//
//	Phase 1: add the two named shapes and Save(). Ids are now on disk.
//	Phase 2: Open() the saved deck, find each shape by its stable name, read its
//	         now-known ID(), add the connector and Connect() it to those shapes,
//	         then Save() again.
//
// Names are the stable handle across the save/reopen boundary; ids are not
// stable until a save has happened.
//
// Run with:
//
//	go run ./examples/pptx_diagram            # writes to diagram.pptx
//	go run ./examples/pptx_diagram out.pptx   # writes to out.pptx
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

// Stable names we give the two boxes so we can find them again after the
// save/reopen round trip (ids are assigned at save time; names survive it).
const (
	sourceName = "Source Box"
	targetName = "Target Box"
	flowSlide  = "Flow"
)

func main() {
	// Resolve the output path. Like every other example, it defaults to the
	// working directory.
	outputPath := "diagram.pptx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
	}

	// Phase 1: build the deck (shapes, master/layout edits, notes) and save it.
	// Saving is what assigns the shape ids the connector will bind to.
	buildDeck(outputPath)

	// Phase 2: reopen, resolve the two boxes by name, and connect them.
	connectBoxes(outputPath)

	// Verify: reopen the finished deck and read the new features back out.
	verify(outputPath)

	fmt.Printf("\nDeck written to: %s\n", outputPath)
}

// buildDeck creates the master/layout edits, the flow slide with its two named
// boxes, and the speaker notes, then saves the presentation for the first time.
func buildDeck(outputPath string) {
	p := pptx.CreateWidescreen()
	p.Properties.Title = "Spine Diagram Tour"
	p.Properties.Creator = "Spine Library"
	p.Properties.Subject = "Connectors, master/layout editing, notes"

	// ── Master / layout editing ──────────────────────────────────────────────
	// A created deck comes with one master and a set of standard layouts. Both
	// are editable through typed APIs — no raw XML.
	master := p.SlideMasters()[0]

	// Edit a master text-style level: level 0 (a:lvl1pPr) of the body style.
	// Every slide that inherits the body placeholder picks this up. Edits mutate
	// the underlying a:lvlNpPr in place, so the other levels and any unmodeled
	// properties round-trip unchanged.
	body := master.BodyStyle()
	body.SetLevelFont(0, "Calibri")
	body.SetLevelColor(0, dml.NewRGB(0x1F, 0x29, 0x37).ToColor())

	// Give the Blank layout a solid background fill. A layout background reuses
	// the same dml.Fill values as a shape fill; slides on this layout inherit it
	// unless they set their own background.
	layout, err := master.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		log.Fatalf("created deck unexpectedly has no Blank layout: %v", err)
	}
	layout.SetName("Diagram Blank")
	layout.SetBackgroundFill(dml.NewSolidFill(dml.NewRGB(0xF3, 0xF4, 0xF6).ToColor()))

	// ── The flow slide and its two boxes ─────────────────────────────────────
	s := p.AddSlide()
	s.SetName(flowSlide)
	addHeading(s, "Pipeline Flow")

	// Two auto shapes we will later join with a connector. We name them so the
	// second phase can find them after their ids are assigned on save.
	source := pptx.NewAutoShape(pptx.PresetRoundRect)
	source.SetName(sourceName)
	source.SetPosition(dml.Inches(1.2), dml.Inches(3))
	source.SetSize(dml.Inches(3), dml.Inches(1.4))
	source.SetFill(dml.NewSolidFill(dml.NewRGB(0x25, 0x63, 0xEB).ToColor()))
	source.SetLine(dml.Line{Width: 0})
	labelShape(source, "Extract")
	mustAddShape(s, source)

	target := pptx.NewAutoShape(pptx.PresetRoundRect)
	target.SetName(targetName)
	target.SetPosition(dml.Inches(9), dml.Inches(3))
	target.SetSize(dml.Inches(3), dml.Inches(1.4))
	target.SetFill(dml.NewSolidFill(dml.NewRGB(0x05, 0x96, 0x69).ToColor()))
	target.SetLine(dml.Line{Width: 0})
	labelShape(target, "Load")
	mustAddShape(s, target)

	// ── Speaker notes ────────────────────────────────────────────────────────
	// SetNotes creates a notesSlide part (wired to the slide, and to the notes
	// master when the deck has one) if the slide has none yet. Paragraphs are
	// separated by "\n".
	s.SetNotes("Talk through the pipeline:\n" +
		"1. Extract pulls from the source system.\n" +
		"2. The connector shows data flowing to the load stage.")

	if err := p.Save(outputPath); err != nil {
		log.Fatalf("phase 1 save: %v", err)
	}
	fmt.Printf("Phase 1: built %d slide(s); master text style and layout background edited; notes set.\n",
		p.SlideCount())
}

// connectBoxes reopens the saved deck, resolves the two boxes by their stable
// names (their ids are now materialized), and adds a connector bound to them.
// This is the second half of the two-phase pattern from the package comment:
// binding an endpoint needs a shape whose id exists, which only happens after a
// save.
func connectBoxes(outputPath string) {
	p, err := pptx.Open(outputPath)
	if err != nil {
		log.Fatalf("phase 2 open: %v", err)
	}
	defer func() { _ = p.Close() }()

	slide := slideByName(p, flowSlide)
	if slide == nil {
		log.Fatalf("phase 2: could not find the %q slide", flowSlide)
	}

	source := shapeByName(slide, sourceName)
	target := shapeByName(slide, targetName)
	if source == nil || target == nil {
		log.Fatal("phase 2: could not resolve both boxes by name")
	}
	// The ids are now assigned (non-zero) because the deck was saved in phase 1.
	fmt.Printf("Phase 2: resolved %q -> id %d, %q -> id %d.\n",
		sourceName, source.ID(), targetName, target.ID())

	// Add an elbow connector and bind each end to a connection site on a box.
	// Connection-site indexes identify a shape's anchor points; for a rounded
	// rectangle they run around the perimeter. We route from the source's right
	// edge (site 3) to the target's left edge (site 1). Connect records the
	// shapes; the bindings are written on save against their now-known ids.
	conn := slide.AddConnector(pptx.ConnectorElbow)
	conn.SetName("Flow Connector")
	conn.Connect(source, 3, target, 1)
	conn.SetLineWidth(2.25)
	conn.SetLineColor(dml.NewRGB(0x37, 0x41, 0x51).ToColor())

	if err := p.Save(outputPath); err != nil {
		log.Fatalf("phase 2 save: %v", err)
	}
	fmt.Println("Phase 2: added an elbow connector bound to both boxes and re-saved.")
}

// verify reopens the finished deck and reads the round-tripped features back.
func verify(outputPath string) {
	p, err := pptx.Open(outputPath)
	if err != nil {
		log.Fatalf("verify open: %v", err)
	}
	defer func() { _ = p.Close() }()

	fmt.Println("\nVerification (read back from disk):")

	slide := slideByName(p, flowSlide)
	if slide == nil {
		log.Fatalf("verify: %q slide missing", flowSlide)
	}

	// Connectors and their resolved endpoint bindings.
	conns := slide.Connectors()
	fmt.Printf("  connectors: %d\n", len(conns))
	for _, c := range conns {
		startID, startIdx, startBound := c.StartConnection()
		endID, endIdx, endBound := c.EndConnection()
		fmt.Printf("    %-16q kind=%s start=(id %d, site %d, bound %v) end=(id %d, site %d, bound %v)\n",
			c.Name(), c.Kind(),
			startID, startIdx, startBound, endID, endIdx, endBound)
	}

	// Speaker notes (read from the notesSlide part).
	notes := slide.Notes()
	fmt.Printf("  notes (%d chars): %q\n", len(notes), firstLine(notes))

	// Layout background: find our edited Blank layout and read its fill back.
	master := p.SlideMasters()[0]
	if layout, err := master.LayoutByName("Diagram Blank"); err == nil {
		if c, ok := layout.BackgroundColor(); ok {
			fmt.Printf("  layout %-16q background = #%s\n", layout.Name(), c.RGB)
		} else {
			fmt.Printf("  layout %-16q has background: %v\n", layout.Name(), layout.HasBackground())
		}
	}

	// Master text-style level 0 of the body style, read back.
	if lvl := master.BodyStyle().Level(0); lvl != nil {
		fmt.Printf("  master body level 0: font=%q hasColor=%v\n", lvl.FontName, lvl.HasColor)
	}

	// ── Reading SmartArt ─────────────────────────────────────────────────────
	// Slide.SmartArt / Presentation.SmartArt return the SmartArt graphics found
	// on a slide, exposing each diagram's text hierarchy from its data part.
	//
	// Creating SmartArt from scratch is NOT supported (a valid diagram also
	// needs layout/quickStyle/colors definition parts and a drawing fallback
	// that PowerPoint rejects if malformed), so this created deck contains none
	// and the read below reports zero. The loop is the exact read path you would
	// use on a deck that DOES contain SmartArt (e.g. one opened from PowerPoint):
	// it would print each top-level node's text and child count.
	arts := p.SmartArt()
	fmt.Printf("  SmartArt graphics: %d (create is unsupported; this is the read path)\n", len(arts))
	for _, sa := range arts {
		for _, node := range sa.Nodes() {
			fmt.Printf("    node %q (%d child node(s))\n", node.Text, len(node.Children))
		}
	}

	fmt.Println("\nSuccess: connectors, notes, and layout background all round-tripped.")
}

// ── small helpers ───────────────────────────────────────────────────────────

// addHeading adds a standard slide title text box.
func addHeading(s *pptx.Slide, text string) {
	h := s.AddTextBox()
	h.SetPosition(dml.Inches(0.6), dml.Inches(0.4))
	h.SetSize(dml.Inches(12), dml.Inches(1))
	hp := h.TextFrame().AddParagraph()
	hr := hp.AddRun()
	hr.SetText(text)
	hr.SetFontSize(32)
	hr.SetBold(true)
	hr.SetColor(dml.NewRGB(0x11, 0x18, 0x27).ToColor())
}

// labelShape centers a white caption inside an auto shape.
func labelShape(shape *pptx.AutoShape, text string) {
	tf := shape.TextFrame()
	tf.SetAnchor(enum.TextAnchorMiddle)
	p := tf.AddParagraph()
	p.SetAlignment(enum.TextAlignCenter)
	r := p.AddRun()
	r.SetText(text)
	r.SetFontSize(20)
	r.SetBold(true)
	r.SetColor(dml.ColorWhite)
}

// mustAddShape adds a shape and fails loudly on error.
func mustAddShape(s *pptx.Slide, shape pptx.Shape) {
	if err := s.AddShape(shape); err != nil {
		log.Fatalf("add shape: %v", err)
	}
}

// slideByName returns the first slide with the given name, or nil.
func slideByName(p *pptx.Presentation, name string) *pptx.Slide {
	for _, s := range p.Slides() {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

// shapeByName returns the first shape on a slide with the given name, or nil.
// The Shape interface exposes ID(), which is what the connector binds against.
func shapeByName(s *pptx.Slide, name string) pptx.Shape {
	for _, sh := range s.Shapes() {
		if sh.Name() == name {
			return sh
		}
	}
	return nil
}

// firstLine returns everything up to the first newline (for compact printing).
func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i] + " …"
		}
	}
	return s
}
