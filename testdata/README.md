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

## python-pptx fixture corpus

A handful of pptx tests (`pptx/schema_test.go`) exercise fixtures from
the [python-pptx](https://github.com/scanny/python-pptx) test suite.
These now live in `python-tests/` at the repository root (committed —
~3 MB of Python test code carried only for its `test_files/` fixtures),
so the tests run on a fresh clone with no extra setup.

python-pptx is MIT-licensed, and MIT requires its copyright notice and
permission notice to travel with any substantial portion of the software —
which 2.9 MB of verbatim source plainly is. The notice is therefore vendored
at [`python-tests/LICENSE`](../python-tests/LICENSE) (Copyright (c) 2013 Steve
Canny). It covers everything under `python-tests/`; the repository's root
`LICENSE` is spine's own and does not apply there.
