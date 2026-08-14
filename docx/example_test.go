package docx_test

import (
	"bytes"
	"fmt"
	"log"

	"github.com/mgilbir/spine/docx"
)

// ExampleCreate mirrors the README quick start for Word: a heading, a plain
// paragraph, and a formatted run, serialized with SaveBytes and reopened from
// memory to count the paragraphs — no files touched.
func ExampleCreate() {
	doc := docx.Create()
	doc.Properties.Title = "My Document"

	doc.AddHeading("Welcome", 1)
	doc.AddParagraphWithText("This is a simple document created with Spine.")

	p := doc.AddParagraph()
	bold := p.AddRun()
	bold.SetText("Bold text")
	bold.SetBold(true)

	data, err := doc.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := docx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	defer func() { _ = reopened.Close() }()

	fmt.Println("paragraphs:", len(reopened.Paragraphs()))
	// Output: paragraphs: 3
}

// Example_comments shows the review flow: add a paragraph, comment on it, reply
// to the comment, resolve the thread, serialize, then reopen from memory and
// read the author and resolved state back.
func Example_comments() {
	doc := docx.Create()

	p := doc.AddParagraphWithText("The quick brown fox.")
	c := p.AddComment("Reviewer", "Please rephrase.")
	reply, err := c.Reply("Author", "Done.")
	if err != nil {
		log.Fatal(err)
	}
	if err := reply.Resolve(); err != nil {
		log.Fatal(err)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := docx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	defer func() { _ = reopened.Close() }()

	for _, got := range reopened.Comments() {
		fmt.Printf("%s: %q resolved=%v\n", got.Author(), got.Text(), got.Resolved())
		for _, r := range got.Replies() {
			fmt.Printf("  reply %s: %q resolved=%v\n", r.Author(), r.Text(), r.Resolved())
		}
	}
	// Output:
	// Reviewer: "Please rephrase." resolved=true
	//   reply Author: "Done." resolved=true
}
