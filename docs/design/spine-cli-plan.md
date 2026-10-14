# Spine CLI plan — agent-first command surface

This document specifies `cmd/spine`, a CLI for agents (human-usable too) that
creates, inspects, and edits Office files without writing code. It is a plan,
not a reference: commands graduate to a user-facing doc once implemented.

Design goals, in order: **correctness** (never corrupt, never silently
misinterpret), **security** (localhost-only server, no injection surfaces,
atomic writes), **performance** (never re-open a file per operation when a
session exists), and **self-description** (an agent can discover the full
surface from the tool itself).

## Roadmap

1. **Slice 1 (this spec): CLI read/write core** — `info`, `ls`, `text`,
   `cells`, `set` for XLSX cells and PPTX slide text. One process per
   operation, atomic save.
2. **Slice 2: shared op vocabulary** — every mutating command is sugar for a
   JSON operation; `spine edit ops.json` applies a batch atomically over one
   open/save. `spine ops-schema` prints the JSON schema of all operations.
   Specified below.
3. **Slice 3: `spine serve`** — local HTTP server holding the document open in
   memory; the same operations over `POST /op` / `POST /ops`; `GET /schema`;
   PNG previews via the render packages; a small embedded single-page UI.
4. Later: DOCX surface, templating/fill, MCP adapter over the same schema.

Slices 2 and 3 add no new operation semantics — they are transports over the
vocabulary slice 1 establishes. That is the point: one op schema, many
surfaces.

## Conventions (all commands)

- **Exit codes** (matching `spine-render`): 0 on success and for `--help`;
  1 on any runtime failure; 2 on flag-parse failure.
- **Standard error** messages are prefixed `spine: `.
- **`--json` flag on every command.** Default output is human-readable text
  (tables/indented lines); `--json` switches stdout to machine-readable JSON
  exactly as specified below. Errors go to stderr always; with `--json` the
  stderr error is a single-line JSON object:
  `{"error":"human message","code":"<code>"}` with one of these codes:
  - `not_found` — file, sheet, slide, or shape address does not exist
  - `invalid_selector` — selector does not parse
  - `invalid_value` — value cannot be coerced per the command's rules
  - `invalid_range` — malformed A1/range reference
  - `io` — file system or save failure
  - `unsupported` — file is encrypted, or feature not supported for this kind
  - `internal` — anything else (include the wrapped error text)
- **`--password STRING` is accepted by every command.** An encrypted file
  without it is `unsupported`; a wrong password is `unsupported` with a
  "wrong password" message. A command that saves re-encrypts its output with
  the same password.
- **Atomic save**: mutating commands open, mutate, and save. The save writes
  to a temporary file in the destination directory, fsyncs, then renames over
  the destination, preserving the destination's existing file mode (a newly
  created file gets the process default). `--out PATH` saves to PATH instead (parent directory must
  exist; PATH must not be an existing directory) leaving the input untouched;
  without it the input file is replaced in place. If the document opened with
  a password (`--password`), the output preserves encryption.
- **No reading of stdin for values.** All inputs are arguments or flags so
  agents cannot be tripped by stdin quirks.
- **Determinism**: identical input file + identical command ⇒ identical
  stdout, byte for byte. Never emit timestamps, durations, or map-iteration
  order.

## Selector grammar (slice 1)

Selectors address one editable thing. They are always a single argument,
quoted by the caller when they contain `!`.

### XLSX cell

```
<Sheet>!<CellRef>       e.g. Sheet1!B2   'Q3 Data'!A1
```

- `<Sheet>` is a sheet name matched **exactly** (byte for byte, as
  `Workbook.SheetByName` matches) against the workbook's sheets — the CLI
  never guesses: an agent takes names from `ls sheets` or `info`, and a
  case-insensitive fallback could silently address the wrong sheet. An
  unknown name is `not_found`, with the closest sheet name by case-insensitive
  prefix match added to the message when one exists (the hint is diagnostic
  only). It may not contain `!`,
  `[`, `]`, `'` alone (a doubled `''` inside a quoted name is not supported in
  slice 1; such names select nothing and return `not_found`), and may not be
  empty. `internal/sheetref.NeedsQuoting` names must be wrapped in single
  quotes by the caller; the parser accepts both quoted and unquoted forms.
- `<CellRef>` is a single A1 reference parsed with `xlsx.ParseCellRef`. Ranges
  are not selectors (use the `cells` command to read; slice 2 adds range ops).
- No sheet: `invalid_selector`. Unknown sheet: `not_found` (message includes
  the closest sheet name by simple prefix match when there is one).

### PPTX text targets

```
slide/<N>                      — slide N (1-based)
slide/<N>/title                — the title placeholder's text
slide/<N>/body                 — the body/content placeholder's text
slide/<N>/shape/<ID>           — the shape whose stored shape id is ID
```

- `<N>` is 1-based and must be an integer; out of range: `not_found`.
- `title`/`body` resolve via `Slide.TitlePlaceholder()` /
  `Slide.BodyPlaceholder()`; absent placeholder: `not_found` with a message
  naming which placeholder and slide.
- `<ID>` is the stored `cNvPr` id (`Shape.ID()`). Shapes created in-process
  have id 0 until save; the CLI is one-shot so every shape it lists comes from
  a loaded file and has a real id. Id 0 or an id not present on that slide:
  `not_found` with the message "shape 7 not found on slide 3; run `spine ls
  <file> slide/3` for ids". Only top-level shapes are addressable in slice 1;
  shapes nested in groups are listed (see `ls`) but selecting them is a
  non-goal until slice 2 flattens the walk.

## Commands

All examples assume `--json` output shown; the human format is for eyes only
and is not specified beyond "contains the same information". `--json` may
also appear before the command name (`spine --json info f`), where it sets
the default for that invocation.

### `spine info <file>`

Opens the file read-only in memory (never saves). Prints one JSON object:

```json
{
  "kind": "xlsx",
  "sheet_count": 3,
  "sheets": [
    {"index": 0, "name": "Data", "active": true},
    {"index": 1, "name": "Summary", "active": false}
  ],
  "title": "Q3 report",
  "creator": "Ada"
}
```

- `kind` is `xlsx` or `pptx` (from the file's content type, not its extension;
  a wrong extension is fine, an unrecognizable file is `unsupported`).
- `sheets` only for xlsx, ascending index, `active` per the workbook's active
  tab (exactly one true).
- PPTX instead: `{"kind":"pptx","slide_count":12,"slides":[{"index":1,
  "shape_count":4,"name":"Title Slide"}],"title":"...","creator":"..."}`.
  `name` is the slide's name, omitted when empty. `title`/`creator` from
  document properties, omitted when empty.
- Flags: `--password STRING` (encrypted files; without it an encrypted file is
  `unsupported`).

### `spine ls <file> <scope>`

Lists addressable children. Scopes:

- `sheets` (xlsx): array of `{"index":N,"name":"...","active":bool}`.
- `slides` (pptx): array of `{"index":N,"name":"...","shape_count":N,
  "text_preview":"first 80 runes of Slide.Text, newlines collapsed to
  spaces, \"…\" appended if truncated"}`.
- `slide/<N>` (pptx): array of that slide's top-level shapes in tree order:
  `{"id":N,"name":"...","type":"text_box|picture|table|chart|group|
  placeholder|connector|auto_shape|video|audio|diagram|ole_object",
  "placeholder":{"type":"title|body|center_title|other","idx":N},
  "text_preview":"..."}`. `placeholder` present only for placeholder shapes,
  `type` mapped from `PlaceholderType`; `text_preview` is the shape's text
  (80 runes, same truncation rule) for text-bearing shapes, `""` otherwise.
  Groups report their own preview (their concatenated children's text) and are
  not descended into.

### `spine text <file>`

Prints extracted plain text to stdout. Without `--json`, stdout is exactly the
`Text()` bytes (UTF-8, trailing newline added if absent). With `--json`,
`{"text":"..."}`.

- xlsx: `Workbook.Text()`; `--sheet NAME` restricts to one sheet's `Text()`
  (`not_found` if absent). Does not save, therefore cannot modify the file.
- pptx: `Presentation.Text()`; `--slide N` restricts to `Slide.Text()`.
  `--slide` values below 1 are a flag error (exit 2), never a silent
  whole-document read.

### `spine cells <file> <sheet> <range>`

XLSX only (`unsupported` otherwise). `<sheet>` matches `SheetByName`; `<range>`
is `A1` or `A1:D20` (split on `:` and parse each endpoint with the exported
`xlsx.ParseCellRef`; `invalid_range` on failure; a single cell is a 1×1 range;
the two endpoints must be ordered top-left to bottom-right after parsing —
swap them if reversed). Reads only — uses `FindCell`, never
`Cell`, so no cell is materialized. Output, row-major, present cells only:

```json
{
  "sheet": "Data",
  "range": "A1:C2",
  "cells": [
    {"ref":"A1","type":"string","value":"Revenue"},
    {"ref":"B1","type":"formula","value":1234.5,"formula":"=SUM(B2:B9)"},
    {"ref":"C1","type":"number","value":42},
    {"ref":"A2","type":"boolean","value":true},
    {"ref":"B2","type":"date","value":"2025-10-14T00:00:00Z"},
    {"ref":"C2","type":"error","value":"#REF!"}
  ]
}
```

- `type` is one of `empty|string|number|boolean|formula|error|date`, mapped
  from `Cell.Type()`. `empty` cells are never emitted (FindCell found nothing).
- `value` JSON typing: string→string, number→number, boolean→bool,
  error→string (`#REF!` etc.), date→RFC 3339 with UTC assumed when the serial
  carries no zone (`time.Time` from the library formatted RFC 3339), formula→
  the cached result typed as above. A formula cell whose cached value is
  absent (`Cell.Value()` nil) emits `"value":null`.
- `formula` present only for `type:"formula"`: the formula string as stored,
  with leading `=`. A formula cell with no cached result reports
  `"value":null`; the library reports such a cell's value as the formula text
  itself, so "no cached result" is defined as `Cell.Value()` equalling
  `Cell.Formula()` (a cached string exactly equal to the formula text is the
  accepted false positive).
- Human format: tab-separated `ref<TAB>type<TAB>value` lines, row-major.

### `spine set <file> <selector> <value>`

Opens, applies one write, atomic-saves. `--out PATH` variant per Conventions.
Flags: `--type STRING` forces interpretation; default inference below.

- XLSX cell (`Sheet1!B2`):
  - `--type number`: parse with `strconv.ParseFloat`; failure →
    `invalid_value`.
  - `--type string`: store as-is, never inferred as number.
  - `--type boolean`: accept exactly `true`/`false` (else `invalid_value`).
  - `--type formula`: value must start with `=` (else `invalid_value`);
    written with `SetFormula`.
  - `--type date`: value is RFC 3339; stored as an Excel date serial with the
    built-in date number format applied, so it reads back as `type:"date"`.
  - default inference: `=...`→formula, `true`/`false`→boolean, a parseable
    float→number, otherwise string.
  - Selecting an absent cell creates it (this is the write path; `Cell` is
    correct here). Writing to a merged cell other than its top-left is an
    error: `invalid_value` with message "B2 is part of merged range A1:B2;
    write to A1". (Implementation: check the sheet's merged ranges; the
    library may already enforce this — if it does, rely on it and map its
    error to `invalid_value`.)
- PPTX text (`slide/N/title`, `slide/N/body`, `slide/N/shape/ID`): value is
  the new text, `\n` separated into paragraphs via `TextFrame.SetText` /
  `PlaceholderShape.SetText`. `--type` is not accepted (flag parse error,
  exit 2). A shape that holds no text frame (picture, table, group, chart,
  …) is `unsupported`.
- On success prints the selector and its new state:
  - xlsx: `{"selector":"Sheet1!B2","type":"number","value":42}`
  - pptx: `{"selector":"slide/3/title","shape_id":5,"text":"..."}`
    (`shape_id` from the shape after the edit; it may be 0 for a placeholder
    newly created by the edit — report it as-is).

### `spine help`

Usage text listing every command with its flags (hand-written, no framework).
`spine` with no arguments prints it and exits 0. Unknown command: usage on
stderr, exit 2.

## Security & robustness rules

- Paths are used exactly as given: no home-directory expansion, no globbing,
  no shell involvement. The binary never execs anything.
- The temporary save file is created with `os.CreateTemp` in the destination
  directory with mode 0600 and renamed after fsync; on any error the temp file
  is removed and the destination is untouched.
- Input size guard: refuse to open files larger than 1 GiB (`unsupported`)
  with a clear message; spine already bounds resource use internally.
- Output is never larger than the document's text; `text` streams to stdout
  with a bounded buffer (the commands hold `Text()` strings in memory, which
  is acceptable — spine's Text() is bounded by document size).
- The CLI must never panic on malformed input files: any recovered panic is
  reported as `internal` with the panic message, exit 1, destination
  untouched.

## Testing requirements

- Table-driven tests per command; every error code exercised at least once.
- Fixtures are generated in `TestMain` by the spine packages themselves
  (`xlsx.Create`, `pptx.CreateWidescreen`, …) into `t.TempDir()` — no binary
  testdata committed for slice 1.
- Golden JSON outputs asserted exactly (field order stable via struct
  definitions, never maps, so `map[string]any` is forbidden in output paths).
- Round-trip test for `set`: create fixture → set → reopen → assert via the
  read commands → also assert `spine info` still succeeds (catches corrupted
  saves).
- Atomicity test: point a save at an unwritable directory, assert input file
  bytes unchanged and no temp files left behind.
- Determinism test: run `cells`/`ls` twice on the same fixture, assert
  identical stdout.
- `go build ./... && go test ./... && go vet ./... && gofmt -l .` clean.

## Non-goals (slice 1)

DOCX, styles/formatting, images, charts, comments, batch ops, server, MCP.
Selectors for shapes inside groups. Creating files (`spine new`) — arrives
with slice 2's op vocabulary.

## Slice 2 — the operation vocabulary

One JSON vocabulary for every mutation, regardless of transport: `spine set`
runs a single operation; `spine edit` runs a batch; slice 3's server accepts
the same objects over HTTP. The vocabulary is versioned so it can grow
without breaking agents that pinned it.

### Document

An operations document is one JSON object:

```json
{
  "version": 1,
  "ops": [
    {"op": "set_cell", "selector": "Data!B1", "value": 42},
    {"op": "add_slide"}
  ]
}
```

- `version` must be `1` (else `invalid_value` with a message naming the
  supported version).
- `ops` must be present and a non-empty array; each element an object with a
  string `op` field naming the operation (else `invalid_value`).
- Unknown `op` name: `invalid_value` with the message "unknown operation
  \"foo\"; run `spine ops-schema` for the list".
- Unknown fields in an operation are errors (`invalid_value`), never ignored:
  a typo like `"valu"` must not silently no-op. Error messages name the
  offending field.

### Operations

Selectors reuse the slice 1 grammar and resolution rules exactly. Fields not
listed are errors. All ops accept nothing else.

- **`set_cell`** — fields: `selector` (xlsx cell), `value` (string, number or
  boolean), `type` (optional: `string|number|boolean|formula|date`). JSON
  typing is authoritative: a JSON number is stored as a number, a boolean as
  a boolean, a string per the same inference rules as `spine set`; `type`
  overrides. `value` must be present; `null` is `invalid_value` (clearing a
  cell is `clear_cell`, below).
- **`clear_cell`** — fields: `selector` (xlsx cell). Makes the cell empty
  (removes value and formula; keeps any style).
- **`set_text`** — fields: `selector` (pptx text target: `slide/N/title`,
  `slide/N/body`, `slide/N/shape/ID`), `text` (string). `
` separates
  paragraphs.
- **`add_sheet`** — fields: `name` (string, optional). Without `name`, the
  sheet is `Sheet2`-style: the first unused `Sheet<N>` (Excel's rule, checked
  case-insensitively, starting from the smallest free N). With `name`, illegal
  or taken names are `invalid_value` — the CLI never silently renames
  (`UniqueSheetName` exists for callers who want coercion; the CLI does not
  guess). Result reports the new sheet's index and name.
- **`remove_sheet`** — fields: `name` (string, exact match; `not_found`
  otherwise). Destructive: the library's cascade rules apply (formulas
  referencing it become `#REF!` permanently; shared pivot caches survive).
  The workbook's last remaining sheet cannot be removed (`invalid_value`).
- **`add_slide`** — fields: `layout` (optional string enum). Without `layout`,
  appends a slide with no placeholders (the library's `AddSlide`) — its text
  can only be set through text boxes added in a later slice. With `layout`,
  the slide is added from the master's layout of that type
  (`AddSlideFromLayout`), inheriting the layout's placeholders, so
  `set_text` on its title/body works immediately. Enum: `title`,
  `title_and_content`, `section_header`, `two_content`, `comparison`,
  `title_only`, `blank`, `content_with_caption`, `picture_with_caption`,
  `title_and_vertical_text`, `vertical_title_and_text` (mapped to the
  library's `SlideLayoutType` constants). A deck whose master lacks that
  layout is `not_found`, with a message naming the type. Result reports the
  new 1-based slide index and the layout used (or null).
- **`remove_slide`** — fields: `index` (integer, 1-based; `not_found` if out
  of range). The presentation's last remaining slide cannot be removed
  (`invalid_value`).
- **`set_properties`** — fields, all optional strings: `title`, `subject`,
  `creator`, `keywords`. Empty string clears the property. Works on both
  kinds.

### `spine edit <file> <opsfile>`

Opens, applies the operations in order, atomic-saves once. `--out PATH` and
`--password` behave as elsewhere. `opsfile` is a path, or `-` for standard
input (the one sanctioned stdin use: agents pipe generated batches).

- All-or-nothing: the first failing operation aborts the batch and nothing is
  saved. Because ops run against the in-memory document, a failure leaves no
  partial state on disk; a later op can never repair an earlier one's
  failure, so agents get exactly one error to fix.
- Failure output (stderr, exit 1, `--json` respected): the usual error
  envelope plus `"op_index": N` (0-based) and `"op": "<name>"`. When several
  defects exist in one operation, the reported one is deterministic: fields
  are checked in the operation's declaration order, unknown fields in
  sorted order.
- Success output: `{"applied": N}` where N is the operation count.
- Human format mirrors this; stderr keeps the `spine: ` prefix.

### `spine new <kind> <file>`

`kind` is `xlsx` or `pptx` (else `invalid_value`). Creates an empty file —
`pptx.CreateWidescreen()`, or `xlsx.Create()` plus one sheet named `Sheet1`
(the library refuses to serialize a sheetless workbook, so the CLI never
produces one) — at `file` via the atomic-save
path. An existing `file` is never overwritten: `io` with the message "file
already exists: PATH". Flags: `--title STRING`, `--creator STRING` (stored as
document properties). Output: `{"kind":"xlsx","path":"..."}`.

### `spine ops-schema`

Prints the machine-readable description of the vocabulary — the same
information an MCP adapter or agent prompt needs, generated from the same Go
structures the commands dispatch on (one source of truth; a hand-written copy
that drifts is a bug):

```json
{
  "version": 1,
  "operations": [
    {
      "op": "set_cell",
      "applies_to": ["xlsx"],
      "fields": {
        "selector": {"type": "string", "required": true,
                     "description": "xlsx cell selector, e.g. Sheet1!B2"},
        "value":    {"type": "string|number|boolean", "required": true},
        "type":     {"type": "string", "required": false,
                     "enum": ["string", "number", "boolean", "formula", "date"]}
      }
    }
  ]
}
```

Every operation must appear; `fields` maps field name to type, requiredness,
`enum` values where applicable, and a one-line description including
destructiveness warnings (`remove_sheet`'s cascade). `--json` is accepted but
changes nothing (the output is always JSON).

### Slice 2 testing requirements

Everything in the slice 1 list, plus:
- Every operation has an accept test and at least one reject test per
  required field and per error condition, including unknown-op,
  unknown-field, and wrong-`version` rejects.
- Batch atomicity: a two-op batch whose second op fails leaves the file
  byte-identical (`md5` before/after) with no temp files.
- `spine edit` via stdin (`-`) and via file path both covered.
- `ops-schema` output parses as JSON and every `op` name in it is accepted by
  `spine edit` (a test iterates the schema, builds one minimal op per entry,
  and asserts `edit` does not fail with "unknown operation").
- `spine new` refuses to overwrite; created files pass `spine info`.

## Slice 3 — `spine serve` (outline, spec before implementation)

`spine serve <file> --port N` binds 127.0.0.1 only, opens the file once, and
holds the live document in memory. Endpoints map to the vocabulary: `GET
/info`, `GET /ls?scope=…`, `GET /text`, `GET /cells?sheet=&range=`, `POST
/op`, `POST /ops`, `POST /save`, `POST /revert`, `GET /schema` (the
`ops-schema` document plus read-command descriptions), `GET /render` (PNG via
the render packages, cached by content hash). Ops apply in memory; nothing
touches disk until `/save`, which uses the same atomic path and keeps the
original bytes for `/revert`. An ops journal sidecar records every applied
batch. This slice gets its own spec section before any code.
