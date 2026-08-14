// Example: Build a rich multi-slide PowerPoint deck, then read it back.
//
// This program is a guided tour of the newer authoring features of the spine
// pptx package. It builds a four-slide deck that exercises, in one place:
//
//   - a title slide with a slide transition;
//   - a slide with a native chart (Slide.AddChart) whose data workbook is
//     embedded automatically;
//   - a slide with a table and an in-text hyperlink;
//   - a slide with an auto shape carrying layered visual effects
//     (shadow + glow + reflection) plus an entrance animation;
//   - slide transitions using two of the newer variants (Zoom and Wheel);
//   - slide sections (Presentation.AddSection) grouping the slides the way
//     PowerPoint's thumbnail pane does;
//   - a threaded comment with a reply.
//
// After saving, it reopens the file and reads the sections, animations, charts,
// and comments back out to prove the round trip.
//
// ── The shape-id / animation ordering lesson ────────────────────────────────
//
// Animations target a shape by its cNvPr id (an unsigned integer), NOT by a Go
// pointer. For a shape you create through this package that id is only assigned
// when the deck is first saved (sequentially, in the order shapes were added) —
// Shape.ID() reports 0 until then. So you cannot reliably call AddAnimation on a
// freshly created shape: you have nothing stable to target.
//
// The robust pattern this example demonstrates is two-phase:
//
//	Phase 1: build the deck and Save() it. Ids are now materialized on disk.
//	Phase 2: Open() the saved deck, find the shape (here, by name), read its
//	         now-known ID(), AddAnimation() against that id, and Save() again.
//
// Reading the id back after a save is what makes the animation land on the
// right shape.
//
// Run with:
//
//	go run ./examples/pptx_deck            # writes to deck.pptx
//	go run ./examples/pptx_deck out.pptx   # writes to out.pptx
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/pptx"
)

// Names we give shapes so we can find them again after a save/reopen round trip
// (ids are assigned at save time; names are stable across the trip).
const (
	effectShapeName = "Highlight Star"
	commentAuthor   = "Spine Reviewer"
)

func main() {
	// Resolve the output path. Like every other example, it defaults to the
	// working directory.
	outputPath := "deck.pptx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
	}

	// Phase 1: build the deck and save it. This assigns shape ids on disk.
	buildDeck(outputPath)

	// Phase 2: reopen, target the now-known shape id with an animation, save.
	addEntranceAnimation(outputPath)

	// Verify: reopen the finished deck and read the new features back out.
	verify(outputPath)

	fmt.Printf("\nDeck written to: %s\n", outputPath)
}

// buildDeck creates the four slides, their content, transitions, sections, and
// a comment, then saves the presentation for the first time.
func buildDeck(outputPath string) {
	p := pptx.CreateWidescreen()
	p.Properties.Title = "Spine Feature Tour"
	p.Properties.Creator = "Spine Library"
	p.Properties.Subject = "Charts, tables, effects, animations, sections"

	titleSlide := addTitleSlide(p)
	chartSlide := addChartSlide(p)
	tableSlide := addTableSlide(p)
	effectSlide := addEffectSlide(p)

	// ── Slide sections ───────────────────────────────────────────────────────
	// Sections are named groups of slides — the collapsible groups in
	// PowerPoint's slide-thumbnail pane. A slide belongs to at most one section;
	// Section.AddSlide moves it out of any other section automatically.
	intro := p.AddSection("Introduction")
	intro.AddSlide(titleSlide)

	body := p.AddSection("Data & Visuals")
	body.AddSlide(chartSlide)
	body.AddSlide(tableSlide)
	body.AddSlide(effectSlide)

	// ── A threaded comment with a reply ──────────────────────────────────────
	// Newly added comments always use the modern (2018 threaded) mechanism,
	// which supports replies and resolution. AddCommentAt anchors it at a slide
	// position (in EMUs).
	comment, err := effectSlide.AddCommentAt(
		int64(dml.Inches(9)), int64(dml.Inches(1)),
		commentAuthor, "Love the glow on this shape — ship it.")
	if err != nil {
		log.Fatalf("adding a comment: %v", err)
	}
	if _, err := comment.Reply("Deck Author", "Thanks! Locking it in."); err != nil {
		log.Fatalf("replying to a comment: %v", err)
	}

	if err := p.Save(outputPath); err != nil {
		log.Fatalf("phase 1 save: %v", err)
	}
	fmt.Printf("Phase 1: built %d slides across %d sections and saved.\n",
		p.SlideCount(), len(p.Sections()))
}

// addTitleSlide builds the cover slide and gives it a Zoom-in transition.
func addTitleSlide(p *pptx.Presentation) *pptx.Slide {
	s := p.AddSlide()
	s.SetName("Title")

	// Zoom is one of the newer transition variants. Its Direction chooses
	// whether the slide zooms In or Out.
	s.SetTransition(pptx.Transition{
		Type:           pptx.TransitionZoom,
		Direction:      pptx.TransitionDirIn,
		Duration:       0.5,
		AdvanceOnClick: true,
	})

	// Full-bleed gradient background.
	bg := pptx.NewAutoShape(pptx.PresetRect)
	bg.SetPosition(0, 0)
	bg.SetSize(dml.Inches(13.33), dml.Inches(7.5))
	bg.SetFill(dml.NewGradientFill(270,
		dml.GradientStop{Position: 0, Color: dml.NewRGB(17, 24, 39).ToColor()},
		dml.GradientStop{Position: 1, Color: dml.NewRGB(37, 99, 235).ToColor()},
	))
	bg.SetLine(dml.Line{Width: 0})
	mustAddShape(s, bg)

	title := s.AddTextBox()
	title.SetPosition(dml.Inches(1), dml.Inches(2.6))
	title.SetSize(dml.Inches(11.33), dml.Inches(1.5))
	tp := title.TextFrame().AddParagraph()
	tp.SetAlignment(enum.TextAlignCenter)
	tr := tp.AddRun()
	tr.SetText("Spine Feature Tour")
	tr.SetFontSize(48)
	tr.SetBold(true)
	tr.SetColor(dml.ColorWhite)

	sub := s.AddTextBox()
	sub.SetPosition(dml.Inches(1), dml.Inches(4.2))
	sub.SetSize(dml.Inches(11.33), dml.Inches(1))
	sp := sub.TextFrame().AddParagraph()
	sp.SetAlignment(enum.TextAlignCenter)
	sr := sp.AddRun()
	sr.SetText("Charts • Tables • Effects • Animations • Sections")
	sr.SetFontSize(22)
	sr.SetColor(dml.NewRGB(191, 219, 254).ToColor())

	return s
}

// addChartSlide places a native column chart on a slide. AddChart writes the
// chart part, embeds an .xlsx data workbook, and wires up all relationships;
// the caller only supplies the chart definition and a placement rectangle.
func addChartSlide(p *pptx.Presentation) *pptx.Slide {
	s := p.AddSlide()
	s.SetName("Chart")
	s.SetTransition(pptx.Transition{
		Type:           pptx.TransitionPush,
		Direction:      pptx.TransitionDirLeft,
		Duration:       0.5,
		AdvanceOnClick: true,
	})

	addHeading(s, "Quarterly Revenue")

	// Build a column chart with two series and category labels. The chart
	// package is format-neutral; AddChart is what binds it into the pptx.
	c := chart.NewColumn()
	c.SetTitle("Revenue by Quarter (USD, thousands)")
	c.SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
	c.AddSeries("Product A", []float64{120, 145, 172, 198}).SetColor("2563EB")
	c.AddSeries("Product B", []float64{88, 102, 96, 131}).SetColor("F59E0B")
	c.SetLegend(chart.LegendBottom)
	c.SetAxisTitles("Quarter", "Revenue")
	c.SetDataLabels(true)

	// Position and size are EMUs; dml.Inches converts from inches.
	if err := s.AddChart(c,
		int64(dml.Inches(1)), int64(dml.Inches(1.6)),
		int64(dml.Inches(11.33)), int64(dml.Inches(5.2))); err != nil {
		log.Fatalf("AddChart: %v", err)
	}

	return s
}

// addTableSlide builds a comparison table and a paragraph with an in-text
// hyperlink (the run-level a:hlinkClick, allocated as an external relationship
// on save).
func addTableSlide(p *pptx.Presentation) *pptx.Slide {
	s := p.AddSlide()
	s.SetName("Table")

	// Wheel is another newer transition variant; Spokes controls its blades.
	s.SetTransition(pptx.Transition{
		Type:           pptx.TransitionWheel,
		Spokes:         4,
		Duration:       1.0,
		AdvanceOnClick: true,
	})

	addHeading(s, "Format Support")

	t := s.AddTable(4, 3)
	t.SetPosition(dml.Inches(1.5), dml.Inches(1.8))
	t.SetSize(dml.Inches(10.33), dml.Inches(3))
	headers := []string{"Format", "Extension", "Status"}
	for j, h := range headers {
		t.Cell(0, j).SetText(h)
	}
	rows := [][]string{
		{"PowerPoint", ".pptx", "Full support"},
		{"Word", ".docx", "Full support"},
		{"Excel", ".xlsx", "Full support"},
	}
	for i, row := range rows {
		for j, val := range row {
			t.Cell(i+1, j).SetText(val)
		}
	}

	// A text box whose run carries a hyperlink with a tooltip.
	link := s.AddTextBox()
	link.SetPosition(dml.Inches(1.5), dml.Inches(5.2))
	link.SetSize(dml.Inches(10.33), dml.Inches(0.6))
	lp := link.TextFrame().AddParagraph()
	lr := lp.AddRun()
	lr.SetText("Read the documentation")
	lr.SetFontSize(18)
	lr.SetColor(dml.NewRGB(37, 99, 235).ToColor())
	lr.SetHyperlink("https://github.com/mgilbir/spine").
		SetTooltip("Open the spine repository")

	return s
}

// addEffectSlide places a star shape that layers three visual effects, and
// names it so a later phase can find it and attach an animation by id.
func addEffectSlide(p *pptx.Presentation) *pptx.Slide {
	s := p.AddSlide()
	s.SetName("Effects")
	s.SetTransition(pptx.Transition{
		Type:           pptx.TransitionFade,
		Duration:       0.5,
		AdvanceOnClick: true,
	})

	addHeading(s, "Layered Shape Effects")

	star := pptx.NewAutoShape(pptx.PresetStar5)
	// The name is our stable handle across the save/reopen round trip; the
	// numeric id we will animate against is only assigned at save time.
	star.SetName(effectShapeName)
	star.SetPosition(dml.Inches(5), dml.Inches(2.4))
	star.SetSize(dml.Inches(3.3), dml.Inches(3.3))
	star.SetFill(dml.NewSolidFill(dml.NewRGB(245, 158, 11).ToColor()))
	star.SetLine(dml.Line{Width: 0})

	// Three effects stack on one shape: each setter adds to the shape's
	// a:effectLst without disturbing the effects already set.
	star.SetShadow(dml.Shadow{
		Color:    dml.NewRGB(0, 0, 0).ToColor().WithAlpha(50),
		BlurRad:  8,
		Distance: 5,
		Angle:    315,
	})
	star.SetGlow(pptx.Glow{
		Color:  dml.NewRGB(251, 191, 36).ToColor(),
		Radius: 12, // points
	})
	star.SetReflection(pptx.Reflection{
		BlurRadius:   3,
		Distance:     2,
		StartOpacity: 0.55,
		EndOpacity:   0,
	})
	mustAddShape(s, star)

	caption := s.AddTextBox()
	caption.SetPosition(dml.Inches(1), dml.Inches(6.1))
	caption.SetSize(dml.Inches(11.33), dml.Inches(0.8))
	cp := caption.TextFrame().AddParagraph()
	cp.SetAlignment(enum.TextAlignCenter)
	cr := cp.AddRun()
	cr.SetText("Shadow + glow + reflection on a single shape, plus a fade-in animation.")
	cr.SetFontSize(16)
	cr.SetColor(dml.NewRGB(75, 85, 99).ToColor())

	return s
}

// addEntranceAnimation reopens the saved deck, looks up the effect shape's
// now-materialized id, and attaches a fade-in entrance animation to it. This is
// the second half of the two-phase pattern described in the package comment:
// ids only exist after a save, so animation authoring happens on a reopened
// deck.
func addEntranceAnimation(outputPath string) {
	p, err := pptx.Open(outputPath)
	if err != nil {
		log.Fatalf("phase 2 open: %v", err)
	}
	defer func() { _ = p.Close() }()

	slide := slideByName(p, "Effects")
	if slide == nil {
		log.Fatal("phase 2: could not find the Effects slide")
	}

	// Find our star by name and read its id, which is now assigned. Slide.Shapes
	// returns the Shape interface; the concrete shapes expose ID().
	shapeID, ok := shapeIDByName(slide, effectShapeName)
	if !ok {
		log.Fatalf("phase 2: shape %q has no id (was the deck saved first?)", effectShapeName)
	}
	fmt.Printf("Phase 2: %q resolved to shape id %d after save.\n", effectShapeName, shapeID)

	// Now the animation has a stable target. TriggerOnClick starts it on the
	// next slide-advance click.
	slide.AddAnimation(shapeID, pptx.EffectFadeIn, pptx.TriggerOnClick)

	if err := p.Save(outputPath); err != nil {
		log.Fatalf("phase 2 save: %v", err)
	}
	fmt.Println("Phase 2: attached a fade-in animation and re-saved.")
}

// verify reopens the finished deck and reads the round-tripped features back.
func verify(outputPath string) {
	p, err := pptx.Open(outputPath)
	if err != nil {
		log.Fatalf("verify open: %v", err)
	}
	defer func() { _ = p.Close() }()

	fmt.Println("\nVerification (read back from disk):")

	// Sections and their membership.
	for _, sec := range p.Sections() {
		names := make([]string, 0, len(sec.Slides()))
		for _, sl := range sec.Slides() {
			names = append(names, sl.Name())
		}
		fmt.Printf("  section %-16q -> %v\n", sec.Name(), names)
	}

	// Charts (parsed back from the embedded chart part).
	for _, c := range p.Charts() {
		fmt.Printf("  chart  %q: %d series, categories %v\n",
			c.Title(), len(c.SeriesList()), c.Categories())
	}

	// Animations on the effects slide.
	if effects := slideByName(p, "Effects"); effects != nil {
		for _, a := range effects.Animations() {
			fmt.Printf("  animation on shape %d: effect=%s trigger=%s\n",
				a.ShapeID(), a.Effect(), a.Trigger())
		}

		// Comments, including threaded replies.
		for _, cm := range effects.Comments() {
			fmt.Printf("  comment by %q: %q (%d repl(y/ies))\n",
				cm.Author(), cm.Text(), len(cm.Replies()))
		}
	}

	// Transitions per slide.
	for _, sl := range p.Slides() {
		if tr := sl.Transition(); tr != nil {
			fmt.Printf("  transition on %-8q: type=%d\n", sl.Name(), tr.Type)
		}
	}
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
	hr.SetColor(dml.NewRGB(17, 24, 39).ToColor())
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

// shapeIDByName returns the cNvPr id of the named shape on a slide. It works
// only after the deck has been saved at least once (ids are 0 before that).
func shapeIDByName(s *pptx.Slide, name string) (uint32, bool) {
	for _, sh := range s.Shapes() {
		if sh.Name() != name {
			continue
		}
		// The Shape interface does not expose ID(); the concrete shape types do.
		if ided, ok := sh.(interface{ ID() uint32 }); ok {
			if id := ided.ID(); id != 0 {
				return id, true
			}
		}
	}
	return 0, false
}
