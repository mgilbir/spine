package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

// TestSelectWorkNoDohSkipNotRetired guards C332: a run with no DoH resolver
// must not permanently retire live references. It selects them only as skips
// that are never ledgered, so a later resolver-configured run still picks them.
func TestSelectWorkNoDohSkipNotRetired(t *testing.T) {
	refs := []Ref{
		{Digest: "W1", Kind: kindWARC},
		{Digest: "L1", Kind: kindLive},
		{Digest: "L2", Kind: kindLive},
	}
	none := func(string) bool { return false }

	// No resolver: the WARC row is selected, the two live rows are passed over
	// as no-DoH skips WITHOUT being selected or consuming budget.
	sel := selectWork(refs, 10, false /*hasDoh*/, none, none)
	if got := digests(sel.work); len(got) != 1 || got[0] != "W1" {
		t.Fatalf("no-doh selection work = %v, want [W1]", got)
	}
	if sel.noDohSkips != 2 {
		t.Fatalf("no-doh skips = %d, want 2", sel.noDohSkips)
	}
	if len(sel.exhausted) != 0 {
		t.Fatalf("no-doh exhausted = %v, want none", digests(sel.exhausted))
	}

	// The no-doh run ledgered only the processable rows. Crucially the skipped
	// live rows are NOT marked done, so a later run with a resolver configured
	// still selects them.
	done := map[string]bool{"W1": true}
	sel2 := selectWork(refs, 10, true /*hasDoh*/, func(d string) bool { return done[d] }, none)
	if got := digests(sel2.work); len(got) != 2 || got[0] != "L1" || got[1] != "L2" {
		t.Fatalf("resolver-configured re-run work = %v, want [L1 L2]", got)
	}
	if sel2.noDohSkips != 0 {
		t.Fatalf("resolver-configured re-run skips = %d, want 0", sel2.noDohSkips)
	}
}

// TestSelectWorkExhausted checks that a reference whose retry cap is already
// hit is returned for terminal retirement, not queued for another fetch.
func TestSelectWorkExhausted(t *testing.T) {
	refs := []Ref{{Digest: "W1", Kind: kindWARC}, {Digest: "W2", Kind: kindWARC}}
	none := func(string) bool { return false }
	exhausted := func(d string) bool { return d == "W2" }
	sel := selectWork(refs, 10, true, none, exhausted)
	if got := digests(sel.work); len(got) != 1 || got[0] != "W1" {
		t.Fatalf("work = %v, want [W1]", got)
	}
	if got := digests(sel.exhausted); len(got) != 1 || got[0] != "W2" {
		t.Fatalf("exhausted = %v, want [W2]", got)
	}
}

func digests(refs []Ref) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Digest
	}
	return out
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
	if o, s, _ := interpretWorker(true, nil, "", "", "D1"); o != outcomeResource || s != "timeout" {
		t.Errorf("timeout: got (%s,%s)", o, s)
	}
	if o, _, _ := interpretWorker(false, nil, "D1\tpass\n", "", "D1"); o != outcomePass {
		t.Errorf("pass: got %s", o)
	}
	if o, s, _ := interpretWorker(false, nil, "D1\tfail\tsave\tx", "", "D1"); o != outcomeFail || s != "save" {
		t.Errorf("fail: got (%s,%s)", o, s)
	}
	if o, s, _ := interpretWorker(false, nil, "unrelated output", "", "D1"); o != outcomeResource || s != "panic" {
		t.Errorf("silent: got (%s,%s)", o, s)
	}
}

// TestInterpretWorkerPanicSignature guards C336: a crashed worker's first panic
// line is folded into the signature so two different panics cluster distinctly
// instead of collapsing into one "worker exited with status N".
func TestInterpretWorkerPanicSignature(t *testing.T) {
	exit2 := &exec.ExitError{ProcessState: fakeExit(2)}

	stderrA := "panic: runtime error: index out of range [5] with length 3\n\ngoroutine 1 [running]:\nmain.foo()\n"
	stderrB := "panic: assignment to entry in nil map\n\ngoroutine 1 [running]:\nmain.bar()\n"

	_, _, sigA := interpretWorker(false, exit2, "", stderrA, "D1")
	_, _, sigB := interpretWorker(false, exit2, "", stderrB, "D1")

	if sigA == sigB {
		t.Fatalf("distinct panics produced identical signatures: %q", sigA)
	}
	for _, sig := range []string{sigA, sigB} {
		if !strings.HasPrefix(sig, "worker exited with status 2: ") {
			t.Errorf("signature missing status/panic prefix: %q", sig)
		}
	}
	// With no stderr, the signature falls back to the bare status line.
	if _, _, sig := interpretWorker(false, exit2, "", "", "D1"); sig != "worker exited with status 2" {
		t.Errorf("no-stderr signature = %q, want bare status line", sig)
	}
}

// fakeExit returns a ProcessState reporting the given exit code, for building
// an *exec.ExitError without hand-constructing platform-specific wait status.
func fakeExit(code int) *os.ProcessState {
	cmd := exec.Command("sh", "-c", "exit "+strconv.Itoa(code))
	_ = cmd.Run()
	return cmd.ProcessState
}

// TestSpawnWorkerEndToEnd injects fake workers via /bin/sh to prove the
// orchestrator interprets a hanging, killed, panicking, or silent worker as a
// resource kill — and a well-behaved one as pass/fail — through the real
// subprocess spawn + interpret path.
func TestSpawnWorkerEndToEnd(t *testing.T) {
	ref := Ref{Digest: "DEAD", Type: "docx"}
	sh := func(script string, hard time.Duration) (outcome, stage string) {
		stdout, stderr, runErr, timedOut := spawnWorker(context.Background(), "/bin/sh",
			[]string{"-c", script}, nil, ref, hard)
		o, s, _ := interpretWorker(timedOut, runErr, stdout, stderr, ref.Digest)
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

// TestProcessRefKeepsDohURLOutOfArgv proves the orchestrator hands the DoH
// resolver URL to its workers through the environment and never on the command
// line: a private resolver profile URL carries an account token, and argv is
// world-readable via ps/proc for the worker's whole lifetime (C577).
func TestProcessRefKeepsDohURLOutOfArgv(t *testing.T) {
	const secret = "https://dns.example.net/PROFILE-TOKEN-SHOULD-NOT-LEAK"

	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	envFile := filepath.Join(dir, "env")
	// A fake worker that records exactly what it was given, then emits a valid
	// result line so processRef takes the normal pass path.
	fake := filepath.Join(dir, "fake-worker.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + argvFile + "\n" +
		"env > " + envFile + "\n" +
		"printf 'DEAD\\tpass\\n'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := orchConfig{scratchDir: dir, timeout: 2 * time.Second, dohURL: secret}
	outcome, _, _ := processRef(fake, Ref{Digest: "DEAD", Type: "docx"}, cfg)
	if outcome != outcomePass {
		t.Fatalf("fake worker outcome = %s, want %s", outcome, outcomePass)
	}

	argv, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(argv), secret) {
		t.Errorf("DoH URL leaked into worker argv (readable via /proc):\n%s", argv)
	}
	if strings.Contains(string(argv), "-doh-url") {
		t.Errorf("worker argv still carries a -doh-url flag:\n%s", argv)
	}

	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "SPINE_DOH_URL="+secret) {
		t.Errorf("worker environment lacks SPINE_DOH_URL, so live refetches would lose the resolver:\n%s", env)
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
