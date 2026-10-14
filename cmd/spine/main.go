// Command spine is an agent-first CLI that inspects and edits Office files
// (XLSX and PPTX in this slice): info, ls, text, cells and set. See
// docs/design/spine-cli-plan.md.
//
// It exits 0 on success and for help, 1 on any runtime failure, and 2 when
// flags or the command line cannot be parsed. Errors go to standard error
// prefixed "spine: ", or as a single-line JSON object after --json.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const usage = `spine — inspect and edit Office files

Usage:

  spine info <file>
      Summarize a file: kind, sheets or slides, title and creator.

      Flags: --password STRING   decrypt a password-encrypted file

  spine ls <file> <scope>
      List addressable children. Scopes: sheets (xlsx), slides (pptx),
      slide/<N> (pptx: that slide's top-level shapes).

      Flags: --password STRING   decrypt a password-encrypted file

  spine text <file>
      Print the document's plain text.

      Flags: --sheet NAME   xlsx: restrict to one sheet
             --slide N      pptx: restrict to one slide
             --password STRING   decrypt a password-encrypted file

  spine cells <file> <sheet> <range>
      Print the present cells of an xlsx range, row-major.

      Flags: --password STRING   decrypt a password-encrypted file

  spine set <file> <selector> <value>
      Apply one write and save atomically. Selectors: Sheet1!B2 (xlsx),
      slide/<N>/title, slide/<N>/body, slide/<N>/shape/<ID> (pptx).

      Flags: --type STRING    number|string|boolean|formula|date (xlsx);
                              not accepted for pptx text
             --out PATH       save to PATH instead of overwriting the input
             --password STRING  decrypt (and re-encrypt the output)

  spine help
      Show this text.

Every command accepts --json for machine-readable output on stdout. Exit
codes: 0 success or help, 1 failure, 2 bad command line.
`

// cliError is a runtime failure with a machine-readable code.
type cliError struct {
	code    string
	message string
}

func (e *cliError) Error() string { return e.message }

func errf(code, format string, args ...any) *cliError {
	return &cliError{code: code, message: fmt.Sprintf(format, args...)}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes one command line and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	defaultJSON = false
	// A global --json before the command name sets the default for it.
	for len(args) > 0 && (args[0] == "--json" || args[0] == "-json") {
		defaultJSON = true
		args = args[1:]
	}
	if len(args) == 0 || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "info":
		return runCmd("info", rest, stdout, stderr, func(fs *flagSetT) *commonFlags {
			var c commonFlags
			c.addCommon(fs, false, true)
			return &c
		}, func(fs *flagSetT, stdout io.Writer) error { return runInfo(fs, stdout) })
	case "ls":
		return runCmd("ls", rest, stdout, stderr, func(fs *flagSetT) *commonFlags {
			var c commonFlags
			c.addCommon(fs, false, true)
			return &c
		}, func(fs *flagSetT, stdout io.Writer) error { return runLs(fs, stdout) })
	case "text":
		return runCmd("text", rest, stdout, stderr, func(fs *flagSetT) *commonFlags {
			var c commonFlags
			c.addCommon(fs, false, true)
			c.addTextFlags(fs)
			return &c
		}, func(fs *flagSetT, stdout io.Writer) error { return runText(fs, stdout) })
	case "cells":
		return runCmd("cells", rest, stdout, stderr, func(fs *flagSetT) *commonFlags {
			var c commonFlags
			c.addCommon(fs, false, true)
			return &c
		}, func(fs *flagSetT, stdout io.Writer) error { return runCells(fs, stdout) })
	case "set":
		return runCmd("set", rest, stdout, stderr, func(fs *flagSetT) *commonFlags {
			var c commonFlags
			c.addCommon(fs, true, true)
			c.addSetFlags(fs)
			return &c
		}, func(fs *flagSetT, stdout io.Writer) error { return runSet(fs, stdout) })
	default:
		fmt.Fprintf(stderr, "spine: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

// defaultJSON is set when --json appears before the command name; the flag
// then defaults to true.
var defaultJSON bool

// guard runs fn, recovering any panic and reporting it as an internal error
// with exit 1. It is the production panic path for every command.
func guard(jsonMode bool, stderr io.Writer, fn func() int) (exit int) {
	defer func() {
		if r := recover(); r != nil {
			emitError(stderr, jsonMode, errf("internal", "internal error: %v", r))
			exit = 1
		}
	}()
	return fn()
}

// runCmd runs one command: flag parsing (exit 2 on failure, usage on stdout
// for --help), panic recovery (internal, exit 1), and error emission honoring
// the command's --json flag.
func runCmd(name string, args []string, stdout, stderr io.Writer, add func(fs *flagSetT) *commonFlags, exec func(fs *flagSetT, stdout io.Writer) error) (exit int) {
	fs := newFlagSet(name)
	common := add(fs)
	if defaultJSON {
		fs.Set("json", "true")
		common.json = true
	}
	fs.common = common
	if err := fs.parse(args); err != nil {
		if _, ok := err.(helpSignal); ok {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "spine: %s\n", err)
		return 2
	}
	return guard(common.json, stderr, func() int {
		if err := exec(fs, stdout); err != nil {
			var fe *flagError
			if asFlagError(err, &fe) {
				fmt.Fprintf(stderr, "spine: %s\n\n%s", fe.message, usage)
				return 2
			}
			var ce *cliError
			if asCLIError(err, &ce) {
				emitError(stderr, common.json, ce)
			} else {
				emitError(stderr, common.json, &cliError{code: "internal", message: err.Error()})
			}
			return 1
		}
		return 0
	})
}

// emitError writes one error to stderr: prefixed text, or a single-line JSON
// object after --json.
func emitError(stderr io.Writer, jsonMode bool, e *cliError) {
	if jsonMode {
		b, _ := marshalJSON(struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}{Error: e.message, Code: e.code})
		fmt.Fprintf(stderr, "%s\n", b)
		return
	}
	fmt.Fprintf(stderr, "spine: %s\n", e.message)
}

// marshalJSON encodes v deterministically, leaving HTML characters such as <
// unescaped so selectors and messages survive byte for byte.
func marshalJSON(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n")), nil
}

func asFlagError(err error, target **flagError) bool {
	for e := err; e != nil; {
		if fe, ok := e.(*flagError); ok {
			*target = fe
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func asCLIError(err error, target **cliError) bool {
	for e := err; e != nil; {
		if ce, ok := e.(*cliError); ok {
			*target = ce
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

// writeJSON prints v as deterministic indented JSON with a trailing newline.
func writeJSON(stdout io.Writer, v any) error {
	b, err := marshalJSONIndent(v)
	if err != nil {
		return errf("internal", "encode output: %v", err)
	}
	fmt.Fprintf(stdout, "%s\n", b)
	return nil
}

// marshalJSONIndent encodes v indented, with HTML characters unescaped.
func marshalJSONIndent(v any) ([]byte, error) {
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n")), nil
}

// textPreview collapses newlines to spaces and truncates to 80 runes,
// appending "…" when truncated.
func textPreview(s string) string {
	const max = 80
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", "\n"), "\n", " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
