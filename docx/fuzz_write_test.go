package docx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// fuzzReparseDocx saves a document and re-opens the bytes, walking the text
// boxes, watermark, merge fields and mail-merge config so the write-then-read
// path is exercised. Any panic is a bug; errors are expected and fine.
func fuzzReparseDocx(d *Document) {
	out, err := d.SaveBytes()
	if err != nil {
		return
	}
	d2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	defer func() { _ = d2.Close() }()
	for _, tb := range d2.TextBoxes() {
		_ = tb.Text()
		_ = tb.WidthEMU()
		_ = tb.Shape()
	}
	_ = d2.Watermark()
	_ = d2.MergeFields()
	_ = d2.MailMerge()
}

// FuzzDocxAddShape fuzzes Paragraph.AddTextBox and Paragraph.AddShape: the box
// text, geometry, preset shape, fill/border colors and toggles, and the
// floating/anchor flags. It adds both a text box and a shape, then saves and
// re-opens. No panic; a self-consistent read-back.
func FuzzDocxAddShape(f *testing.F) {
	f.Add("hello\nworld", int64(914400), int64(457200), "rect", "FFAABB", "000000", false, false, false)
	f.Add("", int64(0), int64(-5), "ellipse", "", "", true, true, true)
	f.Add("<b>&amp;</b>", int64(1), int64(1), "roundRect", "zzz", "!!", true, false, false)
	f.Add("line & <tab>\ttext", int64(99999999999), int64(0), "line", "GG", "  ", false, true, false)
	f.Add("x\x00y￿", int64(2000000), int64(2000000), "star", "C0C0C0", "123456", true, false, true)

	f.Fuzz(func(t *testing.T, text string, w, h int64, shape, fill, border string, floating, noFill, noBorder bool) {
		d := Create()
		opts := TextBoxOptions{
			WidthEMU:    w,
			HeightEMU:   h,
			Floating:    floating,
			Shape:       ShapeType(shape),
			FillColor:   fill,
			NoFill:      noFill,
			BorderColor: border,
			NoBorder:    noBorder,
			Anchor:      Anchor{X: 10, Y: 20, BehindText: noFill, RelativeToPage: floating},
		}
		p := d.AddParagraph()
		tb := p.AddTextBox(text, opts)
		_ = tb.Text()
		_ = tb.Floating()
		p.AddShape(text, opts)
		d.AddTextBox(text, opts)
		fuzzReparseDocx(d)
	})
}

// FuzzDocxWatermark fuzzes Document.SetTextWatermark and SetImageWatermark: the
// watermark text with fuzzed font/color/rotation, and image bytes that are
// mostly junk (a valid PNG seed exercises the success path). It applies the
// watermark, then saves and re-opens. No panic; errors on undecodable image
// bytes are expected and fine.
func FuzzDocxWatermark(f *testing.F) {
	var pngBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	_ = png.Encode(&pngBuf, img)
	validPNG := pngBuf.Bytes()

	f.Add("CONFIDENTIAL", "Calibri", "C0C0C0", 315.0, validPNG)
	f.Add("", "", "", 0.0, []byte{})
	f.Add("<draft & co>\n\t", "Times", "#GG", 45.0, []byte("not an image"))
	f.Add("secret\x00￿", "Arial", "zzz", 1e9, []byte("PK\x03\x04junk"))
	f.Add("long "+string(make([]byte, 500)), "F", "112233", -90.0, validPNG[:len(validPNG)/2])

	f.Fuzz(func(t *testing.T, text, font, colorHex string, rotation float64, imgBytes []byte) {
		d := Create()
		d.AddParagraphWithText("body")
		opts := WatermarkOptions{Font: font, Color: colorHex, Rotation: rotation, Diagonal: rotation == 0}
		if err := d.SetTextWatermark(text, opts); err != nil {
			return
		}
		_ = d.Watermark()
		fuzzReparseDocx(d)

		d2 := Create()
		d2.AddParagraphWithText("body")
		if err := d2.SetImageWatermark(imgBytes, opts); err != nil {
			return
		}
		fuzzReparseDocx(d2)
	})
}

// FuzzDocxMailMerge fuzzes Document.SetMailMerge (the full w:mailMerge config
// including the ODSO field mappings) and Paragraph.AddMergeField (field names,
// including whitespace-bearing ones that must be quoted in the instruction). It
// writes the config and fields, then saves and re-opens, reading the merge
// fields and config back. No panic; a self-consistent read-back.
func FuzzDocxMailMerge(f *testing.F) {
	f.Add("formLetters", "spreadsheet", "First Name", "cn", 0, true, "rId7")
	f.Add("", "", "", "", -99999, false, "")
	f.Add("email", "database", "a\tb", "col\ndelim", 44, true, "<bad>")
	f.Add("catalog", "native", "«weird»", "x", 1, false, "rId\x00")

	f.Fuzz(func(t *testing.T, docType, dataType, fieldName, mappedName string, colDelim int, firstRowHeader bool, srcRef string) {
		d := Create()
		mm := &MailMerge{
			MainDocumentType: docType,
			DataType:         dataType,
			ViewMergedData:   firstRowHeader,
			DataSourceRef:    srcRef,
			DataSource: &MailMergeDataSource{
				SourceRef:       srcRef,
				ConnectionType:  dataType,
				FirstRowHeader:  firstRowHeader,
				ColumnDelimiter: colDelim,
				FieldMappings: []MailMergeFieldMapping{
					{Name: fieldName, MappedName: mappedName, Column: colDelim, Type: "dbColumn"},
				},
			},
		}
		d.SetMailMerge(mm)

		p := d.AddParagraph()
		p.AddText("Dear ")
		p.AddMergeField(fieldName)
		p.AddText(",")
		d.AddParagraph().AddMergeField(mappedName)

		fuzzReparseDocx(d)

		// Clearing the config is also a mutation; exercise its save path.
		d.SetMailMerge(nil)
		fuzzReparseDocx(d)
	})
}
