package main

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mgilbir/spine/internal/renderfonts"
	"github.com/mgilbir/spine/pptx/pptxrender"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx/xlsxrender"
)

// Rendering budgets and DPI copied from spine-render's defaults: /render runs
// with the same budgets as `spine-render`'s flag defaults. See
// cmd/spine-render/main.go for the reasoning behind each figure.
const (
	serveRenderDPI  = 288
	serveEdgeChecks = 4 << 30
	serveMaxPixels  = 64 << 20
	serveMaxDim     = 16384
	servePixelVisit = 1 << 30
	serveMaxOutput  = 256 << 20
	servePathSegs   = 8 << 20
	serveImagePix   = 64 << 20
)

// serveRenderTimeout bounds one /render request; chartRenderer gets the same
// bound because it cannot be interrupted sooner.
const serveRenderTimeout = 60 * time.Second

// maxBodyBytes caps every request body; larger bodies get 413.
const maxBodyBytes = 8 << 20

// renderCacheMaxBytes bounds the render cache's encoded bytes (LRU).
const renderCacheMaxBytes = 64 << 20

// serveOptions is the parsed `spine serve` command line.
type serveOptions struct {
	path         string
	port         int
	portGiven    bool
	token        string
	fonts        renderfonts.Flags
	fallbackNoto bool
	journal      string
	out          string
	signals      <-chan os.Signal // nil uses SIGINT/SIGTERM; tests inject
}

// ---- serve command entry ----

// runServeCmd is the serve command's top level: flag parsing (exit 2 on
// failure), then the server loop. Exit 0 on success, help and a clean
// shutdown; 1 on any failure and on a dirty shutdown; 2 on a bad command
// line.
func runServeCmd(args []string, stdout, stderr io.Writer) (exit int) {
	fs := newFlagSet("serve")
	var opts serveOptions
	fs.StringVar(&opts.out, "out", "", "redirect /save to PATH for the process lifetime")
	fs.IntVar(&opts.port, "port", 0, "port to bind on 127.0.0.1 (0 lets the OS choose)")
	fs.StringVar(&opts.token, "token", "", "require Authorization: Bearer STRING on every request")
	fs.Var(&opts.fonts, "font", "repeatable FAMILY[:regular|bold|italic|bolditalic]=FONT_FILE[#FACE] mapping")
	fs.BoolVar(&opts.fallbackNoto, "fallback-noto", false, "explicitly substitute embedded Noto Sans for unresolved regular fonts")
	fs.StringVar(&opts.journal, "journal", "", "append one JSON line per applied batch to PATH")
	if err := fs.parse(args); err != nil {
		if _, ok := err.(helpSignal); ok {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "spine: %s\n", err)
		return 2
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintf(stderr, "spine: serve expects exactly one file argument (%d given)\n\n%s", len(fs.Args()), usage)
		return 2
	}
	opts.path = fs.Args()[0]
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "port" {
			opts.portGiven = true
		}
	})
	if !opts.portGiven {
		fmt.Fprintln(stderr, "spine: --port is required (0 lets the OS choose)")
		return 2
	}
	if opts.port < 0 || opts.port > 65535 {
		fmt.Fprintf(stderr, "spine: --port must be 0-65535 (got %d)\n", opts.port)
		return 2
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "spine: internal error: %v\n", r)
			exit = 1
		}
	}()
	return serveRun(opts, stdout, stderr)
}

// listen binds a loopback TCP address; any other address is refused. There is
// no flag to disable this.
func listen(addr string) (net.Listener, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address %q: %v", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("refusing to bind non-loopback address %q; spine serve only binds 127.0.0.1", addr)
	}
	return net.Listen("tcp", addr)
}

// serveRun opens the document (under an exclusive lock), serves it until
// SIGINT/SIGTERM, and returns the process exit code: 1 with a stderr line
// when the shutdown discards unsaved changes.
func serveRun(opts serveOptions, stdout, stderr io.Writer) (exit int) {
	h, err := newServeHandler(opts)
	if err != nil {
		fmt.Fprintf(stderr, "spine: %s\n", err)
		return 1
	}
	defer h.close()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.port))
	ln, err := listen(addr)
	if err != nil {
		fmt.Fprintf(stderr, "spine: %s\n", err)
		return 1
	}
	h.setOrigin("http://" + ln.Addr().String())
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	fmt.Fprintf(stdout, "listening on http://%s\n", ln.Addr())
	sigCh := opts.signals
	if sigCh == nil {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(ch)
		sigCh = ch
	}
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(stderr, "spine: %s\n", err)
			return 1
		}
	case <-sigCh:
		_ = srv.Close()
	}
	if h.dirtyState() {
		fmt.Fprintln(stderr, "spine: unsaved changes discarded")
		return 1
	}
	return 0
}

// ---- handler ----

// serveHandler is the whole server: one document, one mutex, fixed routes.
type serveHandler struct {
	mu    sync.Mutex
	d     *document
	path  string
	dirty bool

	lock        *os.File // holds the exclusive flock for the process lifetime
	token       string
	out         string // /save redirect (--out at startup)
	password    string // the password the document was opened with, for /revert
	journalPath string
	journalFile *os.File

	serial      uint64 // bumped by every successful mutation; part of the cache key
	cache       *renderCache
	fonts       render.FontResolver
	fontFiles   []renderfonts.File
	renderOpts  render.Options
	closeCharts func() error

	ownOrigin string // "http://127.0.0.1:PORT"; Origin must match or be absent
}

// newServeHandler opens path under an exclusive non-blocking flock and builds
// the render resources. The caller owns h.close.
func newServeHandler(opts serveOptions) (*serveHandler, error) {
	lock, err := lockDocument(opts.path)
	if err != nil {
		return nil, err
	}
	d, err := openDocument(opts.path, openOptions{})
	if err != nil {
		_ = lock.Close()
		return nil, err
	}
	h := &serveHandler{
		d:           d,
		path:        opts.path,
		token:       opts.token,
		out:         opts.out,
		password:    d.password,
		journalPath: opts.journal,
		cache:       newRenderCache(),
		lock:        lock,
	}
	if opts.journal != "" {
		f, jerr := os.OpenFile(opts.journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if jerr != nil {
			h.close()
			return nil, errf("io", "cannot open journal %s: %v", opts.journal, jerr)
		}
		h.journalFile = f
	}
	fonts, files, ferr := renderfonts.Resolver([]string(opts.fonts), opts.fallbackNoto)
	if ferr != nil {
		h.close()
		return nil, errf("invalid_value", "%v", ferr)
	}
	h.fonts, h.fontFiles = fonts, files
	h.renderOpts = render.Options{Fonts: fonts, Limits: render.Limits{MaxEdgeChecks: serveEdgeChecks, MaxImagePixels: serveImagePix, MaxImageBytes: 256 << 20, MaxImages: 256,
		MaxPixels: serveMaxPixels, MaxDimension: serveMaxDim, MaxPixelVisits: servePixelVisit, MaxOutputBytes: serveMaxOutput, MaxPathSegments: servePathSegs}}
	// Best effort, like spine-render without -strict: content that cannot be
	// drawn is left out of the preview instead of failing the request. The
	// CLI reports such warnings on stderr; the server draws what it can.
	h.renderOpts.Warn = func(error) {}
	if d.kind == "pptx" {
		charts, closeCharts, cerr := chartRenderer(h.fontFiles, serveRenderTimeout)
		if cerr != nil {
			h.close()
			return nil, errf("internal", "%v", cerr)
		}
		h.renderOpts.Charts = charts
		h.closeCharts = closeCharts
	}
	return h, nil
}

// close releases the document, journal, chart renderer and lock. It is not
// serialized: only called after serving has stopped.
func (h *serveHandler) close() {
	if h.closeCharts != nil {
		_ = h.closeCharts()
	}
	if h.journalFile != nil {
		_ = h.journalFile.Close()
	}
	if h.d != nil {
		h.d.close()
	}
	if h.lock != nil {
		_ = h.lock.Close()
	}
}

// lockDocument holds an exclusive non-blocking flock on path for the
// process's lifetime; a second flocker fails with "file is locked".
func lockDocument(path string) (*os.File, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, errf("not_found", "file not found: %s", path)
		}
		return nil, errf("io", "cannot open %s: %v", path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errf("io", "cannot open %s: %v", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, errf("io", "file is locked: %s", path)
	}
	return f, nil
}

// relock re-acquires the flock on the document path after /save replaced the
// file. Best effort: the lock's job is refusing concurrent serves at startup.
func (h *serveHandler) relock() {
	if h.lock != nil {
		_ = h.lock.Close()
		h.lock = nil
	}
	if lock, err := lockDocument(h.path); err == nil {
		h.lock = lock
	}
}

func (h *serveHandler) setOrigin(origin string) { h.ownOrigin = origin }

// dirtyState reports the dirty flag (the shutdown path).
func (h *serveHandler) dirtyState() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dirty
}

// ---- routing ----

// route is one fixed route; methods is the exact set it answers.
type route struct {
	path    string
	methods []string
}

var serveRoutes = []route{
	{"/", []string{"GET"}},
	{"/health", []string{"GET"}},
	{"/info", []string{"GET"}},
	{"/ls", []string{"GET"}},
	{"/text", []string{"GET"}},
	{"/cells", []string{"GET"}},
	{"/schema", []string{"GET"}},
	{"/op", []string{"POST"}},
	{"/ops", []string{"POST"}},
	{"/save", []string{"POST"}},
	{"/revert", []string{"POST"}},
	{"/render", []string{"GET"}},
}

func (h *serveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// No CORS: cross-origin requests are rejected before anything else.
	if origin := r.Header.Get("Origin"); origin != "" && !h.sameOrigin(origin) {
		writeJSONError(w, http.StatusForbidden, errf("unsupported", "cross-origin requests are not allowed"))
		return
	}
	if h.token != "" {
		if auth := r.Header.Get("Authorization"); auth != "Bearer "+h.token {
			writeJSONError(w, http.StatusUnauthorized, errf("unsupported", "missing or invalid bearer token"))
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	for _, rt := range serveRoutes {
		if rt.path != r.URL.Path {
			continue
		}
		allowed := false
		for _, m := range rt.methods {
			if m == r.Method {
				allowed = true
			}
		}
		if !allowed {
			w.Header().Set("Allow", strings.Join(rt.methods, ", "))
			writeJSONError(w, http.StatusMethodNotAllowed, errf("unsupported", "method %s is not allowed for %s", r.Method, rt.path))
			return
		}
		h.dispatch(w, r, rt.path)
		return
	}
	writeJSONError(w, http.StatusNotFound, errf("not_found", "unknown path %q", r.URL.Path))
}

// sameOrigin reports whether origin is the server's own (the UI may be opened
// as localhost, which is still this listener).
func (h *serveHandler) sameOrigin(origin string) bool {
	if h.ownOrigin == "" {
		return false
	}
	if origin == h.ownOrigin {
		return true
	}
	return origin == strings.Replace(h.ownOrigin, "127.0.0.1", "localhost", 1)
}

// dispatch runs one route's handler. Everything touching the document is
// serialized under the handler's single mutex.
func (h *serveHandler) dispatch(w http.ResponseWriter, r *http.Request, path string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch path {
	case "/":
		h.handleUI(w)
	case "/health":
		h.handleHealth(w)
	case "/info":
		writeJSONResponse(w, http.StatusOK, docInfo(h.d))
	case "/ls":
		h.handleLs(w, r)
	case "/text":
		h.handleText(w, r)
	case "/cells":
		h.handleCells(w, r)
	case "/schema":
		writeJSONResponse(w, http.StatusOK, schemaWithReads{schemaDocument: *schemaDocumentOf(), Reads: readEndpoints})
	case "/op":
		h.handleOp(w, r)
	case "/ops":
		h.handleOps(w, r)
	case "/save":
		h.handleSave(w)
	case "/revert":
		h.handleRevert(w)
	case "/render":
		h.handleRender(w, r)
	default:
		writeJSONError(w, http.StatusNotFound, errf("not_found", "unknown path %q", path))
	}
}

// ---- response helpers ----

// writeJSONResponse writes v exactly as the CLI's --json stdout: the same
// deterministic indented encoding of the same structs.
func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	b, err := marshalJSONIndent(v)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, errf("internal", "encode output: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(b, '\n'))
}

// writeJSONError writes the CLI's error envelope with an HTTP status chosen
// by code: 400 for not_found and invalid/unsupported selectors and values,
// 500 for io and internal.
func writeJSONError(w http.ResponseWriter, status int, e *cliError) {
	writeJSONErrorOp(w, status, e, -1, "")
}

// writeJSONErrorOp is writeJSONError plus, when withOp, the operation name
// and (for batches) the 0-based op index — the edit error envelope.
func writeJSONErrorOp(w http.ResponseWriter, status int, e *cliError, opIndex int, op string) {
	if op == "" {
		writeJSONResponse(w, status, struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}{Error: e.message, Code: e.code})
		return
	}
	if opIndex < 0 {
		writeJSONResponse(w, status, struct {
			Error string `json:"error"`
			Code  string `json:"code"`
			Op    string `json:"op"`
		}{Error: e.message, Code: e.code, Op: op})
		return
	}
	writeJSONResponse(w, status, struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		OpIndex int    `json:"op_index"`
		Op      string `json:"op"`
	}{Error: e.message, Code: e.code, OpIndex: opIndex, Op: op})
}

// statusForCode maps a CLI error code to an HTTP status per the spec.
func statusForCode(code string) int {
	switch code {
	case "not_found", "invalid_selector", "invalid_value", "invalid_range", "unsupported":
		return http.StatusBadRequest
	default: // "io", "internal", anything else
		return http.StatusInternalServerError
	}
}

// writeCLIError writes a CLI error with its code-derived status. Any error
// that is not a *cliError is reported as internal.
func writeCLIError(w http.ResponseWriter, err error) {
	var ce *cliError
	if !asCLIError(err, &ce) {
		ce = &cliError{code: "internal", message: err.Error()}
	}
	writeJSONError(w, statusForCode(ce.code), ce)
}

// readBody reads a capped request body, mapping the cap to 413.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, errf("invalid_value", "request body exceeds the %d MiB limit", maxBodyBytes>>20))
			return nil, false
		}
		writeJSONError(w, http.StatusBadRequest, errf("invalid_value", "cannot read request body: %v", err))
		return nil, false
	}
	return data, true
}

// ---- handlers ----

// handleUI serves the placeholder embedded page. HTML only ever comes from
// here.
func (h *serveHandler) handleUI(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(uiHTML())
}

// healthOutput is /health's envelope.
type healthOutput struct {
	OK    bool   `json:"ok"`
	Kind  string `json:"kind"`
	Dirty bool   `json:"dirty"`
	Path  string `json:"path"`
}

// handleHealth is the readiness probe; it reads no document content.
func (h *serveHandler) handleHealth(w http.ResponseWriter) {
	writeJSONResponse(w, http.StatusOK, healthOutput{OK: true, Kind: h.d.kind, Dirty: h.dirty, Path: h.path})
}

// readEndpoints self-describes the GET endpoints for /schema.
var readEndpoints = struct {
	Health string `json:"health"`
	Info   string `json:"info"`
	Ls     string `json:"ls"`
	Text   string `json:"text"`
	Cells  string `json:"cells"`
	Render string `json:"render"`
}{
	Health: "GET /health — ok, kind, dirty, path",
	Info:   "GET /info — the `spine info --json` object",
	Ls:     "GET /ls?scope=… — the `spine ls --json` array",
	Text:   "GET /text?sheet=NAME|slide=N — {\"text\":\"…\"}",
	Cells:  "GET /cells?sheet=NAME&range=A1:D20 — the `spine cells --json` object",
	Render: "GET /render?target=…&format=png|svg — image bytes of the live document",
}

// schemaWithReads is /schema's envelope: the ops-schema document plus the GET
// endpoints' self-description.
type schemaWithReads struct {
	schemaDocument
	Reads any `json:"reads"`
}

func (h *serveHandler) handleLs(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		writeJSONError(w, http.StatusBadRequest, errf("invalid_selector", "unknown scope %q; use sheets, slides or slide/<N>", scope))
		return
	}
	out, err := docLS(h.d, scope)
	if err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (h *serveHandler) handleText(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sheet := q.Get("sheet")
	slideGiven := false
	slide := 0
	if s := q.Get("slide"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, errf("invalid_value", "slide %q is not an integer", s))
			return
		}
		if n < 1 {
			writeJSONError(w, http.StatusBadRequest, errf("invalid_value", "slide must be 1 or greater (got %d)", n))
			return
		}
		slideGiven, slide = true, n
	}
	text, err := docText(h.d, sheet, slideGiven, slide)
	if err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	writeJSONResponse(w, http.StatusOK, textOutput{Text: text})
}

func (h *serveHandler) handleCells(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sheetName, rng := q.Get("sheet"), q.Get("range")
	if sheetName == "" || rng == "" {
		writeJSONError(w, http.StatusBadRequest, errf("invalid_value", `cells expects "sheet" and "range" query parameters`))
		return
	}
	if h.d.kind != "xlsx" {
		writeJSONError(w, http.StatusBadRequest, errf("unsupported", "cells applies only to xlsx files"))
		return
	}
	out, err := docCells(h.d, sheetName, rng)
	if err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	writeJSONResponse(w, http.StatusOK, out)
}

// handleOp applies one operation object in memory.
func (h *serveHandler) handleOp(w http.ResponseWriter, r *http.Request) {
	data, ok := readBody(w, r)
	if !ok {
		return
	}
	result, failure := runOneOp(h.d, 0, data)
	if failure != nil {
		h.journal(1, failure.err.message)
		writeJSONErrorOp(w, statusForCode(failure.err.code), failure.err, -1, failure.op)
		return
	}
	h.afterMutation()
	h.journal(1, "")
	writeJSONResponse(w, http.StatusOK, result)
}

// handleOps applies a versioned operations document, all-or-nothing.
func (h *serveHandler) handleOps(w http.ResponseWriter, r *http.Request) {
	data, ok := readBody(w, r)
	if !ok {
		return
	}
	ops, err := parseOpsDoc(data)
	if err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	if ferr := applyOps(h.d, ops); ferr != nil {
		var of *opFailure
		if errors.As(ferr, &of) {
			h.journal(len(ops), of.err.message)
			writeJSONErrorOp(w, statusForCode(of.err.code), of.err, of.index, of.op)
			return
		}
		h.journal(len(ops), ferr.Error())
		writeJSONError(w, http.StatusInternalServerError, &cliError{code: "internal", message: ferr.Error()})
		return
	}
	h.afterMutation()
	h.journal(len(ops), "")
	writeJSONResponse(w, http.StatusOK, editOutput{Applied: len(ops)})
}

// afterMutation marks the document dirty, bumps the render-cache serial and
// journals a single-op batch.
func (h *serveHandler) afterMutation() {
	h.dirty = true
	h.serial++
}

// journal appends one JSON line per applied batch: timestamp, ops count and
// the first error if any. Disabled unless --journal was given; a failed
// journal write never fails the request.
func (h *serveHandler) journal(count int, errMsg string) {
	if h.journalFile == nil {
		return
	}
	line, err := marshalJSON(struct {
		Time  string `json:"time"`
		Ops   int    `json:"ops"`
		Error string `json:"error,omitempty"`
	}{Time: time.Now().UTC().Format(time.RFC3339), Ops: count, Error: errMsg})
	if err != nil {
		return
	}
	_, _ = h.journalFile.Write(append(line, '\n'))
}

// handleSave writes the document atomically: to --out when given at startup,
// else over the file the server opened. Mode-preserving per the slice 1 save.
func (h *serveHandler) handleSave(w http.ResponseWriter) {
	destination := h.path
	if h.out != "" {
		destination = h.out
	}
	if err := h.d.save(destination); err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	if destination == h.path {
		h.relock()
	}
	h.dirty = false
	writeJSONResponse(w, http.StatusOK, saveOutput{Saved: true, Path: destination})
}

// saveOutput is /save's envelope.
type saveOutput struct {
	Saved bool   `json:"saved"`
	Path  string `json:"path"`
}

// handleRevert discards the in-memory document and reloads it from disk.
// Nothing is written.
func (h *serveHandler) handleRevert(w http.ResponseWriter) {
	fresh, err := openDocument(h.path, openOptions{password: h.password})
	if err != nil {
		writeCLIError(w, err.(*cliError))
		return
	}
	h.d.close()
	h.d = fresh
	h.dirty = false
	h.serial++
	writeJSONResponse(w, http.StatusOK, revertOutput{Reverted: true})
}

// revertOutput is /revert's envelope.
type revertOutput struct {
	Reverted bool `json:"reverted"`
}

// ---- render ----

// isFontResolutionError reports whether err is a font-resolution failure.
// It duplicates errors.Is(err, render.ErrUnsupported) coverage for the pptx
// pipeline (pptxrender wraps the resolver error in the sentinel) but not for
// the xlsx one, where the resolver's error surfaces unwrapped; the message
// check covers both.
func isFontResolutionError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unresolved font")
}

// handleRender renders one target from the live in-memory document, cached
// by (target, format, edit-serial).
func (h *serveHandler) handleRender(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	target := q.Get("target")
	format := q.Get("format")
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "svg" {
		writeJSONError(w, http.StatusBadRequest, errf("invalid_value", "format must be png or svg (got %q)", format))
		return
	}
	if target == "" {
		writeJSONError(w, http.StatusBadRequest, errf("invalid_value", `render expects a "target" query parameter`))
		return
	}
	key := renderCacheKey{Target: target, Format: format, Serial: h.serial}
	if entry, ok := h.cache.get(key); ok {
		w.Header().Set("Content-Type", entry.contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(entry.data)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), serveRenderTimeout)
	defer cancel()
	page, err := h.prepareRender(ctx, target)
	if err != nil {
		// ErrUnsupported marks caller-fixable render profile violations
		// (missing explicit column widths or row heights, unsupported
		// content, unresolved fonts in the pptx pipeline). The xlsx pipeline
		// surfaces the font resolver's error unwrapped, so the message check
		// is not redundant. Sizing violations are fixable with the
		// set_column_width / set_row_height operations, so the message
		// carries that hint for agents.
		if errors.Is(err, render.ErrUnsupported) || isFontResolutionError(err) {
			writeJSONError(w, http.StatusBadRequest, errf("unsupported", "%s (a render capability limit; set_column_width and set_row_height fix sizing errors)", err))
			return
		}
		writeCLIError(w, err)
		return
	}
	var buf bytes.Buffer
	if format == "png" {
		err = page.WritePNG(ctx, &buf, serveRenderDPI)
	} else {
		err = page.WriteSVG(ctx, &buf, serveRenderDPI)
	}
	if err != nil {
		if ctx.Err() != nil {
			writeJSONError(w, http.StatusGatewayTimeout, errf("internal", "render timed out: %v", err))
			return
		}
		code, status := "internal", http.StatusInternalServerError
		// Same dual check as the prepare path above.
		if errors.Is(err, render.ErrUnsupported) || isFontResolutionError(err) {
			code, status = "unsupported", http.StatusBadRequest
		}
		writeJSONError(w, status, errf(code, "render %q: %v", target, err))
		return
	}
	ctype := "image/svg+xml"
	if format == "png" {
		ctype = "image/png"
	}
	h.cache.put(key, buf.Bytes(), ctype)
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}

// prepareRender resolves a render target: pptx slide/N, or xlsx SHEET!A1:D20.
func (h *serveHandler) prepareRender(ctx context.Context, target string) (*render.Page, error) {
	switch h.d.kind {
	case "pptx":
		sel, err := parsePptxSelector(target)
		if err != nil {
			return nil, err
		}
		if sel.kind != "slide" {
			return nil, errf("invalid_selector", "render target %q must be slide/<N> for a pptx file", target)
		}
		slide, err := resolveSlide(h.d.pres, sel.slide)
		if err != nil {
			return nil, err
		}
		return pptxrender.PrepareSlide(ctx, slide, h.renderOpts)
	default: // xlsx
		sheetName, rng, selectsNothing, err := parseCellSelector(target)
		if err != nil {
			return nil, err
		}
		if selectsNothing || rng == "" {
			return nil, errf("invalid_selector", "render target %q must be SHEET!A1:D20 for an xlsx file", target)
		}
		sheet, err := resolveSheet(h.d.wb, sheetName)
		if err != nil {
			return nil, err
		}
		return xlsxrender.PrepareRange(ctx, sheet, rng, h.renderOpts)
	}
}

// renderCacheKey is (target, format, edit-serial).
type renderCacheKey struct {
	Target string
	Format string
	Serial uint64
}

// cacheEntry is one cached encoded image.
type cacheEntry struct {
	key         renderCacheKey
	data        []byte
	contentType string
	size        int
}

// renderCache is a bounded LRU over encoded image bytes.
type renderCache struct {
	mu      sync.Mutex
	entries map[renderCacheKey]*list.Element // element.Value is *cacheEntry
	order   *list.List                       // front is the most recently used
	total   int
}

func newRenderCache() *renderCache {
	return &renderCache{entries: make(map[renderCacheKey]*list.Element), order: list.New()}
}

func (c *renderCache) get(key renderCacheKey) (*cacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.order.MoveToFront(el)
		return el.Value.(*cacheEntry), true
	}
	return nil, false
}

// put stores an encoded image, evicting the least recently used entries until
// the total fits the budget. An entry larger than the budget is not cached.
func (c *renderCache) put(key renderCacheKey, data []byte, contentType string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(data) > renderCacheMaxBytes {
		return
	}
	if el, ok := c.entries[key]; ok {
		c.total -= el.Value.(*cacheEntry).size
		c.order.Remove(el)
		delete(c.entries, key)
	}
	for c.total+len(data) > renderCacheMaxBytes && c.order.Len() > 0 {
		oldest := c.order.Back()
		entry := oldest.Value.(*cacheEntry)
		c.order.Remove(oldest)
		delete(c.entries, entry.key)
		c.total -= entry.size
	}
	entry := &cacheEntry{key: key, data: data, contentType: contentType, size: len(data)}
	c.entries[key] = c.order.PushFront(entry)
	c.total += entry.size
}
