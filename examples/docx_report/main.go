// Example: Author a rich Word report with the spine DOCX library.
//
// Where examples/create_document focuses on the everyday furniture of a
// document (page setup, headers/footers, lists, a table, an image), this
// example exercises the richer authoring surface spine grew for structured,
// review-ready reports:
//
//   - Custom paragraph AND character styles (Document.Styles)
//   - A custom multi-level numbered list definition (Document.Numbering)
//   - A field-driven table of contents (Document.AddTableOfContents)
//   - A table with a vertical cell merge, borders, and shading
//   - An inline image and an inline, editable chart (Document.AddChart)
//   - Review metadata: threaded comments (Run.AddComment / Comment.Reply)
//   - A block content control (Document.AddContentControl)
//   - Section layout: a page-numbered body plus a two-column appendix
//   - Document protection (Document.Protect)
//
// After saving, the program reopens the file and reads the comments and
// content controls back, proving the metadata survives a round trip.
//
// Run with: go run ./examples/docx_report
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/opc"
)

func main() {
	doc := docx.Create()

	doc.Properties.Title = "Quarterly Operations Report"
	doc.Properties.Creator = "Spine Library"
	doc.Properties.Subject = "Demonstration of spine's rich DOCX authoring API"

	// ── Section & Page Setup ─────────────────────────────────────────
	//
	// The default (final) section governs the main body. We give it A4
	// pages, one-inch margins, and decimal page numbering starting at 1.
	// The footer below renders that number with a PAGE field.

	body := doc.DefaultSection()
	body.SetPageSize(docx.PageSizeA4())
	body.SetMargins(docx.PageMargins{
		Top: 72, Bottom: 72, Left: 72, Right: 72, // 1 inch (points) each
		Header: 36, Footer: 36,
	})
	startAt := 1
	body.SetPageNumbering(docx.PageNumbering{Format: "decimal", Start: &startAt})

	// ── Header & Footer ──────────────────────────────────────────────

	hdr := doc.AddHeader(docx.HeaderDefault)
	hp := hdr.AddParagraphWithText("Quarterly Operations Report")
	hp.SetAlignment(docx.AlignmentRight)
	hpRun := hp.Runs()[0]
	hpRun.SetColor("808080")
	hpRun.SetFontSize(9)

	ftr := doc.AddFooter(docx.FooterDefault)
	fp := ftr.AddParagraph()
	fp.SetAlignment(docx.AlignmentCenter)
	// "Page N of M" built from two Word fields Word recalculates on repagination.
	fp.AddText("Page ").SetColor("808080")
	fp.AddField(docx.FieldPage).SetColor("808080")
	fp.AddText(" of ").SetColor("808080")
	fp.AddField(docx.FieldNumPages).SetColor("808080")

	// ── Custom Styles ────────────────────────────────────────────────
	//
	// Document.Styles() edits word/styles.xml. A created document is seeded
	// with Word's compact defaults (Normal + Heading1-9), so our styles sit
	// alongside the built-ins that AddHeading references. Setters chain and
	// each one marks the styles part modified.
	styles := doc.Styles()

	// A paragraph style for the report's cover title.
	styles.AddParagraphStyle("ReportTitle", "Report Title").
		SetBasedOn("Normal").
		SetNext("Normal").
		SetQuickFormat(true).
		SetFont("Georgia").
		SetFontSize(28).
		SetBold(true).
		SetColor("1F3864").
		SetAlignment(docx.AlignmentCenter).
		SetSpaceAfter(6)

	// A paragraph style for the subtitle under the title.
	styles.AddParagraphStyle("ReportSubtitle", "Report Subtitle").
		SetBasedOn("Normal").
		SetFont("Georgia").
		SetFontSize(13).
		SetItalic(true).
		SetColor("7F7F7F").
		SetAlignment(docx.AlignmentCenter).
		SetSpaceAfter(18)

	// A character (run) style used to emphasise defined terms inline. A
	// character style formats individual runs, not whole paragraphs.
	styles.AddCharacterStyle("KeyTerm", "Key Term").
		SetBold(true).
		SetColor("C00000")

	// ── Cover Title (custom paragraph styles) ────────────────────────

	title := doc.AddParagraph()
	title.SetStyle("ReportTitle")
	title.SetText("Quarterly Operations Report")

	subtitle := doc.AddParagraph()
	subtitle.SetStyle("ReportSubtitle")
	subtitle.SetText("Fiscal Year 2026 · Second Quarter")

	// ── Table of Contents ────────────────────────────────────────────
	//
	// AddTableOfContents inserts a TOC field wrapped in a structured document
	// tag, so Word shows its "Update Table" control. Word computes the entries
	// (from the Heading1/Heading2 styles used below) the first time it opens
	// the file; until then the placeholder text is shown.
	doc.AddHeading("Contents", 1)
	if err := doc.AddTableOfContents(docx.TOCOptions{MinLevel: 1, MaxLevel: 2}); err != nil {
		log.Fatalf("Failed to add table of contents: %v", err)
	}

	// ── Overview (custom character style inline) ─────────────────────

	doc.AddHeading("Overview", 1)

	overview := doc.AddParagraph()
	overview.SetLineSpacing(1.15)
	overview.SetSpaceAfter(6)
	overview.AddText(
		"This report summarises operational performance for the quarter. The ")
	overview.AddText("service-level agreement").SetStyle("KeyTerm") // KeyTerm character style
	overview.AddText(
		" (SLA) target was met in every region, and the incident backlog fell for the third consecutive period.")

	// ── Custom Numbered List (Document.Numbering) ────────────────────
	//
	// Document.Numbering() builds definitions in word/numbering.xml. Here we
	// author a two-level list: upper-Roman top level ("I.", "II.") with a
	// lower-alpha sub level ("a)", "b)"). Paragraph.SetListStyle applies the
	// resulting instance at a given level.
	doc.AddHeading("Key Findings", 1)

	list := doc.Numbering().AddDefinition()
	list.SetLevel(0, docx.NumberFormatUpperRoman, "%1.")
	list.SetLevel(1, docx.NumberFormatLowerLetter, "%2)").SetIndent(54)
	listStyle := list.ListStyle()

	findings := []struct {
		level int
		text  string
	}{
		{0, "Reliability improved across the board."},
		{1, "Uptime reached 99.98%, up from 99.94% last quarter."},
		{1, "Mean time to recovery dropped to 18 minutes."},
		{0, "Costs held steady despite higher traffic."},
		{1, "Compute spend grew 4% against 21% traffic growth."},
	}
	for _, f := range findings {
		p := doc.AddParagraph()
		p.SetText(f.text)
		p.SetListStyle(listStyle, f.level)
		p.SetSpaceAfter(2)
	}

	// ── Table with a Vertical Cell Merge ─────────────────────────────
	//
	// A 4x3 grid where the first column groups two rows per region using a
	// vertical merge: the top cell restarts the merge (and holds the label),
	// the cell below continues it (and stays empty). Borders and header
	// shading come from the table/cell property setters.
	doc.AddHeading("Regional Detail", 1)

	tbl := doc.AddTable(5, 3)
	tbl.SetWidth(450) // points
	tbl.SetBorders(docx.TableBorders{
		Top:     &docx.Border{Style: "single", Width: 1, Color: "1F3864"},
		Bottom:  &docx.Border{Style: "single", Width: 1, Color: "1F3864"},
		Left:    &docx.Border{Style: "single", Width: 0.5, Color: "BFBFBF"},
		Right:   &docx.Border{Style: "single", Width: 0.5, Color: "BFBFBF"},
		InsideH: &docx.Border{Style: "single", Width: 0.5, Color: "D9D9D9"},
		InsideV: &docx.Border{Style: "single", Width: 0.5, Color: "D9D9D9"},
	})

	// Header row.
	header := tbl.Rows()[0]
	header.SetHeaderRow(true)
	for i, text := range []string{"Region", "Metric", "Value"} {
		cell := header.Cells()[i]
		cell.SetShading("1F3864")
		cell.SetVerticalAlignment("center")
		p := cell.Paragraphs()[0]
		p.SetAlignment(docx.AlignmentCenter)
		r := p.AddRun()
		r.SetText(text)
		r.SetBold(true)
		r.SetColor("FFFFFF")
	}

	// Two regions, two metrics each. The region name spans its two rows via a
	// vertical merge on column 0.
	type metricRow struct{ region, metric, value string }
	data := []metricRow{
		{"Americas", "Uptime", "99.99%"},
		{"", "Incidents", "3"},
		{"EMEA", "Uptime", "99.97%"},
		{"", "Incidents", "5"},
	}
	for i, d := range data {
		row := tbl.Rows()[i+1]
		region := row.Cells()[0]
		if d.region != "" {
			// Top cell of a region group: restart the merge and label it.
			region.SetVerticalMerge(docx.VerticalMergeRestart)
			region.SetVerticalAlignment("center")
			rp := region.Paragraphs()[0]
			rr := rp.AddRun()
			rr.SetText(d.region)
			rr.SetBold(true)
		} else {
			// Continuation cell: merges upward, carries no text of its own.
			region.SetVerticalMerge(docx.VerticalMergeContinue)
		}
		row.Cells()[1].Paragraphs()[0].SetText(d.metric)
		row.Cells()[2].Paragraphs()[0].SetText(d.value)
	}

	// ── Inline Image ─────────────────────────────────────────────────

	doc.AddHeading("Status Legend", 1)
	legend := doc.AddParagraph()
	legend.AddText("Green indicates every SLA was met: ")
	img, err := legend.AddRun().AddImageFromBytes(solidPNG(24, color.RGBA{R: 0x2E, G: 0x7D, B: 0x32, A: 0xFF}), opc.ContentTypePNG)
	if err != nil {
		log.Fatalf("Failed to add image: %v", err)
	}
	img.SetSize(12, 12) // a small inline swatch
	img.SetAltText("Green status swatch")

	// ── Inline Chart (Document.AddChart) ─────────────────────────────
	//
	// AddChart embeds an editable chart. Build it with the shared chart
	// package, then hand it over with a display size in EMUs (914400 per
	// inch). The data is stored in an embedded workbook so Office can edit it.
	doc.AddHeading("Uptime Trend", 1)

	col := chart.NewColumn()
	col.SetTitle("Quarterly Uptime (%)")
	col.SetCategories([]string{"Q1", "Q2", "Q3", "Q4"})
	col.AddSeries("Americas", []float64{99.95, 99.99, 99.98, 99.99})
	col.AddSeries("EMEA", []float64{99.93, 99.97, 99.96, 99.98})
	if err := doc.AddChart(col, 6*914400, 3*914400); err != nil { // 6" x 3"
		log.Fatalf("Failed to add chart: %v", err)
	}

	// ── Comments (threaded review metadata) ──────────────────────────
	//
	// A comment is anchored over a run; Reply threads under it and Resolve
	// marks the thread done. These are read back after reopen below.
	doc.AddHeading("Next Steps", 1)
	action := doc.AddParagraph()
	action.AddText("Owners should confirm the ")
	target := action.AddText("Q3 capacity plan") // the run we comment on
	action.AddText(" before the next review.")

	comment := target.AddComment("Reviewer A", "Is the capacity plan finalised?")
	comment.Reply("Owner", "Draft is ready; final sign-off pending.")
	comment.Resolve() // mark the whole thread resolved

	// ── Content Control (Document.AddContentControl) ─────────────────
	//
	// A block-level rich-text content control carries a tag (programmatic id)
	// and a value. Word renders it as an editable field; SetAlias gives it a
	// friendly label. Read back after reopen below.
	doc.AddHeading("Sign-off", 1)
	doc.AddParagraphWithText("Approved by:")
	cc := doc.AddContentControl("approver", "Pending")
	cc.SetAlias("Approving Manager")

	// ── Two-Column Appendix (section layout) ─────────────────────────
	//
	// AddSectionBreak ends the single-column body (its section properties —
	// page size, numbering, header/footer — move onto the last body
	// paragraph) and returns a fresh final section. We give that section the
	// same page geometry, a matching footer, and a two-column layout with a
	// separator line, so the appendix flows in newspaper columns.
	appendix := doc.AddSectionBreak()
	appendix.SetPageSize(docx.PageSizeA4())
	appendix.SetMargins(docx.PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72, Header: 36, Footer: 36})
	appendix.SetColumns(docx.Columns{Count: 2, Spacing: 24, Separator: true, EqualWidth: true})

	af := doc.AddFooter(docx.FooterDefault) // footer for the new section
	afp := af.AddParagraph()
	afp.SetAlignment(docx.AlignmentCenter)
	afp.AddText("Appendix · Page ").SetColor("808080")
	afp.AddField(docx.FieldPage).SetColor("808080")

	doc.AddHeading("Appendix: Glossary", 1)
	glossary := []struct{ term, def string }{
		{"SLA", "Service-level agreement; the contractual uptime target."},
		{"MTTR", "Mean time to recovery after an incident is detected."},
		{"Uptime", "Share of the period the service was available."},
		{"Backlog", "Open incidents carried into the next period."},
	}
	for _, g := range glossary {
		p := doc.AddParagraph()
		p.SetSpaceAfter(6)
		p.AddText(g.term + ": ").SetStyle("KeyTerm")
		p.AddText(g.def)
	}

	// ── Document Protection ──────────────────────────────────────────
	//
	// Restrict editing to comments only and enforce it. Note this is a UI
	// guard, not encryption — any tool can clear it — so it is never a way to
	// protect confidential data.
	doc.Protect(docx.DocumentProtectionOptions{
		Edit:     docx.EditComments,
		Password: "review", // deliberately weak legacy hash; UI guard only
	})

	// ── Save ─────────────────────────────────────────────────────────

	outputPath := "report.docx"
	if len(os.Args) > 1 {
		outputPath = os.Args[1]
	}
	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}
	}
	if err := doc.Save(outputPath); err != nil {
		log.Fatalf("Failed to save document: %v", err)
	}
	fmt.Printf("Report saved to: %s\n", outputPath)

	// ── Reopen and verify the review metadata survived ───────────────

	reopened, err := docx.Open(outputPath)
	if err != nil {
		log.Fatalf("Failed to reopen document: %v", err)
	}

	fmt.Println("\nComments read back from the saved file:")
	for _, c := range reopened.Comments() {
		fmt.Printf("  [%s] %q on %q (resolved=%v)\n",
			c.Author(), c.Text(), c.AnchorText(), c.Resolved())
		for _, reply := range c.Replies() {
			fmt.Printf("    ↳ [%s] %q\n", reply.Author(), reply.Text())
		}
	}

	fmt.Println("\nContent controls read back from the saved file:")
	for _, ctrl := range reopened.ContentControls() {
		fmt.Printf("  tag=%q alias=%q value=%q\n", ctrl.Tag(), ctrl.Alias(), ctrl.Value())
	}

	if prot := reopened.Protection(); prot != nil {
		fmt.Printf("\nProtection: edit=%s enforced=%v hasPassword=%v\n",
			prot.Edit(), prot.Enforced(), prot.HasPassword())
	}

	fmt.Printf("\nDone. Sections: %d, top-level paragraphs: %d\n",
		len(reopened.Sections()), len(reopened.Paragraphs()))
}

// solidPNG generates a solid-color square PNG of the given size.
func solidPNG(size int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatalf("Failed to encode PNG: %v", err)
	}
	return buf.Bytes()
}
