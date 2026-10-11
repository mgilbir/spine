# Troubleshooting

Common errors and what they mean. Match errors with `errors.Is`: every error
below with a name is a sentinel that the returned error wraps. Every entry
links to the reference section that documents the behavior in full.

## Error catalog

| Error | Returned by | Means | What to do |
|---|---|---|---|
| `pptx.ErrNotPPTX`, `docx.ErrNotDOCX`, `xlsx.ErrNotXLSX` | `Open`, `OpenReader` | A valid Office package of another kind, e.g. a workbook opened with `docx` | Open it with the package for its kind |
| `zip: not a valid zip file` (no sentinel) | `Open`, `OpenReader` | Neither a zip package nor a compound file: not an Office Open XML document | — |
| `opc.ErrEncrypted` | `Open`, `OpenReader` without a password | A password-encrypted document | See [below](#open-returns-opcerrencrypted) |
| `opc.ErrLegacyBinary` | `Open`, `OpenReader` | A legacy binary `.doc`/`.xls`/`.ppt` (Office 2003 and earlier), with or without a password | spine does not read these; convert the file to the current format first |
| `opc.ErrCorruptedPackage` | `Open`, `OpenReader` | The package or its CFB container is malformed | — |
| `crypto.ErrWrongPassword` | `Open` with `opc.WithPassword` | The password did not decrypt the file | Ask again |
| `crypto.ErrIntegrityCheckFailed` | `Open` with `opc.WithPassword` | The encrypted file's integrity check failed: tampered, or written without one | See [Integrity](encryption-and-signing.md#integrity) |
| `crypto.ErrUnsupportedEncryption` | `Open` with `opc.WithPassword` | An encryption scheme spine cannot decode | — |
| `opc.ErrStrictOOXML` | `Open`, `OpenReader` | An ISO-Strict document | See [below](#open-returns-opcerrstrictooxml) |
| `… (pass WithMaxDecompressedPartSize to allow it)` and the other `WithMax*` messages (no sentinel) | `Open`, `OpenReader` | A [resource limit](../README.md#resource-limits) was reached | Pass the option the message names, if you trust the input |
| `xml.ErrNestingTooDeep` (`common/xml`) | `Open`, `OpenReader` | An XML part nests deeper than `WithMaxNestingDepth` allows | As above |
| `crypto.ErrInvalidPassword` | `SaveEncrypted*` | The password is empty, not valid UTF-8, or longer than 255 UTF-16 code units (Office's limit) | — |
| `xlsx.ErrNoSheets` | `Save*` on a workbook | The workbook has no sheets; Excel refuses such files | Add a sheet |
| `validate.Report` (an error value) | `Save*` | The pre-save check refused the save | See the next two entries |

## Save refused with a validation error

`Save`, `SaveBytes`, and `SaveTo` run `Validate()` first and refuse to write when
any **error-severity** finding is present, returning the report as an error, so an
edit cannot introduce a structural error. Warnings never block a save.

To see what tripped it, call `Validate()` yourself before saving and read the
`validate.Report` (from `github.com/mgilbir/spine/common/validate`) — each finding
carries a stable `Code`, a `Severity`, the `Part` it concerns, and a
human-readable `Detail`:

```go
for _, f := range p.Validate() {
    fmt.Printf("%s [%s] %s: %s\n", f.Code, f.Severity, f.Part, f.Detail)
}
```

Fix the reported condition (for example a duplicate shape id, a dangling
`headerReference`/sheet/`sldLayoutId` relationship reference, or overlapping
merged ranges). Only error-severity findings refuse the save — a warning such as
a `numPr` that references an undefined numbering definition is reported and the
save proceeds, because Word opens such a document and blocking it would reject a
file Word accepts. If an error-severity finding is genuinely advisory for your
use case, `SaveToUnvalidated` writes without the pre-save check — but the
default gate exists because Office rejects the structures it flags. The
[Validation](../README.md#validation) section of the README carries the full
catalog, code by code, with each check's severity; it is the single source of
truth and is verified against the validators by a test.

## Save refused after only reading a file

The input file already had the problem; reading it is what exposed it. The
pre-save check covers what the program has loaded. A pptx slide or the docx
body that was never read is written back byte-for-byte without being checked,
so a damaged file opens and saves cleanly. Once your code reads that slide or
the body (`Shapes()`, `Text()`, `Paragraphs()`, `ReplaceText`, ...), it is
loaded, checked, and the same save is refused. xlsx checks every sheet on every
save, so there this shows up on the first save.

The finding's `Part` names the damaged part. Either repair the condition through
the API before saving, or, if you only meant to pass the file through, save
before reading it, or write with `SaveToUnvalidated`. Before you trust a file,
call `Validate()` after reading the parts you rely on. An untouched file that
saves cleanly has not been checked.

## Open returns `opc.ErrEncrypted`

The input is a password-encrypted document and the open was given no password.
spine reports it as `opc.ErrEncrypted` rather than failing obscurely, so you can
prompt for the password and retry the same call with it:

```go
doc, err := docx.Open(path, opc.WithPassword(password))
```

There is no separate encrypted entry point — `Open` and `OpenReader` in every
format take the option, as do `opc.OpenReader` and `opc.NewReader` when you want
the raw package rather than a document model. See
[Encryption and signing](encryption-and-signing.md).

A legacy binary Office file (`.doc`, `.xls`, `.ppt`, from Office 2003 and
earlier) is stored the same way, but is not encrypted: it returns
`opc.ErrLegacyBinary` instead, so it is never mistaken for one that needs a
password.

## Open returns `crypto.ErrWrongPassword`

The file is encrypted and the supplied password did not decrypt it. Match it with
`errors.Is(err, crypto.ErrWrongPassword)` (from
`github.com/mgilbir/spine/common/crypto`). See
[Encryption and signing](encryption-and-signing.md).

## Open returns `opc.ErrStrictOOXML`

The file is a valid Office document written in the ISO-Strict (ISO/IEC 29500
Strict) dialect, which uses the `purl.oclc.org/ooxml` namespaces instead of the
transitional ones spine reads. This is a distinct signal that the file is a
genuine Office document in an as-yet-unsupported dialect, not a corrupt or
non-Office file. See [Supported Flavors](../README.md#supported-flavors).

## Office offers to repair my file

This should not happen for a file spine built, or for the parts of an opened
file that your code changed. Every save runs `Validate()` first and refuses to
write when a structural error is present, and the validator is tuned so that no
error-severity finding fires on a file the corresponding Office app accepts.

Check first whether the input file already prompts for repair. Parts your code
never read are passed through unchecked (see
[Save refused after only reading a file](#save-refused-after-only-reading-a-file)),
so a damaged input stays damaged. If the input opens cleanly and spine's output
does not, that is a bug. Please open an issue and attach both files and the code
that produced the output, so the failing structure can be reproduced. See
[Validation](../README.md#validation).

## Tests skip on my machine

Many tests depend on fixtures that are not checked into the repository, and they
skip silently when the fixture is absent — so a fresh clone exercises fewer cases
than a fully provisioned tree:

- **External round-trip fixtures** are downloaded with `make fetch`; tests that
  need a missing file skip. Four fixtures have no public URL and always skip. See
  [testdata/README.md](../testdata/README.md).
- **python-pptx fixtures** (a few pptx tests) need no setup: `python-tests/` is
  committed, so they run on a fresh clone. See
  [testdata/README.md](../testdata/README.md#python-pptx-fixture-corpus).
- **The Common Crawl corpus** (`go test ./cctest`) needs `make fetch-cc`; a plain
  run checks only a fast deterministic subset. See
  [testdata/cc/README.md](../testdata/cc/README.md).

This is expected behavior, not a failure. See [Testing](../README.md#testing) for
the full fetch-and-run flow.
