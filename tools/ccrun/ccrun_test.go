package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseManifestName(t *testing.T) {
	tests := []struct {
		base, typ, kind string
		wantErr         bool
	}{
		{"manifest-pptx.tsv", "pptx", kindWARC, false},
		{"manifest-docx-truncated.tsv", "docx", kindLive, false},
		{"manifest-xlsx.tsv", "xlsx", kindWARC, false},
		{"manifest-other.tsv", "", "", true},
		{"manifest-truncated.tsv", "", "", true},
	}
	for _, tt := range tests {
		typ, kind, err := parseManifestName(tt.base)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tt.base)
			}
			continue
		}
		if err != nil || typ != tt.typ || kind != tt.kind {
			t.Errorf("%s: got (%s,%s,%v), want (%s,%s,nil)", tt.base, typ, kind, err, tt.typ, tt.kind)
		}
	}
}

func TestLoadManifestColumns(t *testing.T) {
	dir := t.TempDir()
	// 5-column legacy layout.
	legacy := filepath.Join(dir, "manifest-docx.tsv")
	if err := os.WriteFile(legacy,
		[]byte("url\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n"+
			"https://e/a.docx\tcrawl-data/x.warc.gz\t10\t20\tD1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refs, err := loadManifest(legacy, "docx", kindWARC)
	if err != nil || len(refs) != 1 {
		t.Fatalf("legacy: %v, refs=%d", err, len(refs))
	}
	if refs[0].Crawl != "" || refs[0].Offset != 10 || refs[0].Length != 20 || refs[0].Digest != "D1" {
		t.Errorf("legacy row: %+v", refs[0])
	}
	// 6-column multi-crawl layout.
	multi := filepath.Join(dir, "manifest-pptx.tsv")
	if err := os.WriteFile(multi,
		[]byte("crawl\turl\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n"+
			"CC-MAIN-2026-21\thttps://e/a.pptx\tcrawl-data/y.warc.gz\t7\t8\tD2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refs, err = loadManifest(multi, "pptx", kindWARC)
	if err != nil || len(refs) != 1 {
		t.Fatalf("multi: %v, refs=%d", err, len(refs))
	}
	if refs[0].Crawl != "CC-MAIN-2026-21" || refs[0].Offset != 7 || refs[0].Digest != "D2" {
		t.Errorf("multi row: %+v", refs[0])
	}
}

func TestLoadAllRefsDedup(t *testing.T) {
	dir := t.TempDir()
	// Same digest D1 appears in two manifests; only the first survives.
	if err := os.WriteFile(filepath.Join(dir, "manifest-docx.tsv"),
		[]byte("crawl\turl\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n"+
			"C1\thttps://e/a\tf1\t1\t2\tD1\n"+
			"C1\thttps://e/b\tf2\t3\t4\tD2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest-docx-truncated.tsv"),
		[]byte("crawl\turl\twarc_filename\twarc_record_offset\twarc_record_length\tcontent_digest\n"+
			"C2\thttps://e/a\tf3\t5\t6\tD1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := resolveManifests([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	refs, dupes, err := loadAllRefs(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 2 || dupes != 1 {
		t.Fatalf("got %d refs, %d dupes; want 2, 1", len(refs), dupes)
	}
}

func TestLedgerResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.tsv")
	now := time.Unix(1700000000, 0)

	l, err := openLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append("D1", outcomePass, "", "", now); err != nil {
		t.Fatal(err)
	}
	if err := l.Append("D2", outcomeFail, "save", "boom", now); err != nil {
		t.Fatal(err)
	}
	// Simulate an interrupt: close and reopen (as a fresh invocation would).
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l2, err := openLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l2.Close() }()
	if !l2.Has("D1") || !l2.Has("D2") {
		t.Errorf("resume did not see prior rows: D1=%v D2=%v", l2.Has("D1"), l2.Has("D2"))
	}
	if l2.Has("D3") {
		t.Errorf("unexpected D3 in ledger")
	}
}

func TestSignature(t *testing.T) {
	err := errors.New("12 identical, 3 changed parts (first: word/document9.xml)")
	got := signature(err)
	want := "N identical, N changed parts (first: word/documentN.xml)"
	if got != want {
		t.Errorf("signature = %q, want %q", got, want)
	}
}

func TestParseResultLine(t *testing.T) {
	tests := []struct {
		line           string
		outcome, stage string
		sig            string
	}{
		{"D1\tpass", outcomePass, "", ""},
		{"D1\tretry\tfetch\tthrottled N", outcomeRetry, "fetch", ""},
		{"D1\tfail\tsave\tboom N", outcomeFail, "save", "boom N"},
		{"D1\tfail\tvalidate", outcomeFail, "validate", ""},
		{"D1", outcomeResource, "panic", "malformed result line"},
		{"D1\tweird", outcomeResource, "panic", "unknown outcome weird"},
	}
	for _, tt := range tests {
		o, s, sig := parseResultLine(tt.line)
		if o != tt.outcome || s != tt.stage || sig != tt.sig {
			t.Errorf("parseResultLine(%q) = (%s,%s,%s), want (%s,%s,%s)",
				tt.line, o, s, sig, tt.outcome, tt.stage, tt.sig)
		}
	}
}

func TestLastResultLine(t *testing.T) {
	stdout := "noise\nD1\tpass\ntrailing junk\n"
	line, ok := lastResultLine(stdout, "D1")
	if !ok || line != "D1\tpass" {
		t.Errorf("lastResultLine = (%q,%v)", line, ok)
	}
	if _, ok := lastResultLine("nothing here", "D1"); ok {
		t.Errorf("expected no match")
	}
}

// TestInterpretWorkerSynthetic exercises the mapping from a worker's raw exit
// signals to a ledger outcome without spawning real processes.
func TestInterpretWorkerSynthetic(t *testing.T) {
	if o, s, _ := interpretWorker(true, nil, "", "D1"); o != outcomeResource || s != "timeout" {
		t.Errorf("timeout: got (%s,%s)", o, s)
	}
	if o, _, _ := interpretWorker(false, nil, "D1\tpass\n", "D1"); o != outcomePass {
		t.Errorf("pass: got %s", o)
	}
	if o, s, _ := interpretWorker(false, nil, "D1\tfail\tsave\tx", "D1"); o != outcomeFail || s != "save" {
		t.Errorf("fail: got (%s,%s)", o, s)
	}
	if o, s, _ := interpretWorker(false, nil, "unrelated output", "D1"); o != outcomeResource || s != "panic" {
		t.Errorf("silent: got (%s,%s)", o, s)
	}
}

// TestSpawnWorkerEndToEnd injects fake workers via /bin/sh to prove the
// orchestrator interprets a hanging, killed, panicking, or silent worker as a
// resource kill — and a well-behaved one as pass/fail — through the real
// subprocess spawn + interpret path.
func TestSpawnWorkerEndToEnd(t *testing.T) {
	ref := Ref{Digest: "DEAD", Type: "docx"}
	sh := func(script string, hard time.Duration) (outcome, stage string) {
		stdout, runErr, timedOut := spawnWorker(context.Background(), "/bin/sh",
			[]string{"-c", script}, ref, hard)
		o, s, _ := interpretWorker(timedOut, runErr, stdout, ref.Digest)
		return o, s
	}

	// printf (unlike echo) reliably emits real tabs across shells.
	if o, _ := sh(`printf 'DEAD\tpass\n'`, 2*time.Second); o != outcomePass {
		t.Errorf("pass worker: got %s", o)
	}
	if o, s := sh(`printf 'DEAD\tfail\tsave\tboom\n'`, 2*time.Second); o != outcomeFail || s != "save" {
		t.Errorf("fail worker: got (%s,%s)", o, s)
	}
	if o, s := sh("sleep 5", 200*time.Millisecond); o != outcomeResource || s != "timeout" {
		t.Errorf("hang worker: got (%s,%s)", o, s)
	}
	if o, s := sh("kill -9 $$", 2*time.Second); o != outcomeResource || s != "killed" {
		t.Errorf("killed worker: got (%s,%s)", o, s)
	}
	if o, s := sh("exit 3", 2*time.Second); o != outcomeResource || s != "panic" {
		t.Errorf("nonzero worker: got (%s,%s)", o, s)
	}
	if o, s := sh("exit 0", 2*time.Second); o != outcomeResource || s != "panic" {
		t.Errorf("silent worker: got (%s,%s)", o, s)
	}
}

// TestTestBytesRejectsNonOOXML checks the round-trip harness attributes a
// non-package payload to the open stage.
func TestTestBytesRejectsNonOOXML(t *testing.T) {
	r := testBytes("docx", []byte("not a zip"))
	if r.ok || r.stage != "open" {
		t.Errorf("got ok=%v stage=%s, want stage=open", r.ok, r.stage)
	}
}
