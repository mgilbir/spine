package docx_test

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/docx"
)

// Example_openAndModify shows the open-edit-save cycle entirely in memory: an
// existing document (here built and serialized first) is opened with
// OpenReader, a paragraph is rewritten and another appended, and the result is
// saved and reopened to read the text back.
func Example_openAndModify() {
	src := docx.Create()
	src.AddParagraphWithText("Status: draft")
	original, err := src.SaveBytes()
	if err != nil {
		panic(err)
	}

	doc, err := docx.OpenReader(bytes.NewReader(original), int64(len(original)))
	if err != nil {
		panic(err)
	}
	doc.Paragraphs()[0].SetText("Status: final")
	doc.AddParagraphWithText("Approved by the board.")

	edited, err := doc.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := docx.OpenReader(bytes.NewReader(edited), int64(len(edited)))
	if err != nil {
		panic(err)
	}
	for _, p := range reopened.Paragraphs() {
		fmt.Println(p.Text())
	}
	// Output:
	// Status: final
	// Approved by the board.
}

// ExampleDocument_ReplaceText fills template placeholders. The "{{name}}" key
// is split across two runs, as Word often leaves it; the match still succeeds,
// and the replacement takes the formatting of the first run it touches.
func ExampleDocument_ReplaceText() {
	doc := docx.Create()
	p := doc.AddParagraph()
	p.AddText("Dear {{na")
	p.AddText("me}}, your order {{order}} has shipped.")

	doc.ReplaceText(map[string]string{
		"{{name}}":  "Ada",
		"{{order}}": "#1042",
	})

	fmt.Println(p.Text())
	// Output: Dear Ada, your order #1042 has shipped.
}
