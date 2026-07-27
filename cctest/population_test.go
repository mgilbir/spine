package cctest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeCorpus builds a corpus tree under a temp root: every type in dirs gets a
// directory, and every entry in files (type -> count) gets that many files.
func makeCorpus(t *testing.T, dirs []string, files map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for _, typ := range dirs {
		if err := os.MkdirAll(filepath.Join(root, typ), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for typ, n := range files {
		for i := 0; i < n; i++ {
			name := filepath.Join(root, typ, strings.Repeat("a", 15)+string(rune('a'+i))+"."+typ)
			if err := os.WriteFile(name, []byte("not really a package"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// TestCorpusPopulationGuard is the C443 guard. TestCCCorpus's only precondition
// used to be that the corpus root stats, but ccfetch creates <root>/<type> for
// every type before fetching anything — so an aborted fetch left a root that
// exists, globs to nothing, runs zero subtests, and exits 0. Every case below
// where the corpus is present-but-useless must produce a defect.
func TestCorpusPopulationGuard(t *testing.T) {
	tests := []struct {
		name       string
		dirs       []string
		files      map[string]int
		wantDefect bool
		wantMsg    string
	}{
		{
			// Exactly what `make fetch-cc` leaves behind when it is interrupted
			// before storing its first file.
			name:       "fetch aborted at row 0",
			dirs:       docTypes,
			wantDefect: true,
			wantMsg:    "holds no corpus files",
		},
		{
			name:       "root exists with no type dirs at all",
			wantDefect: true,
			wantMsg:    "holds no corpus files",
		},
		{
			name:       "fetch aborted after the first type",
			dirs:       docTypes,
			files:      map[string]int{"pptx": 3},
			wantDefect: true,
			wantMsg:    "partially fetched",
		},
		{
			name:  "fully populated",
			dirs:  docTypes,
			files: map[string]int{"pptx": 2, "xlsx": 2, "docx": 2},
		},
		{
			// A deliberately single-type corpus: the other directories were
			// never created, so there is nothing half-done about it.
			name:  "single-type corpus with no empty dirs",
			dirs:  []string{"docx"},
			files: map[string]int{"docx": 2},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := makeCorpus(t, tc.dirs, tc.files)
			pop, err := scanCorpus(root)
			if err != nil {
				t.Fatal(err)
			}
			defect := pop.defect(root)
			switch {
			case tc.wantDefect && defect == nil:
				t.Fatalf("corpus present but unusable went unreported; the run would have "+
					"passed with zero subtests (dirs=%v files=%v)", tc.dirs, tc.files)
			case !tc.wantDefect && defect != nil:
				t.Fatalf("usable corpus rejected: %v", defect)
			case tc.wantDefect && !strings.Contains(defect.Error(), tc.wantMsg):
				t.Errorf("defect %q does not mention %q", defect, tc.wantMsg)
			}
		})
	}
}

// TestCorpusPopulationScanFindsFiles pins that the scan the test body now
// consumes really enumerates the files (a scan that silently returned nothing
// would make the guard fire on a good corpus, or worse, be routed around).
func TestCorpusPopulationScanFindsFiles(t *testing.T) {
	root := makeCorpus(t, docTypes, map[string]int{"pptx": 2, "xlsx": 1, "docx": 3})
	pop, err := scanCorpus(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"pptx": 2, "xlsx": 1, "docx": 3}
	for typ, n := range want {
		if got := len(pop.files[typ]); got != n {
			t.Errorf("scanCorpus found %d %s files, want %d", got, typ, n)
		}
		if !pop.dirs[typ] {
			t.Errorf("scanCorpus did not record the %s directory as present", typ)
		}
	}
}
