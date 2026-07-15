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
of `open`, `save`, `reopen`, `fidelity`, or `wontfix`). Files failing at a
quarantined stage are skip-counted; **new** failures fail the test with the
file's hash, source URL, stage, and error. The quarantine is the running
catalog of compatibility work: fixing a bug means deleting its rows.

The `wontfix` stage marks a row as **permanent-by-design**: the file cannot
round-trip byte-identically for a reason outside the library's control, and
the note carries the hand-written reason instead of an error signature.
Current reasons fall into three families: corrupt source archives (a stored
CRC that does not match the data — the library tolerates the part, a strict
re-read cannot), the decoder's mandatory XML end-of-line normalization
(pretty-printed parts whose CRLF inter-element text reaches the model as
LF), and whitespace-preserving producers (parts pretty-printed with
inter-element whitespace at every level of the tree, which the typed model
does not capture). A `wontfix` row matches a failure at *any* stage and is
skip-counted like a quarantined row, but reported in its own column of the
stats block, so the actionable quarantine and the permanent tail stay
separately visible. `wontfix` rows survive quarantine regeneration verbatim;
if the file starts passing they drop out like any other row.

To regenerate the quarantine after a fix wave, run the full corpus test with
`SPINE_CC_UPDATE_QUARANTINE=1 go test ./cctest -count=1 -timeout 45m`: the
committed quarantine is ignored (except `wontfix` rows, whose curated notes
are kept), every failure becomes a fresh row, and `known_failures.tsv` is
rewritten in place (sorted, ready to commit). The older
`SPINE_CC_EMIT_QUARANTINE=1` mode still prints `CCQUARANTINE` lines for
ad-hoc collection.

## Reproducibility and politeness

- The manifests are pinned to a single crawl (`CC-MAIN-2026-25`), so a fetch
  from the same manifests yields the same files. WARC records are immutable;
  rows can only disappear if Common Crawl retires the crawl entirely.
- The fetcher identifies itself as
  `spine-corpus-fetch/1.0 (+github.com/mgilbir/spine)`, uses ranged requests
  (only the exact record bytes are transferred), runs 4 workers by default
  with a per-worker politeness delay, and backs off exponentially on
  429/5xx/timeouts. Please keep those defaults when scaling up.

## Batched multi-crawl harvest (scaled, reference-only)

The single-crawl `sweep.sh` + `ccfetch` pipeline above keeps every fetched
binary on disk, which does not scale to a 10k/type corpus. The batched
pipeline instead treats binaries as **transient**: it commits only
*references*, downloads a file, tests it, records the outcome, and discards
the binary. Disk is not a constraint because nothing is kept.

```
sweep-multi.sh ─► manifest-{type}[-truncated].tsv ─► tools/ccrun ─► ledger + quarantine
 (several crawls)      (committed, 6-column)         (systemd-run scope)   (references only)
                                                     fetch→test→record→discard
```

### Reference-only manifests (`sweep-multi.sh`)

`sweep-multi.sh` sweeps a **list** of recent crawls (one crawl's warc subset
yields only a few thousand distinct pptx, so reaching 10k needs several),
deduplicates candidates **across all of them** by `content_digest`, applies
the per-registered-domain cap globally, and writes manifests with a leading
**`crawl` column** so every row is a self-contained reference:

```
crawl  url  warc_filename  warc_record_offset  warc_record_length  content_digest
```

Scanning stops early once every complete manifest has reached the target, so
listing extra crawls is harmless. Complete and truncated manifests are both
emitted. Regenerate with `make harvest-sweep` (override `HARVEST_CRAWLS`,
`HARVEST_TARGET`); get the current crawl ids from
<https://index.commoncrawl.org/collinfo.json>. These manifests are committed
(references only, a few MB/type). `ccfetch` and `ccrun` both accept the new
6-column layout and the legacy 5-column one.

### Sharded fetch → test → record → discard (`tools/ccrun`)

`ccrun` processes the manifests **one bounded batch per invocation**. The
**orchestrator** loads the durable ledger, selects the next `-batch` (default
2000) references not already in it, and for each spawns a **worker
subprocess** with a per-file timeout. The **worker** fetches that one file
(WARC range read, or DoH-gated live refetch for a truncated row), runs the
library round-trip discipline — `Open`, `Validate()` (the pre-save check; an
error-severity finding is a `validate`-stage failure), `SaveBytes`, reopen,
and part-by-part byte fidelity — prints one result line, and **deletes its
scratch file**. The orchestrator appends the outcome to the ledger and, on
failure, to the quarantine, then makes sure the scratch file is gone.

One invocation = one batch; the maintainer loops it. Because each reference is
processed in its own short-lived subprocess, a single pathological file cannot
take down the run.

### Resource isolation with `systemd-run` (and *why*)

`make harvest-batch` runs one batch inside a resource-capped scope:

```
systemd-run --user --scope -p MemoryMax=2G -p CPUQuota=200% \
  ccrun -manifest testdata/cc -ledger … -quarantine … -scratch … -batch 2000 -workers 2 -timeout 90s
```

The worker-subprocess-per-file boundary is the whole point. Under a
`MemoryMax` cgroup, a file that decompresses to gigabytes makes the **kernel
OOM-kill the worker** (the largest process in the scope) — not the
orchestrator, which holds almost nothing. The worker exits with a nonzero /
signal status; the orchestrator reads that (or a per-file-timeout kill, or a
panic) as a `resource:{killed,timeout,panic}` outcome and turns one poison
file into **one quarantine row, not a dead batch**. A worker only ever exits
nonzero on a genuine crash/OOM/hang: every ordinary test outcome (pass or a
staged failure) is a clean `exit 0` with a result line.

### Resumable ledger

The ledger (`testdata/corpus/cc-batch/ledger.tsv`, gitignored — it lives under
the ignored `testdata/corpus/`) has one append-only row per processed
reference (`digest  outcome  stage  signature  timestamp`; outcome is `pass`,
`fail`, `skip`, or `resource`) and is flushed per row. On restart `ccrun` skips
every digest already present, so an interrupted or OOM-killed batch loses at
most the in-flight file. A *transient* fetch failure (CDN throttling, timeout)
is **deferred** — left out of the ledger so a later run retries the reference
instead of burning it; only terminal outcomes are recorded. Resume loop:

```sh
# Repeat until a batch reports 0 remaining.
while :; do
  out=$(make harvest-batch 2>&1); echo "$out"
  echo "$out" | grep -q "0 remaining" && break
done
```

### Reference-keyed quarantine and refetch-for-debug

Failures accumulate in `testdata/cc/batch-quarantine.tsv`
(`digest  crawl  url  stage  signature`) — the growing cross-batch,
cross-run catalog. This is distinct from `known_failures.tsv` /
cctest's sha16-on-disk quarantine (whose files are **kept**): the batched
quarantine references files that were already discarded. To debug one, refetch
it to a named path (it is **not** deleted):

```sh
go run ./tools/ccrun -refetch <content_digest> -manifest testdata/cc -out /tmp/bad.docx
```

### Scaling beyond 10k

Raise `-t` on `sweep-multi.sh` and list more crawls (`make harvest-sweep
HARVEST_TARGET=20000 HARVEST_CRAWLS="CC-MAIN-2026-25 CC-MAIN-2026-21 …"`),
commit the larger manifests, and keep looping `harvest-batch` — the runner is
unchanged. More workers or a larger `MemoryMax` trade throughput for headroom;
the isolation model is identical at any size.

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
