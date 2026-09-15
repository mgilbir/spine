package docx

import (
	"bytes"
	"sync"
	"testing"
)

// TestConcurrentReadsOfADocument pins that reading a document from several
// goroutines is safe. The main part is parsed lazily on first access, so the
// goroutines race to be first to touch it — pre-warming would hide the case
// that matters.
//
// It does not make a document safe to modify concurrently, which it is not.
//
// The assertion is the race detector: this test only means something under
// -race, a required gate (make test-race).
func TestConcurrentReadsOfADocument(t *testing.T) {
	d := Create()
	for i := 0; i < 40; i++ {
		p := d.AddParagraph()
		p.AddRun().SetText("paragraph text")
	}
	d.AddTable(3, 3)
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, p := range reopened.Paragraphs() {
				_ = p.Text()
				_ = p.Runs()
			}
			_ = reopened.Tables()
			_ = reopened.Text()
		}()
	}
	wg.Wait()

	if got := len(reopened.Paragraphs()); got < 40 {
		t.Fatalf("paragraph count = %d, want at least 40", got)
	}
}
