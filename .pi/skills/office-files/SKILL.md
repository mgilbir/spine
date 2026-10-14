---
name: office-files
description: Create, read, edit, and verify Office files (xlsx, docx, pptx) with the spine CLI and spine serve. Use for any task involving .xlsx/.pptx/.docx files — spreadsheets, decks, documents — instead of writing Python or Go scripts. Covers the read/author loop, the JSON operation vocabulary, sessions over HTTP, and when to verify with text versus rendered images.
---

# Office files with spine

spine is a Go CLI (and local HTTP server) that edits Office files with
round-trip fidelity: parts you do not touch are written back byte-for-byte.
Build it once per session:

```bash
go build -o /tmp/spine ./cmd/spine   # run from the spine repository root
```

`/tmp/spine` below refers to that binary. Every command prints human-readable
output by default; add `--json` for machine-readable output. Exit codes:
0 success, 1 failure, 2 bad flags. Errors carry a `code`
(`not_found`, `invalid_selector`, `invalid_value`, `invalid_range`, `io`,
`unsupported`, `internal`) — read the message, it usually says what to do.

## The loop

1. **Discover** what is addressable, never guess:
   - `/tmp/spine info book.xlsx --json` — kind, sheets/slides, properties
   - `/tmp/spine ls deck.pptx slides` — one row per slide
   - `/tmp/spine ls deck.pptx slide/3` — shapes on slide 3 with their **ids**
   - `/tmp/spine ls book.xlsx sheets` — exact sheet names
2. **Read** content as text: `/tmp/spine text deck.pptx` (or `--slide N`),
   `/tmp/spine cells book.xlsx Sheet1 A1:D20 --json` (typed values; formulas
   report their stored formula and cached value, `null` when not yet
   calculated — **spine does not evaluate formulas**; Excel does that on
   open).
3. **Author** with one command per edit (`spine set`) or a batch
   (`spine edit`): generate an operations document from
   `/tmp/spine ops-schema` and apply it — all-or-nothing, one save:
   ```bash
   /tmp/spine edit book.xlsx ops.json            # or -  for stdin
   ```
4. **Verify by re-reading**: run the same `text`/`cells` command again and
   check what you expected is there.
5. **Verify visually when presentation matters** (see below), then you are
   done — mutating commands already saved.

## Selectors

- xlsx cell: `Sheet1!B2` — sheet names match **exactly** (case-sensitive);
  take names from `ls sheets`, never guess. Quoted form `'Q3 Data'!A1` for
  names with spaces.
- pptx text: `slide/3/title`, `slide/3/body`, `slide/3/shape/7` — shape ids
  come from `ls deck.pptx slide/3`; re-list after structural changes.
- Values: JSON numbers/booleans are stored typed; strings follow inference
  (`=...` formula, `true`/`false`, parseable number, else string). Negative
  numbers work as bare arguments (`-0.5`). `--type` forces interpretation.

## Sessions: `spine serve` for many operations on one file

```bash
/tmp/spine serve deck.pptx --port 8089 --fallback-noto
```

Opens the file once, holds it in memory; every read and operation is an HTTP
call (same JSON envelopes as the CLI): `GET /info`, `/ls?scope=…`, `/text`,
`/cells?sheet=&range=`, `POST /op`, `POST /ops`, `POST /save`, `POST
/revert`, `GET /render?target=slide/3&format=png`, `GET /schema`. Bind is
127.0.0.1 only. Nothing touches disk until `POST /save`; failed batches are
rolled back in memory. Use serve when you will do more than a handful of
operations, when a human wants to watch the embedded UI at
`http://127.0.0.1:8089/`, or when you need renders of unsaved edits.

## Text vs image verification — decide deliberately

**Text is the content path; images are the presentation path.** They answer
different questions:

- `text`/`cells`/`ls` tell you *what is there* — values, strings, structure.
  They are cheap (no fonts, no CPU) and always available. Author and read
  through them.
- A **render** tells you *what it looks like* — the only way to catch
  overlapping shapes, text overflow, broken styling, or a chart that does not
  draw. It costs fonts and CPU, so do it once at the end, or when the task is
  explicitly about appearance ("make the title slide look right").

One-shot renders (no session): `spine-render -input deck.pptx -out slides/
-font ... --fallback-noto`. In a session: `GET /render`. If a render fails
with `unsupported`, the message is instructions: sizing errors are fixed
with the `set_column_width`/`set_row_height` operations; unresolved fonts by
mapping `--font` or passing `--fallback-noto`. A deck or sheet can be
content-perfect and still fail to render — that is a preview limitation,
not file corruption; do not "fix" the content because a render failed.

## Recipes

**Spreadsheet report from scratch** — `new xlsx` (one default sheet) → `edit`
with `add_sheet`, `set_cell` for values and `=SUM(...)` formulas →
`set_column_width` on every populated column and `set_row_height` on
populated rows if you will render or screenshot it.

**Fill a template deck** — `ls file slide/N` to find placeholders →
`set_text` on `slide/N/title` and `slide/N/body`. A slide added with
`add_slide` has no placeholders; use `add_slide` with `"layout":
"title_and_content"` (or another enum from `ops-schema`) to inherit them.

**Read a document for an LLM** — `text` for the whole file or one
slide/sheet; nothing else. It is deterministic plain text with no markup.

**Inspect before editing someone else's file** — `info` and `ls` first,
`cells`/`text` second, then edit. Never edit blind.

## Gotchas

- Merged cells: only the top-left cell of a merged range accepts writes.
- `remove_sheet` is destructive and permanent (formulas referencing it become
  `#REF!`); the last remaining sheet/slide cannot be removed.
- Encrypted files: pass `--password` to any command that opens one. A
  plaintext file stays plaintext even when `--password` is set.
- Sheet names with `!`, `[`, `]` or doubled `''` are rejected — nothing is
  ever silently renamed.
- The authoritative, always-current contract for operations is
  `/tmp/spine ops-schema` and `GET /schema`; this skill summarizes but the
  binary wins.
