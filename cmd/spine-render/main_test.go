package main

import (
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

func TestRenderInputFiles(t *testing.T) {
	dir := t.TempDir()
	d := docx.Create()
	s := d.DefaultSection()
	s.SetPageSize(200, 150)
	s.SetMargins(docx.PageMargins{Top: 10, Bottom: 10, Left: 10, Right: 10})
	for i := 0; i < 2; i++ {
		p := d.AddParagraph()
		p.SetAlignment(docx.AlignmentLeft)
		p.SetLineSpacingExact(18)
		p.SetSpaceBefore(0)
		p.SetSpaceAfter(0)
		if i == 1 {
			p.SetPageBreakBefore(true)
		}
		r := p.AddRun()
		r.SetText("Page")
		r.SetFont("Calibri")
		r.SetFontSize(12)
		r.SetBold(false)
		r.SetItalic(false)
		r.SetStrike(false)
		r.SetColor("000000")
	}
	if err := d.Save(filepath.Join(dir, "in.docx")); err != nil {
		t.Fatal(err)
	}
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	p.AddSlideFromLayout(layout)
	p.AddSlideFromLayout(layout)
	if err = p.Save(filepath.Join(dir, "in.pptx")); err != nil {
		t.Fatal(err)
	}
	w := xlsx.Create()
	sheet, err := w.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err = sheet.SetColWidth(1, 16); err != nil {
		t.Fatal(err)
	}
	if err = sheet.SetRowHeight(1, 24); err != nil {
		t.Fatal(err)
	}
	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	cell.SetString("Cell")
	if err = w.Save(filepath.Join(dir, "in.xlsx")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ext, prefix string
		count       int
	}{{"docx", "page", 2}, {"pptx", "slide", 2}, {"xlsx", "sheet", 1}} {
		t.Run(tc.ext, func(t *testing.T) {
			out := filepath.Join(dir, tc.ext)
			c := config{input: filepath.Join(dir, "in."+tc.ext), out: out, format: "both", dpi: 96, maxPages: 10, work: 1 << 30, timeout: time.Minute, fallback: true}
			if tc.ext == "xlsx" {
				c.cellRange = "A1:A2"
			}
			if err := run(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(out)
			if err != nil || len(files) != tc.count*2 {
				t.Fatalf("outputs: %v %v", files, err)
			}
			data, err := os.ReadFile(filepath.Join(out, tc.prefix+"-0001.png"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = png.Decode(strings.NewReader(string(data))); err != nil {
				t.Fatal(err)
			}
			if err = run(context.Background(), c); err == nil {
				t.Fatal("existing outputs overwritten")
			}
			after, err := os.ReadFile(filepath.Join(out, tc.prefix+"-0001.png"))
			if err != nil || string(after) != string(data) {
				t.Fatal("existing PNG changed")
			}
		})
	}
	c := config{input: filepath.Join(dir, "in.docx"), out: filepath.Join(dir, "limited"), format: "png", dpi: 96, maxPages: 1, work: 1 << 30, timeout: time.Minute, fallback: true}
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "max-pages") {
		t.Fatalf("page limit: %v", err)
	}
	c.maxPages = 10
	c.out = filepath.Join(dir, "missing-font")
	c.fallback = false
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "unresolved font") {
		t.Fatalf("font: %v", err)
	}
	c.input = filepath.Join(dir, "in.xlsx")
	c.cellRange = ""
	c.out = filepath.Join(dir, "no-range")
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "requires -range") {
		t.Fatalf("range: %v", err)
	}
}
