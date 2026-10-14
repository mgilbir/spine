package main

import (
	"errors"
	"flag"
	"io"
	"strconv"
	"strings"
)

// flagSetT wraps flag.FlagSet so parse failures become exit-2 flag errors and
// --help becomes a success exit with usage on stdout.
type flagSetT struct {
	*flag.FlagSet
	common     *commonFlags
	positional []string
}

// Args returns the positional arguments left after parsing.
func (fs *flagSetT) Args() []string {
	if fs.positional != nil {
		return fs.positional
	}
	return fs.FlagSet.Args()
}

// looksNegativeNumber reports whether tok is a negative number rather than a
// flag: '-' followed by a digit or dot. No flag in this CLI begins with a
// digit, so treating such tokens as positional is unambiguous and lets
// `spine set f S!A1 -0.5` work without `--`.
func looksNegativeNumber(tok string) bool {
	return len(tok) > 1 && tok[0] == '-' && (tok[1] >= '0' && tok[1] <= '9' || tok[1] == '.')
}

// helpSignal reports that -h/--help was requested.
type helpSignal struct{}

func (helpSignal) Error() string { return "help requested" }

// flagError marks a command line that cannot be parsed (exit 2). Runtime
// errors use cliError instead.
type flagError struct{ message string }

func (e *flagError) Error() string { return e.message }

func newFlagSet(name string) *flagSetT {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return &flagSetT{FlagSet: fs}
}

// parse parses args, converting flag package failures: ErrHelp to helpSignal,
// anything else to a flagError. Flags may appear anywhere on the line: the
// positionals ahead of each flag run are collected and parsing resumes after
// them, so `spine set f S!A1 v --type number` works. A bare "--" terminates
// flag parsing: everything after it is positional (standard flag package
// semantics), so negative values can be written with
// `spine set f S!A1 -- -5`.
func (fs *flagSetT) parse(args []string) error {
	fs.positional = nil
	// Split off everything after the first bare "--" before parsing: the flag
	// package would consume the terminator itself, and the rescan loop below
	// could then no longer tell where it was. The tail is appended once flag
	// parsing is done, after the positionals of the flagged region.
	rest, tail := args, args[len(args):]
	for i, a := range args {
		if a == "--" {
			rest, tail = args[:i], args[i+1:]
			break
		}
	}
	for {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return helpSignal{}
			}
			return &flagError{message: err.Error()}
		}
		left := fs.FlagSet.Args()
		i := 0
		for i < len(left) && (left[i] == "-" || left[i] == "--" || !strings.HasPrefix(left[i], "-") || looksNegativeNumber(left[i])) {
			i++
		}
		fs.positional = append(fs.positional, left[:i]...)
		if i >= len(left) {
			fs.positional = append(fs.positional, tail...)
			return nil
		}
		rest = left[i:]
	}
}

// commonFlags collects the flags commands share.
type commonFlags struct {
	json     bool
	out      string
	password string
	setType  string
	sheet    string
	slide    int
	hasSlide bool
}

// addCommon registers --json (and --out/--password where wanted) on fs.
func (c *commonFlags) addCommon(fs *flagSetT, out, password bool) {
	fs.BoolVar(&c.json, "json", false, "emit JSON on stdout")
	if out {
		fs.StringVar(&c.out, "out", "", "save to PATH instead of the input file")
	}
	if password {
		fs.StringVar(&c.password, "password", "", "password for an encrypted file")
	}
}

// addTextFlags registers --sheet (xlsx) and --slide (pptx) for the text
// command.
func (c *commonFlags) addTextFlags(fs *flagSetT) {
	fs.StringVar(&c.sheet, "sheet", "", "xlsx sheet to restrict the text to")
	fs.IntVar(&c.slide, "slide", 0, "pptx slide to restrict the text to")
	c.hasSlide = false
}

// addSetFlags registers --type for the set command.
func (c *commonFlags) addSetFlags(fs *flagSetT) {
	fs.StringVar(&c.setType, "type", "", "force value interpretation: number, string, boolean, formula or date")
}

// parseSlideArg parses a 1-based slide number argument.
func parseSlideArg(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, errf("invalid_selector", "slide number %q is not a positive integer", s)
	}
	return n, nil
}

// typeNotAllowed is the flagError for --type on a command that rejects it.
func typeNotAllowed() error {
	return &flagError{message: "--type is not accepted for pptx text; remove it"}
}

// intFlag parses a string that must be a plain integer.
func intFlag(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errf("invalid_selector", "%q is not an integer", s)
	}
	return n, nil
}

// quoted renders s for an error message.
func quoted(s string) string { return strconv.Quote(s) }
