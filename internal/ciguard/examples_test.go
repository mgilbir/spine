package ciguard_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Every program under examples/ runs to a clean exit.
//
// CI only ever built the examples, so an example that compiled but failed at
// run time stayed broken unnoticed: examples/docx_report exited 1 from v0.2.0
// to v0.4.0, refused by the save validator because replacing a footer dropped
// a part another section still referenced. The examples are what the README
// sends a newcomer to first, and each one reopens its own output to check it,
// so running them is a cheap end-to-end test as well as a docs guard.
//
// Each example writes its output relative to the working directory, so each
// runs in its own temporary directory and nothing lands in the tree.
func TestExamplesRun(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs every example")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}
	root := repoRoot(t)

	entries, err := os.ReadDir(filepath.Join(root, "examples"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("found no examples; the guard is looking in the wrong place")
	}

	bin := t.TempDir()
	build := exec.Command(goBin, "build", "-o", bin+string(filepath.Separator), "./examples/...")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the examples: %v\n%s", err, out)
	}

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, filepath.Join(bin, name))
			cmd.Dir = t.TempDir()
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("examples/%s failed: %v\n%s", name, err, out)
			}
		})
	}
}
