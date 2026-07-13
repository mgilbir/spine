# Common Crawl OOXML test corpus

This directory holds the *pointers* to a large corpus of real-world Office
files harvested from [Common Crawl](https://commoncrawl.org/), plus the
quarantine list for files the library cannot yet round-trip. The corpus
itself is fetched locally and is **never committed** (see Licensing below).

## Pipeline

```
sweep.sh  ──►  manifest-{pptx,xlsx,docx}.tsv  ──►  tools/ccfetch  ──►  testdata/corpus/cc/  ──►  go test ./cctest
 (dev-only)          (committed)                  (make fetch-cc)          (gitignored)          (skips if absent)
```

1. **Sweep** — `sweep.sh` queries the Common Crawl columnar index (the
   `cc-index` parquet table on `data.commoncrawl.org`, via DuckDB + httpfs)
   for responses whose detected MIME type is pptx, xlsx, or docx, with
   `fetch_status = 200`. Complete payloads go to `manifest-<type>.tsv`;
   payloads the crawler truncated go to `manifest-<type>-truncated.tsv`
   (see "Live fetch" below). Candidates are deduplicated by
   `content_digest`, capped at 5 files per registered domain per type (so a
   single WordPress site cannot dominate the corpus), stably sorted, and cut
   to the target count. The resulting manifests are committed here.
   Contributors never need to run the sweep; it is only rerun to refresh the
   manifests (e.g. from a newer crawl).

2. **Fetch** — `make fetch-cc` (i.e. `go run ./tools/ccfetch -manifest
   testdata/cc -out testdata/corpus/cc -n 1000`) downloads each manifest row
   with an HTTP range request against the public WARC archive, decodes the
   WARC record, validates the payload is a real OPC/OOXML package, classifies
   its *actual* type from the package contents (mislabeled MIME types are
   refiled), dedups by SHA-256, and stores it as
   `testdata/corpus/cc/<type>/<sha16>.<type>`. The run is resumable: progress
   is journaled to `testdata/corpus/cc/fetched.tsv` and consulted on restart.

3. **Test** — `go test ./cctest` opens corpus files, saves them without
   modifications, reopens the output, and compares the original and saved
   packages part by part. By default it checks a fast deterministic subset
   (the first 60 files per type in sha16 order) so a plain `go test ./...`
   finishes well inside Go's default 10-minute package timeout; run the
   complete corpus with `make test-corpus` (which sets `SPINE_CC_FULL=1` and
   a 45-minute timeout — the full pass needs ~15-20 minutes). When the corpus
   directory is absent the test skips (same philosophy as the
   `testdata/external/` fixtures). Set `SPINE_CC_CORPUS` to point at a corpus
   in a non-default location.

## Live fetch of truncated candidates

Common Crawl truncates stored payloads at 1 MiB, so the WARC path can never
yield larger documents — and large real-world files exercise different code
paths (many parts, huge shared-string tables, embedded media). For rows in
the `manifest-<type>-truncated.tsv` manifests the fetcher therefore ignores
the (incomplete) WARC record and refetches the *original URL* from the live
web, with extra safeguards:

- **DNS blocklist gate**: every host — including the target of each
  cross-host redirect — is first resolved through a filtering
  DNS-over-HTTPS resolver (RFC 8484). Point `-doh-url` (or the
  `SPINE_DOH_URL` env var) at your resolver, e.g. a NextDNS profile
  endpoint. There is deliberately no default: without a resolver URL the
  live phase refuses to run. Hosts answering with the unspecified address
  (`0.0.0.0` / `::`) are treated as blocked and skipped; NXDOMAIN/SERVFAIL
  hosts are dead and skipped. Verdicts are cached per host for the run.
- **Politeness**: these are origin servers, not a CDN — live concurrency is
  fixed at 2 workers with a longer per-request delay, at most 5 redirects,
  a single retry, and a hard size cap (`-live-max-size`, default 50 MiB).
- **Caps**: `-live-n` (default 200) live files per type on top of the
  WARC-fetched corpus; live files are journaled with `source=live`.

Live-fetched corpora are inherently less reproducible than WARC-fetched
ones: origins change and die (expect heavy link rot — `rot`, `dead`, and
`blocked` counts in the summary are normal, the fetcher just moves on). The
committed manifest still pins exactly what was attempted.

## Quarantine (`known_failures.tsv`)

Wild files exercise corners the library does not handle yet — failures are
expected and are the point of the corpus. Known failures are recorded in
`known_failures.tsv` (columns: `sha16`, `type`, `stage`, `note`; stage is one
of `open`, `save`, `reopen`, `fidelity`). Files failing at a quarantined
stage are skip-counted; **new** failures fail the test with the file's hash,
source URL, stage, and error. The quarantine is the running catalog of
compatibility work: fixing a bug means deleting its rows.

To regenerate the quarantine after a fix wave, run the full corpus test with
`SPINE_CC_UPDATE_QUARANTINE=1 go test ./cctest -count=1 -timeout 45m`: the
committed quarantine is ignored, every failure becomes a fresh row, and
`known_failures.tsv` is rewritten in place (sorted, ready to commit). The
older `SPINE_CC_EMIT_QUARANTINE=1` mode still prints `CCQUARANTINE` lines
for ad-hoc collection.

## Reproducibility and politeness

- The manifests are pinned to a single crawl (`CC-MAIN-2026-25`), so a fetch
  from the same manifests yields the same files. WARC records are immutable;
  rows can only disappear if Common Crawl retires the crawl entirely.
- The fetcher identifies itself as
  `spine-corpus-fetch/1.0 (+github.com/mgilbir/spine)`, uses ranged requests
  (only the exact record bytes are transferred), runs 4 workers by default
  with a per-worker politeness delay, and backs off exponentially on
  429/5xx/timeouts. Please keep those defaults when scaling up.

## Licensing

The fetched files are third-party web content retrieved locally from Common
Crawl's public archive purely for compatibility testing. They are **never
redistributed with this repository** — the same stance as the fixtures in
`testdata/external.txt`. Only the manifests (URLs and WARC offsets), the
tooling, and the quarantine metadata are committed.

## Scaling up

- More files per type: raise the fetcher limit (`go run ./tools/ccfetch ...
  -n 3000`). The committed manifests carry ~3x margin over the default goal.
- Beyond the manifest margin: rerun `sweep.sh -t <bigger target>` to emit
  larger manifests (pptx is scarce — roughly one crawl's warc subset yields
  only a few thousand distinct candidates).
- Newer crawls: `sweep.sh -c CC-MAIN-<yyyy-ww>` regenerates the manifests
  from another crawl; commit the new manifests to repin the corpus.
