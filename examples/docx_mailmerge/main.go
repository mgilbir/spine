// Example: Author a mail-merge form letter with the spine DOCX library.
//
// Where examples/docx_report tours the structured-report surface (styles,
// numbering, TOC, comments, protection), this example exercises the features
// spine grew for *template* documents — the kind a mail-merge campaign, a
// contract generator, or a review workflow starts from:
//
//   - Mail-merge configuration (Document.SetMailMerge): mark the file as a
//     form-letter main document and attach an Office Data Source Object (odso)
//     describing the recipient columns.
//   - MERGEFIELD placeholders (Paragraph.AddMergeField): «FirstName» style
//     fields Word swaps for real values when the merge runs, plus reading them
//     back with Document.MergeFields().
//   - A text-box callout (Paragraph.AddTextBox): a bordered, shaded box that
//     floats a note beside the letter body, needing no extra parts.
//   - A text watermark (Document.SetTextWatermark): a "DRAFT" WordArt stamp
//     across every page's header furniture.
//   - Author-side tracked changes (Paragraph.AddInsertedRun / Run.MarkDeleted):
//     insertions and deletions attributed to a reviewer, then enumerated with
//     Document.Revisions().
//
// After saving, the program reopens the file and reads the merge fields, text
// boxes, watermark, and revisions back, proving each survives a round trip.
//
// The output is deterministic: tracked-change timestamps are fixed, so running
// the program twice yields the same bytes.
//
// Run with: go run ./examples/docx_mailmerge
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mgilbir/spine/docx"
)

// reviewDate is a fixed timestamp for the tracked changes. Using the *WithDate
// authoring variants (instead of AddInsertedRun / MarkDeleted, which stamp
// time.Now()) keeps the emitted markup byte-for-byte reproducible.
var reviewDate = time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)

func main() {
	doc := docx.Create()

	doc.Properties.Title = "Renewal Letter (Mail Merge Template)"
	doc.Properties.Creator = "Spine Library"
	doc.Properties.Subject = "Demonstration of spine's mail-merge and template API"

	// ── Page Setup ───────────────────────────────────────────────────

	sec := doc.DefaultSection()
	sec.SetPageSize(docx.PageSizeA4())
	sec.SetMargins(docx.PageMargins{
		Top: 72, Bottom: 72, Left: 72, Right: 72, // 1 inch each
		Header: 36, Footer: 36,
	})

	// ── Mail-Merge Configuration (Document.SetMailMerge) ─────────────
	//
	// SetMailMerge writes w:mailMerge into the settings part, turning the file
	// into a form-letter *main document*: Word's Mailings tab will recognise it
	// and offer to complete the merge. The odso (Office Data Source Object)
	// records where the recipient records live and maps data-source columns to
	// the merge-field names used in the body below. No external data file is
	// shipped here — the connect string just points Word at one — so the
	// template stays self-contained and dependency-free.
	doc.SetMailMerge(&docx.MailMerge{
		MainDocumentType: docx.MailMergeFormLetters,
		DataType:         "textFile",
		ConnectString:    "SELECT * FROM recipients.csv",
		Destination:      "newDocument",
		ViewMergedData:   false, // show field codes «FirstName», not sample data
		DataSource: &docx.MailMergeDataSource{
			ConnectionType:  "textFile",
			FirstRowHeader:  true,
			ColumnDelimiter: ',',
			FieldMappings: []docx.MailMergeFieldMapping{
				{Name: "FirstName", MappedName: "First Name", Column: 0, Type: "dbColumn"},
				{Name: "LastName", MappedName: "Last Name", Column: 1, Type: "dbColumn"},
				{Name: "PlanName", MappedName: "Plan", Column: 2, Type: "dbColumn"},
				{Name: "RenewalDate", MappedName: "Renewal Date", Column: 3, Type: "dbColumn"},
			},
		},
	})

	// ── Letterhead ───────────────────────────────────────────────────

	title := doc.AddHeading("Northwind Mutual — Policy Renewal", 1)
	title.SetSpaceAfter(6)

	dateline := doc.AddParagraph()
	dateline.SetAlignment(docx.AlignmentRight)
	dateline.AddText("Generated on merge").SetColor("808080")

	// ── Salutation with MERGEFIELDs (Paragraph.AddMergeField) ────────
	//
	// AddMergeField inserts a MERGEFIELD simple field plus a «name» placeholder
	// run. Word replaces the placeholder with the recipient's value at merge
	// time; until then the document literally shows the guillemet-wrapped name.
	// The returned Run is the placeholder, so we can format the merged value —
	// here the surname is bold.
	greeting := doc.AddParagraph()
	greeting.SetSpaceBefore(12)
	greeting.AddText("Dear ")
	greeting.AddMergeField("FirstName")
	greeting.AddText(" ")
	greeting.AddMergeField("LastName").SetBold(true)
	greeting.AddText(",")

	// ── Body with inline merge fields ────────────────────────────────

	body := doc.AddParagraph()
	body.SetSpaceBefore(12)
	body.SetLineSpacing(1.15)
	body.AddText("Thank you for being a valued member. Your ")
	body.AddMergeField("PlanName").SetBold(true)
	body.AddText(" plan is scheduled to renew on ")
	body.AddMergeField("RenewalDate").SetBold(true)
	body.AddText(". No action is required to continue your coverage.")

	// ── Text-Box Callout (Paragraph.AddTextBox) ──────────────────────
	//
	// A text box is a self-contained DrawingML shape: it needs no extra parts or
	// relationships, so it round-trips like an image. We float a shaded, rounded
	// "reminder" callout beside the body. AddTextBox splits its text on newlines
	// into one paragraph per line.
	callout := doc.AddParagraph()
	callout.AddTextBox(
		"Reminder\nUpdate your payment details before the renewal date to avoid a lapse.",
		docx.TextBoxOptions{
			WidthEMU:    int64(3 * 914400), // 3 inches wide
			HeightEMU:   int64(914400),     // 1 inch tall
			Shape:       docx.ShapeRoundRectangle,
			FillColor:   "FFF4CE", // pale amber
			BorderColor: "D9A400",
		})

	// ── Closing with author-side tracked changes ─────────────────────
	//
	// The reviewer sharpens the closing line with Track Changes on. AddInsertedRun
	// wraps new text in a w:ins (attributed to the author); Run.MarkDeleted wraps
	// an existing run in a w:del, converting its w:t to w:delText. We use the
	// *WithDate variants for a fixed timestamp (the plain AddInsertedRun /
	// MarkDeleted stamp time.Now()). Both read back via Document.Revisions().
	doc.AddParagraph().SetSpaceBefore(12) // spacer

	closing := doc.AddParagraph()
	closing.AddText("We ")
	// A tracked deletion: the reviewer removes the weaker word "hope".
	closing.AddText("hope").MarkDeletedWithDate("Reviewer A", reviewDate)
	// A tracked insertion: replaced with a more confident phrasing.
	closing.AddInsertedRunWithDate("Reviewer A", "look forward to", reviewDate)
	closing.AddText(" serving you for another year.")

	signoff := doc.AddParagraph()
	signoff.SetSpaceBefore(12)
	signoff.AddText("Sincerely,")
	sig := doc.AddParagraph()
	sig.AddText("The Northwind Mutual Team").SetItalic(true)

	// ── "DRAFT" Text Watermark (Document.SetTextWatermark) ───────────
	//
	// SetTextWatermark stamps a diagonal WordArt watermark into the header
	// furniture so it shows behind every page — the classic way to mark a
	// template as not-yet-final. It creates a default header if the document has
	// none.
	if err := doc.SetTextWatermark("DRAFT", docx.WatermarkOptions{
		Color:    "C0C0C0",
		Diagonal: true,
	}); err != nil {
		log.Fatalf("Failed to set watermark: %v", err)
	}

	// ── Save ─────────────────────────────────────────────────────────

	outputPath := "mailmerge.docx"
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
	fmt.Printf("Mail-merge template saved to: %s\n", outputPath)

	// ── Reopen and verify everything survived the round trip ─────────

	reopened, err := docx.Open(outputPath)
	if err != nil {
		log.Fatalf("Failed to reopen document: %v", err)
	}

	// Mail-merge configuration.
	if mm := reopened.MailMerge(); mm != nil {
		fmt.Printf("\nMail merge: type=%s dataType=%s destination=%s\n",
			mm.MainDocumentType, mm.DataType, mm.Destination)
		if mm.DataSource != nil {
			fmt.Printf("  data source: %d column mapping(s), firstRowHeader=%v\n",
				len(mm.DataSource.FieldMappings), mm.DataSource.FirstRowHeader)
		}
	} else {
		log.Fatal("expected a mail-merge configuration after reopen")
	}

	// Merge fields, in first-appearance order.
	fields := reopened.MergeFields()
	fmt.Printf("\nMerge fields read back (%d): %v\n", len(fields), fields)

	// Text boxes.
	fmt.Println("\nText boxes read back:")
	for _, tb := range reopened.TextBoxes() {
		fmt.Printf("  %.0fx%.0f pt, shape=%q, text=%q\n",
			tb.Width(), tb.Height(), tb.Shape(), tb.Text())
	}

	// Watermark.
	if wm := reopened.Watermark(); wm != nil && wm.Type == docx.WatermarkText {
		fmt.Printf("\nWatermark read back: %q\n", wm.Text)
	} else {
		log.Fatal("expected a text watermark after reopen")
	}

	// Tracked changes.
	fmt.Println("\nRevisions read back:")
	for _, rev := range reopened.Revisions() {
		fmt.Printf("  [%s] %s %q (date %s)\n",
			rev.Author(), rev.Type(), rev.Text(), rev.Date())
	}

	fmt.Printf("\nDone. Top-level paragraphs: %d\n", len(reopened.Paragraphs()))
}
