package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// runEdit implements `spine edit <file> <opsfile>`: apply a batch of
// operations in order over one open/save, all-or-nothing.
func runEdit(fs *flagSetT, stdout io.Writer) error {
	args := fs.Args()
	if len(args) != 2 {
		return &flagError{message: fmt.Sprintf("edit expects a file and an operations file (%d arguments given)", len(args))}
	}
	common := flagsCommon(fs)
	path, opsPath := args[0], args[1]
	data, err := readOpsSource(opsPath)
	if err != nil {
		return err
	}
	ops, perr := parseOpsDoc(data)
	if perr != nil {
		return perr
	}
	d, err := openDocument(path, openOptions{password: common.password})
	if err != nil {
		return err
	}
	defer d.close()
	if ferr := applyOps(d, ops); ferr != nil {
		return ferr // runCmd emits it with the op_index/op context
	}
	destination := d.path
	if common.out != "" {
		destination = common.out
	}
	if serr := d.save(destination); serr != nil {
		return serr
	}
	return writeJSON(stdout, editOutput{Applied: len(ops)})
}

// editOutput is the edit command's success envelope.
type editOutput struct {
	Applied int `json:"applied"`
}

// readOpsSource reads the operations document from a path, or standard input
// for "-" (the one sanctioned stdin use).
func readOpsSource(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, errf("io", "cannot read operations from stdin: %v", err)
		}
		return data, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, errf("not_found", "file not found: %s", path)
		}
		return nil, errf("io", "cannot open %s: %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errf("io", "cannot read %s: %v", path, err)
	}
	return data, nil
}

// parseOpsDoc validates the operations document envelope: version 1 and a
// non-empty ops array.
func parseOpsDoc(data []byte) ([]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, errf("invalid_value", "operations document is not a JSON object: %v", err)
	}
	vraw, ok := top["version"]
	if !ok {
		return nil, errf("invalid_value", `operations document must have "version": %d`, opsVersion)
	}
	var version float64
	if err := json.Unmarshal(vraw, &version); err != nil || version != float64(opsVersion) {
		return nil, errf("invalid_value", "unsupported operations document version; only version %d is supported", opsVersion)
	}
	opsRaw, ok := top["ops"]
	if !ok {
		return nil, errf("invalid_value", `operations document must have a non-empty "ops" array`)
	}
	var ops []json.RawMessage
	if err := json.Unmarshal(opsRaw, &ops); err != nil {
		return nil, errf("invalid_value", `"ops" must be an array of operation objects`)
	}
	if len(ops) == 0 {
		return nil, errf("invalid_value", `"ops" must be a non-empty array`)
	}
	return ops, nil
}

// emitOpError writes an operation failure to stderr: the usual error envelope
// plus op_index and op after --json, or a spine:-prefixed line naming the
// operation otherwise.
func emitOpError(stderr io.Writer, jsonMode bool, e *cliError, opIndex int, op string) {
	if jsonMode {
		b, _ := marshalJSON(struct {
			Error   string `json:"error"`
			Code    string `json:"code"`
			OpIndex int    `json:"op_index"`
			Op      string `json:"op"`
		}{Error: e.message, Code: e.code, OpIndex: opIndex, Op: op})
		_, _ = fmt.Fprintf(stderr, "%s\n", b)
		return
	}
	if op == "" {
		_, _ = fmt.Fprintf(stderr, "spine: %s (at operation %d)\n", e.message, opIndex)
		return
	}
	_, _ = fmt.Fprintf(stderr, "spine: %s (at operation %d: %s)\n", e.message, opIndex, op)
}

// runNew implements `spine new <kind> <file>`: create an empty xlsx or pptx
// file via the atomic-save path, never overwriting an existing file.
func runNew(fs *flagSetT, stdout io.Writer, title, creator string) error {
	args := fs.Args()
	if len(args) != 2 {
		return &flagError{message: fmt.Sprintf("new expects a kind (xlsx or pptx) and a file path (%d arguments given)", len(args))}
	}
	kind, file := args[0], args[1]
	if kind != "xlsx" && kind != "pptx" {
		return errf("invalid_value", "kind must be xlsx or pptx (got %q)", kind)
	}
	if _, err := os.Stat(file); err == nil {
		return errf("io", "file already exists: %s", file)
	}
	var d *document
	switch kind {
	case "xlsx":
		wb := xlsx.Create()
		// xlsx.Create() makes a sheetless workbook, which the library refuses
		// to serialize; a new file gets one default sheet like Excel does.
		if _, err := wb.AddSheet("Sheet1"); err != nil {
			return errf("internal", "cannot create default sheet: %v", err)
		}
		wb.Properties.Title = title
		wb.Properties.Creator = creator
		d = &document{kind: "xlsx", path: file, wb: wb}
	default:
		p := pptx.CreateWidescreen()
		p.Properties.Title = title
		p.Properties.Creator = creator
		d = &document{kind: "pptx", path: file, pres: p}
	}
	defer d.close()
	if err := d.save(file); err != nil {
		return err
	}
	return writeJSON(stdout, newOutput{Kind: kind, Path: file})
}

// newOutput is the new command's envelope.
type newOutput struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// runOpsSchema implements `spine ops-schema`: print the machine-readable
// description of the operation vocabulary, generated from opRegistry.
func runOpsSchema(fs *flagSetT, stdout io.Writer) error {
	if len(fs.Args()) != 0 {
		return &flagError{message: fmt.Sprintf("ops-schema takes no arguments (%d given)", len(fs.Args()))}
	}
	b, err := marshalJSON(schemaDocumentOf())
	if err != nil {
		return errf("internal", "encode schema: %v", err)
	}
	// fieldList marshals compactly (custom MarshalJSON), so re-indent the
	// whole document for readability; the bytes stay deterministic.
	var indented bytes.Buffer
	if err := json.Indent(&indented, b, "", "  "); err != nil {
		return errf("internal", "indent schema: %v", err)
	}
	if _, werr := fmt.Fprintf(stdout, "%s\n", indented.String()); werr != nil {
		return errf("io", "cannot write schema: %v", werr)
	}
	return nil
}
