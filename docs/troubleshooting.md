# Troubleshooting

Common errors and what they mean. Every entry links to the reference section
that documents the behavior in full.

## Save refused with a validation error

`Save`, `SaveBytes`, and `SaveTo` run `Validate()` first and refuse to write when
any **error-severity** finding is present, returning the report as an error, so a
structurally corrupt package is never produced. Warnings never block a save.

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

## Open returns `opc.ErrEncrypted`

The file is password-protected. The plain open path detects an encrypted input
and returns `opc.ErrEncrypted` rather than failing obscurely. For Word, open it
with `docx.OpenEncrypted` and the password to get a `*docx.Document`. For xlsx
and pptx there is no format wrapper yet: decrypt with `opc.OpenEncrypted`, which
returns a low-level `*opc.Reader`. See
[Encryption and signing](encryption-and-signing.md).

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

This should not happen for a file spine produced: every save runs `Validate()`
first and refuses to write when a structural error is present, and the validator
is tuned so that no error-severity finding fires on a file the corresponding
Office app accepts. If a library-produced file does prompt for repair, that is a
bug — please open an issue and attach the file (and the code that produced it)
so the failing structure can be reproduced. See
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
