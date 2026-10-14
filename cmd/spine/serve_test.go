package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/spine/internal/renderfonts"
	"github.com/mgilbir/spine/xlsx"
	"time"
)

// ---- helpers ----

// newTestHandler builds a real serveHandler against a fixture copy.
func newTestHandler(t *testing.T, fixture string, mutate func(*serveOptions)) *serveHandler {
	t.Helper()
	opts := serveOptions{path: copyFixture(t, fixture), port: 0, fallbackNoto: true}
	if mutate != nil {
		mutate(&opts)
	}
	h, err := newServeHandler(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.close)
	return h
}

// doReq performs one request against the handler and returns status, body and
// response headers.
func doReq(t *testing.T, h *serveHandler, method, path, body string, headers map[string]string) (int, []byte, http.Header) {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	res := rec.Result()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, data, res.Header
}

// compact normalizes a JSON body so substring checks survive indentation.
func compact(t *testing.T, body []byte) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		t.Fatalf("body is not JSON: %v: %s", err, body)
	}
	return buf.String()
}

// cliJSON runs the CLI itself and returns its stdout (the --json envelope).
func cliJSON(t *testing.T, args ...string) string {
	t.Helper()
	out, errOut, code := runCLI(args...)
	if code != 0 {
		t.Fatalf("spine %s: exit %d, stderr %s", strings.Join(args, " "), code, errOut)
	}
	return out
}

// assertEnvelopeEqual asserts the server body equals the CLI's --json output
// byte for byte (same structs, same encoding).
func assertEnvelopeEqual(t *testing.T, serverBody []byte, cliOut string, what string) {
	t.Helper()
	want := strings.TrimSuffix(cliOut, "\n")
	if got := strings.TrimSuffix(string(serverBody), "\n"); got != want {
		t.Fatalf("%s:\nserver: %s\ncli:    %s", what, got, want)
	}
}

// ---- reads ----

func TestServeReadsMatchCLI(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	p := h.path
	for _, tc := range []struct{ name, cliPath, servePath string }{
		{"health", "", "/health"},
		{"info", "info $F", "/info"},
		{"ls sheets", "ls $F sheets", "/ls?scope=sheets"},
		{"text", "text $F", "/text"},
		{"text sheet", "text $F --sheet Summary", "/text?sheet=Summary"},
		{"cells", "cells $F Data A1:C2", "/cells?sheet=Data&range=A1:C2"},
		{"cells single", "cells $F Data B2", "/cells?sheet=Data&range=B2"},
	} {
		if tc.name == "health" {
			status, body, _ := doReq(t, h, "GET", "/health", "", nil)
			if status != 200 || !strings.Contains(compact(t, body), `"ok":true`) || !strings.Contains(compact(t, body), `"kind":"xlsx"`) || !strings.Contains(compact(t, body), `"dirty":false`) {
				t.Fatalf("health: %d %s", status, body)
			}
			continue
		}
		args := []string{"--json"}
		for _, a := range strings.Fields(tc.cliPath) {
			if a == "$F" {
				a = p
			}
			args = append(args, a)
		}
		cli := cliJSON(t, args...)
		status, body, _ := doReq(t, h, "GET", tc.servePath, "", nil)
		if status != 200 {
			t.Fatalf("%s: status %d body %s", tc.name, status, body)
		}
		assertEnvelopeEqual(t, body, cli, tc.name)
	}
}

func TestServeReadsPPTX(t *testing.T) {
	h := newTestHandler(t, pptxPath, nil)
	p := h.path
	for _, tc := range []struct{ name, cliPath, servePath string }{
		{"info", "info $F", "/info"},
		{"ls slides", "ls $F slides", "/ls?scope=slides"},
		{"ls slide", "ls $F slide/1", "/ls?scope=slide/1"},
		{"text", "text $F", "/text"},
		{"text slide", "text $F --slide 2", "/text?slide=2"},
	} {
		args := []string{"--json"}
		for _, a := range strings.Fields(tc.cliPath) {
			if a == "$F" {
				a = p
			}
			args = append(args, a)
		}
		cli := cliJSON(t, args...)
		status, body, _ := doReq(t, h, "GET", tc.servePath, "", nil)
		if status != 200 {
			t.Fatalf("%s: status %d body %s", tc.name, status, body)
		}
		assertEnvelopeEqual(t, body, cli, tc.name)
	}
}

func TestServeReadErrors(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	for _, tc := range []struct {
		name, path, code string
		status           int
	}{
		{"ls missing scope", "/ls", "invalid_selector", 400},
		{"ls unknown scope", "/ls?scope=nope", "invalid_selector", 400},
		{"ls wrong kind", "/ls?scope=slides", "unsupported", 400},
		{"text unknown sheet", "/text?sheet=Nope", "not_found", 400},
		{"text slide on xlsx", "/text?slide=1", "invalid_value", 400},
		{"cells unknown sheet", "/cells?sheet=Nope&range=A1:B2", "not_found", 400},
		{"cells bad range", "/cells?sheet=Data&range=NOPE", "invalid_range", 400},
		{"cells missing params", "/cells?sheet=Data", "invalid_value", 400},
	} {
		status, body, _ := doReq(t, h, "GET", tc.path, "", nil)
		if status != tc.status || !strings.Contains(string(body), `"`+tc.code+`"`) {
			t.Fatalf("%s: status %d body %s (want %d, code %s)", tc.name, status, body, tc.status, tc.code)
		}
	}
}

// ---- operations ----

func TestServeOpMatchesSet(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	// The op result envelope equals `spine set --json` on the same fixture.
	cli := cliJSON(t, "--json", "set", copyFixture(t, xlsxPath), "Data!D4", "42", "--type", "number")
	status, body, _ := doReq(t, h, "POST", "/op", `{"op":"set_cell","selector":"Data!D4","value":42,"type":"number"}`, nil)
	if status != 200 {
		t.Fatalf("status %d body %s", status, body)
	}
	assertEnvelopeEqual(t, body, cli, "set_cell")
	// The read reflects the in-memory edit before any save.
	_, body, _ = doReq(t, h, "GET", "/cells?sheet=Data&range=D4", "", nil)
	if !strings.Contains(compact(t, body), `"value":42`) {
		t.Fatalf("read after op: %s", body)
	}
	// Nothing reached disk.
	disk, err := os.ReadFile(h.path)
	if err != nil {
		t.Fatal(err)
	}
	orig, _ := os.ReadFile(xlsxPath)
	if bytes.Equal(disk, orig) {
		// The copy starts equal; a save changes bytes, so equality here means
		// the file is untouched. (set_cell above did not save.)
	} else {
		t.Fatal("the in-memory op reached the disk")
	}
	if _, body, _ := doReq(t, h, "GET", "/health", "", nil); !strings.Contains(compact(t, body), `"dirty":true`) {
		t.Fatalf("health after op: %s", body)
	}
}

func TestServeOpErrors(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	for _, tc := range []struct {
		name, body, code string
		status           int
		wantOp           string
	}{
		{"unknown op", `{"op":"frobnicate"}`, "invalid_value", 400, "frobnicate"},
		{"unknown field", `{"op":"set_cell","selector":"Data!A1","valu":1}`, "invalid_value", 400, "set_cell"},
		{"missing field", `{"op":"set_cell","value":1}`, "invalid_value", 400, "set_cell"},
		{"unknown sheet", `{"op":"set_cell","selector":"Nope!A1","value":1}`, "not_found", 400, "set_cell"},
		{"null value", `{"op":"set_cell","selector":"Data!A1","value":null}`, "invalid_value", 400, "set_cell"},
		{"wrong kind", `{"op":"set_text","selector":"slide/1/title","text":"x"}`, "unsupported", 400, "set_text"},
		{"not an object", `["nope"]`, "invalid_value", 400, ""},
		{"merged cell", `{"op":"set_cell","selector":"Summary!B4","value":1}`, "invalid_value", 400, "set_cell"},
	} {
		status, body, _ := doReq(t, h, "POST", "/op", tc.body, nil)
		if status != tc.status || !strings.Contains(string(body), `"`+tc.code+`"`) {
			t.Fatalf("%s: status %d body %s", tc.name, status, body)
		}
		if tc.wantOp != "" && !strings.Contains(compact(t, body), `"op":"`+tc.wantOp+`"`) {
			t.Fatalf("%s: body lacks op name: %s", tc.name, body)
		}
	}
}

func TestServeOpsMatchesEdit(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	opsPath := filepath.Join(t.TempDir(), "ops.json")
	if err := os.WriteFile(opsPath, []byte(`{"version":1,"ops":[{"op":"set_cell","selector":"Data!A9","value":"nine"},{"op":"set_properties","creator":"Server"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cli := cliJSON(t, "--json", "edit", copyFixture(t, xlsxPath), opsPath)
	status, body, _ := doReq(t, h, "POST", "/ops", `{"version":1,"ops":[{"op":"set_cell","selector":"Data!A9","value":"nine"},{"op":"set_properties","creator":"Server"}]}`, nil)
	if status != 200 || !strings.Contains(compact(t, body), `"applied":2`) {
		t.Fatalf("status %d body %s", status, body)
	}
	assertEnvelopeEqual(t, body, cli, "ops")
	// The whole batch is visible in memory before a save.
	_, body, _ = doReq(t, h, "GET", "/cells?sheet=Data&range=A9", "", nil)
	if !strings.Contains(string(body), "nine") {
		t.Fatalf("batch result: %s", body)
	}
}

func TestServeOpsErrors(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	for _, tc := range []struct {
		name, body, code string
		status           int
	}{
		{"wrong version", `{"version":2,"ops":[{"op":"set_cell","selector":"Data!A1","value":1}]}`, "invalid_value", 400},
		{"empty ops", `{"version":1,"ops":[]}`, "invalid_value", 400},
		{"not an object", `[1,2]`, "invalid_value", 400},
		{"second op fails", `{"version":1,"ops":[{"op":"set_cell","selector":"Data!A1","value":1},{"op":"clear_cell","selector":"Nope!A1"}]}`, "not_found", 400},
	} {
		status, body, _ := doReq(t, h, "POST", "/ops", tc.body, nil)
		if status != tc.status || !strings.Contains(string(body), `"`+tc.code+`"`) {
			t.Fatalf("%s: status %d body %s", tc.name, status, body)
		}
	}
	// The op-failure envelope carries op_index and op.
	status, body, _ := doReq(t, h, "POST", "/ops", `{"version":1,"ops":[{"op":"set_cell","selector":"Data!A1","value":1},{"op":"clear_cell","selector":"Nope!A1"}]}`, nil)
	if status != 400 || !strings.Contains(compact(t, body), `"op_index":1`) || !strings.Contains(compact(t, body), `"op":"clear_cell"`) {
		t.Fatalf("op failure envelope: %d %s", status, body)
	}
	// All-or-nothing: the first op is not visible (nothing was applied).
	_, body, _ = doReq(t, h, "GET", "/health", "", nil)
	if strings.Contains(compact(t, body), `"dirty":true`) {
		t.Fatalf("a failed batch marked the document dirty: %s", body)
	}
}

// ---- save / revert ----

func TestServeSaveAndRevert(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	orig, _ := os.ReadFile(h.path)
	doReq(t, h, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":"changed"}`, nil)
	// /revert discards the in-memory document and reloads from disk.
	status, body, _ := doReq(t, h, "POST", "/revert", "", nil)
	if status != 200 || !strings.Contains(compact(t, body), `"reverted":true`) {
		t.Fatalf("revert: %d %s", status, body)
	}
	_, body, _ = doReq(t, h, "GET", "/health", "", nil)
	if strings.Contains(compact(t, body), `"dirty":true`) {
		t.Fatalf("dirty after revert: %s", body)
	}
	_, body, _ = doReq(t, h, "GET", "/cells?sheet=Data&range=A1", "", nil)
	if strings.Contains(string(body), "changed") {
		t.Fatalf("revert did not discard the edit: %s", body)
	}
	if !bytes.Equal(orig, mustRead(t, h.path)) {
		t.Fatal("revert wrote to disk")
	}
	// /save writes the file; dirty clears; reads on disk reflect the edit.
	doReq(t, h, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":"saved"}`, nil)
	status, body, _ = doReq(t, h, "POST", "/save", "", nil)
	if status != 200 || !strings.Contains(compact(t, body), `"saved":true`) || !strings.Contains(compact(t, body), `"path":"`+h.path+`"`) {
		t.Fatalf("save: %d %s", status, body)
	}
	_, body, _ = doReq(t, h, "GET", "/health", "", nil)
	if strings.Contains(compact(t, body), `"dirty":true`) {
		t.Fatalf("dirty after save: %s", body)
	}
	cli := cliJSON(t, "--json", "cells", h.path, "Data", "A1")
	if !strings.Contains(cli, "saved") {
		t.Fatalf("save did not write the edit: %s", cli)
	}
	// /save to --out leaves the input untouched.
	h2 := newTestHandler(t, xlsxPath, func(o *serveOptions) {
		o.out = filepath.Join(t.TempDir(), "elsewhere.xlsx")
	})
	doReq(t, h2, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":"redirected"}`, nil)
	status, _, _ = doReq(t, h2, "POST", "/save", "", nil)
	if status != 200 {
		t.Fatalf("save out: %d", status)
	}
	if !bytes.Equal(orig, mustRead(t, h2.path)) {
		t.Fatal("--out save touched the input file")
	}
	if cli := cliJSON(t, "--json", "cells", h2.out, "Data", "A1"); !strings.Contains(cli, "redirected") {
		t.Fatalf("--out save missing: %s", cli)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// ---- schema and UI ----

// TestServeRenderFontErrorIsUnsupported: a render failure caused by
// unresolved fonts is an expected, caller-fixable condition, so it maps to
// code "unsupported" (HTTP 400), not "internal" (500), with the renderer's
// message text unchanged.
func TestServeRenderFontErrorIsUnsupported(t *testing.T) {
	// renderFixture has the column widths the renderer requires, but no font
	// is mapped, so rendering fails on font resolution.
	h := newTestHandler(t, renderFixture(t), func(o *serveOptions) { o.fallbackNoto = false })
	status, body, _ := doReq(t, h, "GET", "/render?target=Data!A1:B1&format=png", "", nil)
	if status != 400 || !strings.Contains(string(body), `"code": "unsupported"`) || !strings.Contains(string(body), "unresolved font") {
		t.Fatalf("font-resolution render failure: %d %s", status, body)
	}
}

func TestServeSchema(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	status, body, _ := doReq(t, h, "GET", "/schema", "", nil)
	if status != 200 {
		t.Fatalf("schema: %d", status)
	}
	if !strings.Contains(compact(t, body), `"version":1`) || !strings.Contains(compact(t, body), `"operations"`) || !strings.Contains(compact(t, body), `"reads"`) {
		t.Fatalf("schema body: %s", body)
	}
	// The schema is the CLI's ops-schema document (plus "reads").
	cli := cliJSON(t, "ops-schema")
	for _, op := range []string{"set_cell", "clear_cell", "set_text", "add_sheet", "remove_sheet", "add_slide", "remove_slide", "set_properties"} {
		if !strings.Contains(cli, `"op": "`+op+`"`) || !strings.Contains(compact(t, body), `"op":"`+op+`"`) {
			t.Fatalf("schema lacks %s", op)
		}
	}
}

// TestServeOpSetCellReportsInferredType: a set_cell without an explicit
// "type" reports the interpretation actually used — the same string
// `spine set` prints — computed from the coerced value.
func TestServeOpSetCellReportsInferredType(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	tests := []struct {
		note, body, wantType string
	}{
		{"json number", `{"op":"set_cell","selector":"Data!A1","value":42}`, "number"},
		{"numeric string", `{"op":"set_cell","selector":"Data!A1","value":"42"}`, "number"},
		{"json boolean", `{"op":"set_cell","selector":"Data!A1","value":true}`, "boolean"},
		{"boolean string", `{"op":"set_cell","selector":"Data!A1","value":"true"}`, "boolean"},
		{"formula string", `{"op":"set_cell","selector":"Data!A1","value":"=SUM(B1:B2)"}`, "formula"},
		{"plain string", `{"op":"set_cell","selector":"Data!A1","value":"hello"}`, "string"},
		{"explicit type", `{"op":"set_cell","selector":"Data!A1","value":"42","type":"string"}`, "string"},
	}
	for _, tc := range tests {
		status, body, _ := doReq(t, h, "POST", "/op", tc.body, nil)
		if status != 200 {
			t.Fatalf("%s: %d %s", tc.note, status, body)
		}
		var out struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatal(err)
		}
		if out.Type != tc.wantType {
			t.Fatalf("%s: reported type %q, want %q (body %s)", tc.note, out.Type, tc.wantType, body)
		}
	}
}

func TestServeUI(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	status, body, header := doReq(t, h, "GET", "/", "", nil)
	if status != 200 || !strings.HasPrefix(header.Get("Content-Type"), "text/html") {
		t.Fatalf("UI: %d %v", status, header)
	}
	if !strings.Contains(string(body), "<h1>spine</h1>") {
		t.Fatalf("UI page: %s", body)
	}
	// The page must mention every operation name from /schema.
	_, schema, _ := doReq(t, h, "GET", "/schema", "", nil)
	for _, m := range regexp.MustCompile(`"op":"([a-z_]+)"`).FindAllStringSubmatch(string(schema), -1) {
		if !strings.Contains(string(body), m[1]) {
			t.Fatalf("UI page lacks operation %q", m[1])
		}
	}
}

// ---- security ----

func TestServeOriginRejection(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	status, _, _ := doReq(t, h, "GET", "/health", "", map[string]string{"Origin": "http://evil.example"})
	if status != 403 {
		t.Fatalf("cross-origin: %d", status)
	}
	// The server's own origin is allowed.
	h.setOrigin("http://127.0.0.1:12345")
	status, _, _ = doReq(t, h, "GET", "/health", "", map[string]string{"Origin": "http://127.0.0.1:12345"})
	if status != 200 {
		t.Fatalf("same-origin: %d", status)
	}
	// The localhost spelling of the same listener is allowed too.
	status, _, _ = doReq(t, h, "GET", "/health", "", map[string]string{"Origin": "http://localhost:12345"})
	if status != 200 {
		t.Fatalf("localhost origin: %d", status)
	}
}

func TestServeToken(t *testing.T) {
	h := newTestHandler(t, xlsxPath, func(o *serveOptions) { o.token = "s3cret" })
	status, _, _ := doReq(t, h, "GET", "/health", "", nil)
	if status != 401 {
		t.Fatalf("no token: %d", status)
	}
	status, _, _ = doReq(t, h, "GET", "/health", "", map[string]string{"Authorization": "Bearer wrong"})
	if status != 401 {
		t.Fatalf("wrong token: %d", status)
	}
	status, body, _ := doReq(t, h, "GET", "/health", "", map[string]string{"Authorization": "Bearer s3cret"})
	if status != 200 || !strings.Contains(compact(t, body), `"ok":true`) {
		t.Fatalf("good token: %d %s", status, body)
	}
}

func TestServeMethodAndPath(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	status, body, header := doReq(t, h, "GET", "/op", "", nil)
	if status != 405 || header.Get("Allow") != "POST" {
		t.Fatalf("GET /op: %d allow %q body %s", status, header.Get("Allow"), body)
	}
	status, _, _ = doReq(t, h, "POST", "/info", "", nil)
	if status != 405 {
		t.Fatalf("POST /info: %d", status)
	}
	status, body, _ = doReq(t, h, "GET", "/nope", "", nil)
	if status != 404 || !strings.Contains(string(body), "not_found") {
		t.Fatalf("unknown path: %d %s", status, body)
	}
	status, _, _ = doReq(t, h, "GET", "/info/extra", "", nil)
	if status != 404 {
		t.Fatalf("path parameters: %d", status)
	}
}

func TestServeBodyCap(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	big := `{"op":"set_cell","selector":"Data!A1","value":"` + strings.Repeat("x", 9<<20) + `"}`
	status, _, _ := doReq(t, h, "POST", "/op", big, nil)
	if status != 413 {
		t.Fatalf("oversized body: %d", status)
	}
}

func TestServeLoopbackBindRefusal(t *testing.T) {
	if _, err := listen("0.0.0.0:0"); err == nil {
		t.Fatal("bound a non-loopback address")
	}
	if _, err := listen("192.168.1.1:0"); err == nil {
		t.Fatal("bound a non-loopback address")
	}
	ln, err := listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
}

func TestServeFileLock(t *testing.T) {
	opts := serveOptions{path: copyFixture(t, xlsxPath), port: 0}
	h, err := newServeHandler(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.close)
	if _, err := newServeHandler(opts); err == nil || !strings.Contains(err.Error(), "file is locked") {
		t.Fatalf("second serve on a locked file: %v", err)
	}
}

// ---- render ----

func TestServeRenderPNG(t *testing.T) {
	h := newTestHandler(t, pptxPath, nil)
	status, body, header := doReq(t, h, "GET", "/render?target=slide/1&format=png", "", nil)
	if status != 200 || header.Get("Content-Type") != "image/png" || len(body) < 8 || string(body[:4]) != "\x89PNG" {
		t.Fatalf("render slide: %d %v %d bytes", status, header, len(body))
	}
	// The cached second response is byte-identical.
	status2, body2, _ := doReq(t, h, "GET", "/render?target=slide/1&format=png", "", nil)
	if status2 != 200 || !bytes.Equal(body, body2) {
		t.Fatal("cached render differs")
	}
	hx := newTestHandler(t, xlsxPath, nil)
	status, body, _ = doReq(t, hx, "GET", "/render?target=slide/1", "", nil) // xlsx rejects pptx targets
	if status != 400 {
		t.Fatalf("xlsx pptx target: %d %s", status, body)
	}
	hr := newRenderHandler(t)
	status, body, header = doReq(t, hr, "GET", "/render?target=Data!A1:B1&format=png", "", nil)
	if status != 200 || header.Get("Content-Type") != "image/png" || string(body[:4]) != "\x89PNG" {
		t.Fatalf("render range: %d %v %d bytes", status, header, len(body))
	}
	status, body, header = doReq(t, hr, "GET", "/render?target=Data!A1:B1&format=svg", "", nil)
	if status != 200 || header.Get("Content-Type") != "image/svg+xml" || !strings.Contains(string(body), "<svg xmlns") {
		t.Fatalf("render svg: %d %d bytes", status, len(body))
	}
	status, body, _ = doReq(t, hr, "GET", "/render?target=Data!A1:B1&format=gif", "", nil)
	if status != 400 {
		t.Fatalf("bad format: %d %s", status, body)
	}
	status, body, _ = doReq(t, hx, "GET", "/render?target=Nope!A1:B2", "", nil)
	if status != 400 || !strings.Contains(string(body), "not_found") {
		t.Fatalf("unknown sheet target: %d %s", status, body)
	}
	status, body, _ = doReq(t, hx, "GET", "/render?target=A1:B2", "", nil)
	if status != 400 {
		t.Fatalf("sheetless target: %d %s", status, body)
	}
	// A sheet without the explicit column widths the renderer requires is a
	// render failure with the renderer's message.
	status, body, _ = doReq(t, hx, "GET", "/render?target=Data!A1:B2", "", nil)
	if status != 500 || !strings.Contains(string(body), "render") {
		t.Fatalf("unrenderable range: %d %s", status, body)
	}
}

// renderFixture is an xlsx file whose range renders visibly: explicit column
// widths (the renderer requires them) and a cell whose text the mapped test
// font draws.
func renderFixture(t *testing.T) string {
	t.Helper()
	wb := xlsx.Create()
	sheet, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	for col := 1; col <= 2; col++ {
		if err := sheet.SetColWidth(col, 8.7109375); err != nil {
			t.Fatal(err)
		}
	}
	if err := sheet.SetCellValue("A1", "Revenue"); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetCellValue("B1", 42); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "render.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// newRenderHandler serves renderFixture with the test font mapped, so cell
// text is drawn and pixel-visible.
func newRenderHandler(t *testing.T) *serveHandler {
	t.Helper()
	fontPath := filepath.Join(t.TempDir(), "test.ttf")
	glyphs := []fonttest.Glyph{{Rune: 'R', Advance: 700, HasShape: true, Ink: [4]int{0, 0, 600, 700}}, {Rune: 'M', Advance: 900, HasShape: true, Ink: [4]int{0, 0, 800, 700}}}
	for r := rune('a'); r <= rune('z'); r++ {
		glyphs = append(glyphs, fonttest.Glyph{Rune: r, Advance: 500, HasShape: true, Ink: [4]int{0, 0, 400, 700}})
	}
	for c := '0'; c <= '9'; c++ {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 500, HasShape: true, Ink: [4]int{0, 0, 400, 700}})
	}
	if err := os.WriteFile(fontPath, fonttest.SFNT(fonttest.SFNTOptions{Name: "Test", Glyphs: glyphs}), 0o600); err != nil {
		t.Fatal(err)
	}
	return newTestHandler(t, renderFixture(t), func(o *serveOptions) {
		o.fonts = renderfonts.Flags{"Calibri=" + fontPath}
	})
}

func TestServeRenderCacheInvalidation(t *testing.T) {
	h := newRenderHandler(t)
	_, first, _ := doReq(t, h, "GET", "/render?target=Data!A1:B1&format=png", "", nil)
	status, opResp, _ := doReq(t, h, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":"Mutated"}`, nil)
	if status != 200 {
		t.Fatalf("mutate: %d %s", status, opResp)
	}
	_, second, _ := doReq(t, h, "GET", "/render?target=Data!A1:B1&format=png", "", nil)
	if bytes.Equal(first, second) {
		t.Fatal("the render cache served stale bytes after a mutation")
	}
	// A revert bumps the serial again; the cache does not serve the mutated
	// image after the document is reloaded from disk.
	if status, _, _ := doReq(t, h, "POST", "/revert", "", nil); status != 200 {
		t.Fatal("revert")
	}
	_, third, _ := doReq(t, h, "GET", "/render?target=Data!A1:B1&format=png", "", nil)
	if bytes.Equal(second, third) {
		t.Fatal("revert did not invalidate the render cache")
	}
}

// ---- concurrency ----

// TestServeConcurrency drives 32 parallel /ops batches and interleaved /info
// reads through the real handler wiring. Each batch writes a distinct cell,
// so serialization is proven by every value surviving; -race must be clean.
func TestServeConcurrency(t *testing.T) {
	h := newTestHandler(t, xlsxPath, nil)
	const workers = 32
	var wg sync.WaitGroup
	errs := make(chan error, workers*2)
	for i := 1; i <= workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"version":1,"ops":[{"op":"set_cell","selector":"Data!A%d","value":%d}]}`, 10+i, i*100)
			status, resp, _ := doReq(t, h, "POST", "/ops", body, nil)
			if status != 200 || !strings.Contains(compact(t, resp), `"applied":1`) {
				errs <- fmt.Errorf("worker %d /ops: %d %s", i, status, resp)
				return
			}
			status, resp, _ = doReq(t, h, "GET", "/info", "", nil)
			if status != 200 || !strings.Contains(compact(t, resp), `"kind":"xlsx"`) {
				errs <- fmt.Errorf("worker %d /info: %d %s", i, status, resp)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	// All 32 mutations survived: none was lost to a race.
	_, body, _ := doReq(t, h, "GET", "/cells?sheet=Data&range=A11:A42", "", nil)
	for i := 1; i <= workers; i++ {
		if !strings.Contains(string(body), strconv.Itoa(i*100)) {
			t.Fatalf("mutation %d was lost: %s", i, body)
		}
	}
}

// ---- journal ----

func TestServeJournal(t *testing.T) {
	journal := filepath.Join(t.TempDir(), "journal.jsonl")
	h := newTestHandler(t, xlsxPath, func(o *serveOptions) { o.journal = journal })
	doReq(t, h, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":1}`, nil)
	doReq(t, h, "POST", "/ops", `{"version":1,"ops":[{"op":"set_cell","selector":"Data!A2","value":2},{"op":"clear_cell","selector":"Nope!A1"}]}`, nil)
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("journal lines: %q", data)
	}
	for _, line := range lines {
		if !strings.Contains(line, `"time":"`) || !strings.Contains(line, `"ops":`) {
			t.Fatalf("journal line: %s", line)
		}
	}
	if !strings.Contains(lines[0], `"ops":1`) || strings.Contains(lines[0], `"error"`) {
		t.Fatalf("first line: %s", lines[0])
	}
	if !strings.Contains(lines[1], `"ops":2`) || !strings.Contains(lines[1], `"error":"sheet not found`) {
		t.Fatalf("error line: %s", lines[1])
	}
	// Without --journal nothing is written and nothing fails.
	h2 := newTestHandler(t, xlsxPath, nil)
	status, _, _ := doReq(t, h2, "POST", "/op", `{"op":"set_cell","selector":"Data!A1","value":1}`, nil)
	if status != 200 {
		t.Fatalf("no journal: %d", status)
	}
}

// ---- lifecycle ----

// TestServeDirtyShutdown runs the real serve loop with an injected signal
// channel: dirty shutdown exits 1 with the stderr line, clean shutdown 0.
func TestServeDirtyShutdown(t *testing.T) {
	path := copyFixture(t, xlsxPath)
	var stdout, stderr syncBuffer
	signals := make(chan os.Signal, 1)
	opts := serveOptions{path: path, port: 0, signals: signals}
	code := make(chan int, 1)
	go func() { code <- serveRun(opts, &stdout, &stderr) }()

	waitForListening(t, &stdout)
	addr := listeningAddr(t, &stdout)

	// Make the document dirty over real HTTP, then signal.
	res, err := http.Post("http://"+addr+"/op", "application/json",
		strings.NewReader(`{"op":"set_cell","selector":"Data!A1","value":1}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	signals <- syscall.SIGTERM
	if got := <-code; got != 1 {
		t.Fatalf("dirty shutdown exit %d, stderr %q", got, stderr.String())
	}
	if !strings.Contains(stderr.String(), "spine: unsaved changes discarded") {
		t.Fatalf("dirty shutdown stderr %q", stderr.String())
	}

	// Clean shutdown: nothing dirty, exit 0.
	stdout.Reset()
	stderr.Reset()
	signals = make(chan os.Signal, 1)
	opts.signals = signals
	code = make(chan int, 1)
	go func() { code <- serveRun(opts, &stdout, &stderr) }()
	waitForListening(t, &stdout)
	signals <- syscall.SIGTERM
	if got := <-code; got != 0 {
		t.Fatalf("clean shutdown exit %d, stderr %q", got, stderr.String())
	}
}

// syncBuffer is a mutex-guarded buffer: serveRun writes it from its
// goroutine while the test polls it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

// startupLine matches the one startup line serveRun prints.
var startupLine = regexp.MustCompile(`listening on http://(127\.0\.0\.1:\d+)\n`)

func waitForListening(t *testing.T, stdout *syncBuffer) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if startupLine.MatchString(stdout.String()) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server never printed its startup line: %q", stdout.String())
}

func listeningAddr(t *testing.T, stdout *syncBuffer) string {
	t.Helper()
	m := startupLine.FindStringSubmatch(stdout.String())
	if m == nil {
		t.Fatalf("no listening line in %q", stdout.String())
	}
	return m[1]
}

// ---- flag handling ----

func TestServeFlagErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"no port", []string{copyFixture(t, xlsxPath)}, 2},
		{"port out of range", []string{"--port", "70000", copyFixture(t, xlsxPath)}, 2},
		{"no file", []string{"--port", "0"}, 2},
		{"two files", []string{"--port", "0", "a", "b"}, 2},
	} {
		var stdout, stderr bytes.Buffer
		if got := runServeCmd(tc.args, &stdout, &stderr); got != tc.code {
			t.Fatalf("%s: exit %d (want %d), stderr %q", tc.name, got, tc.code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if got := runServeCmd([]string{"--help"}, &stdout, &stderr); got != 0 || !strings.Contains(stdout.String(), "spine serve") {
		t.Fatalf("serve --help: exit %d", got)
	}
}
