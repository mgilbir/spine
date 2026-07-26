# Test fixtures

## Fetched fixtures

`fetch.sh` downloads the external fixtures listed in `external.txt` into
`testdata/external/` directories (gitignored). It is best-effort by
default: unreachable fixtures are reported and skipped, and the tests
that need them skip silently when the files are absent. Run it with
`--strict` (or `make fetch-strict`) to fail on any download error. A few
fixtures have no known public URL (their commented-out entries in
`external.txt` suffixed `— URL unknown`) and can never be fetched; the corresponding tests always
skip on a fresh clone.

## Optional python-pptx fixture corpus

A handful of pptx tests (`pptx/schema_test.go`) exercise fixtures from
the [python-pptx](https://github.com/scanny/python-pptx) test suite. To
use them, copy python-pptx's `tests/` directory to `python-tests/` at
the repository root (gitignored — it is ~3 MB of Python test code
carried only for its `test_files/` fixtures). Without it those tests
skip. python-pptx is MIT-licensed; its license and copyright notice
apply to the copied fixtures.
