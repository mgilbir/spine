# Common Crawl corpus — harvest & regeneration operations

This guide covers the **maintainer-only** operations for the Common Crawl
OOXML corpus: live-fetching the truncated candidates in full, regenerating the
manifests with the batched multi-crawl sweep, building the docx/xlsx stress
set, and running the resource-capped `ccrun` fetch→test→record→discard harness
at scale. Contributors do not need any of this — fetching the corpus and
running the tests is covered in [README.md](README.md), which you should read
first; this document picks up where it ends.

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
  live phase refuses to run. **Prefer the env var**: a NextDNS-style profile
  endpoint embeds an account token, and anything on a command line is
  world-readable through `ps` / `/proc/<pid>/cmdline` for the process's whole
  lifetime. `ccrun`'s orchestrator hands the resolver to each worker through
  the worker's environment for exactly that reason (C577). Hosts answering with the unspecified address
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
(references only, ~9.6 MB total). `ccfetch` and `ccrun` both accept the new
6-column layout and the legacy 5-column one.

**URL sanitization (query strings are stripped).** The `url` column keeps only
`scheme://host/path` — the entire `?query` is removed at manifest-write time.
Crawled presigned S3/MinIO document URLs carry AWS credentials in the query
(`AWSAccessKeyId`, `X-Amz-Signature`, `Signature=`), which must never be
committed or redistributed. Stripping the query is safe for the harvest:
WARC-complete rows are fetched by `warc_filename`+offset+length and identified
by `content_digest`, which deduplicates candidates across crawls and keys each
reference in the ledger and quarantine (the URL is provenance only), so nothing
breaks. The `content_digest` is the Common Crawl Base32-SHA-1 payload digest,
but the harness does **not** currently recompute and compare it: a fetched
payload is validated structurally instead — it must decode to a valid OOXML
package (`ValidateOOXMLPackage`) — and a mismatch there fails the fetch. The one
tradeoff is that a *truncated* row's live refetch can no longer replay a
presigned URL — but those signatures are short-lived and had almost always
expired anyway, so this is a minor, documented limitation of the reference-only
manifest, not a regression.

The committed set uses a **per-type domain cap and a per-type crawl span**,
because pptx is much scarcer than xlsx/docx:

| type | crawls | domain cap | complete | truncated |
| ---- | -----: | ---------: | -------: | --------: |
| docx | 4      | 15         | 10000    | 2107      |
| xlsx | 4      | 15         | 10000    | 1054      |
| pptx | 6      | 25         | 10000    | 6591      |

- **docx / xlsx** — swept from the 4 newest crawls (CC-MAIN-2026-25 / -21 /
  -17 / -12) at `-d 15`. They are abundant (tens of thousands of distinct per
  crawl), so they hit the 10k target from the first crawl and the sweep stopped
  early; a higher cap or more crawls would only cost them source diversity.
- **pptx** — swept from **6 crawls** (the same four plus the two next-older,
  CC-MAIN-2026-08 / -04) at `-p 25`, deduplicated across all six by
  `content_digest`. pptx is diversity-limited: distinct pptx are scarce
  (~1.5–1.8k per crawl after the cap), so the count climbed 5830 (cap 5, 4
  crawls) → 8063 (cap 15) → 8892 (cap 25) → **10000 (cap 25, 6 crawls)**, at
  which point it hit the target and was cut. Un-capped distinct pptx across the
  six crawls is 18840, so there is ample headroom to grow further with a higher
  `-p` or more crawls.

Reproduce with `sweep-multi.sh -d 15 -p 25 <crawls>`: the four newest crawls
give the docx/xlsx manifests; sweeping the six crawls (and taking the pptx
manifests from that run) gives pptx. The sweep is resumable, so pptx was topped
up incrementally by sweeping the two older crawls and merging. The default cap
stays 5 (diversity-first).

### Second distinct set — the docx/xlsx stress set (`testdata/cc/stress/`)

`testdata/cc/stress/manifest-{docx,xlsx}[-truncated].tsv` is a **second, fully
distinct** 10k-each corpus of docx and xlsx that **shares no `content_digest`
with the canonical set above**. Its purpose is a library stress test: a fresh
10k/type of real-world files to run through the same `ccrun` round-trip
discipline without re-testing anything the first set already covers. The
canonical manifests are left untouched; the stress set is purely additional and
lives in its own subdirectory so the two never mix.

| type | crawls swept | domain cap | complete | truncated |
| ---- | -----------: | ---------: | -------: | --------: |
| docx | 6 available  | 15         | 10000    | 52        |
| xlsx | 6 available  | 15         | 10000    | 80        |

It was swept from the same six crawls as the pptx set (CC-MAIN-2026-25 / -21 /
-17 / -12 / -08 / -04), deduplicated across them by `content_digest`, at the
same `-d 15` per-registered-domain cap as the canonical docx/xlsx set — but with
**every canonical docx/xlsx digest excluded up front**. docx and xlsx are so
abundant (the newest crawl alone holds ~61k distinct docx and ~34k distinct
xlsx) that even after removing the ~23k canonical digests the first crawl still
yields well over 10k *new* distinct of each type, so the sweep hit the target
and stopped early on CC-MAIN-2026-25 — the remaining five crawls were available
as deeper sources but not needed (the same early-stop the canonical docx/xlsx
set took). Truncated (>1 MiB) candidates are much rarer once the canonical set
is excluded, hence the small truncated counts.

**Two new `sweep-multi.sh` flags make this reproducible:**

- `-T <types>` — a comma-separated subset of `pptx,xlsx,docx` to sweep and emit
  (default all three). The stress sweep used `-T docx,xlsx`, so pptx MIME rows
  were never scanned, emitted, or counted toward the early-stop.
- `-x <digest-file>` — a newline-delimited list of `content_digest`s to exclude
  from every emitted manifest **and** from the early-stop count (a DuckDB
  anti-join). This is what guarantees zero overlap with the canonical set.

Regenerate the exclusion list from the committed canonical manifests, then
sweep into the `stress/` subdirectory:

```sh
# 1. every canonical docx/xlsx digest -> exclusion file (not committed)
tail -q -n+2 manifest-docx.tsv manifest-docx-truncated.tsv \
             manifest-xlsx.tsv manifest-xlsx-truncated.tsv \
  | cut -f6 | sort -u > /tmp/exclude-digests.txt

# 2. sweep docx+xlsx across the six crawls, cap 15, excluding the first set
mkdir -p stress
bash sweep-multi.sh -t 10000 -d 15 -T docx,xlsx -x /tmp/exclude-digests.txt \
  -o stress -w /tmp/spine-stress-work \
  CC-MAIN-2026-25 CC-MAIN-2026-21 CC-MAIN-2026-17 \
  CC-MAIN-2026-12 CC-MAIN-2026-08 CC-MAIN-2026-04
```

`ccrun`/`ccfetch` read the stress manifests exactly like the canonical ones —
point `-manifest` at `testdata/cc/stress` and give the run its **own** ledger
and quarantine so the two corpora stay separate:

`make harvest-batch` hardcodes `-manifest testdata/cc`, so invoke the runner
directly with the same resource-capped scope (loop it until a batch reports
`0 remaining`):

```sh
go build -o testdata/corpus/cc-stress/ccrun ./tools/ccrun
systemd-run --user --scope -p MemoryMax=2G -p CPUQuota=200% -p OOMPolicy=continue \
  testdata/corpus/cc-stress/ccrun \
  -manifest   testdata/cc/stress \
  -ledger     testdata/corpus/cc-stress/ledger.tsv \
  -quarantine testdata/cc/stress/batch-quarantine.tsv \
  -scratch    testdata/corpus/cc-stress/scratch \
  -batch 2000 -workers 2 -timeout 90s
```

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
systemd-run --user --scope -p MemoryMax=2G -p CPUQuota=200% -p OOMPolicy=continue \
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

**`OOMPolicy=continue` is what makes that true**, and it is the one place where
the reasoning inverts relative to `make test-race`. systemd's default
(`DefaultOOMPolicy=stop`) kills the *entire unit* when the kernel OOM-kills any
process inside its cgroup. For test-race that is the containment we want; here
it would kill the orchestrator together with the worker — and the orchestrator
records the kill *after* the worker dies, so the offending reference would never
reach the ledger and never count as an attempt. The resume loop below would then
re-select the same file on every batch, forever. Measured on systemd 255.4 with
a scope whose child blows a 128M `MemoryMax`: with the default policy the scope
leader is SIGTERMed and never reaches its post-worker line (exit 143); with
`OOMPolicy=continue` it observes the worker's 137 and carries on (exit 0).
`make harvest-batch` sets it; a hand-rolled invocation must too.

### Resumable ledger

The ledger (`testdata/corpus/cc-batch/ledger.tsv`, gitignored — it lives under
the ignored `testdata/corpus/`) has one append-only row per processed
reference (`digest  outcome  stage  signature  timestamp`; outcome is `pass`,
`fail`, `skip`, or `resource`) and is flushed per row. On restart `ccrun` skips
every digest already present, so an interrupted or OOM-killed batch loses at
most the in-flight file.

**Fetch failures are classified permanent vs transient, and the retry is
capped, so a batch always makes forward progress.** Every fetch outcome is
bucketed before it touches the ledger:

- **Permanent** — a DNS NXDOMAIN / no-such-host, a connection refused, a TLS or
  certificate failure, an HTTP 4xx (404/410/403/401 and most others), a blocked
  or dead DoH-gate verdict, an over-cap body, or a payload that is **not a valid
  OOXML package** (see below). A permanent failure is written to the ledger
  **immediately** as a terminal `fail` at `stage=fetch` with a specific
  signature (`fetch:dns`, `fetch:http-404`, `fetch:not-ooxml`, `fetch:tls`,
  `fetch:conn-refused`, …) and is **never retried**. A dead origin therefore
  terminates on its first attempt instead of being re-selected every batch.
- **Transient** — a timeout, HTTP 429, an HTTP 5xx, a connection reset, or a
  temporary DNS failure (SERVFAIL). A transient failure is **deferred** (left
  out of the ledger so a later batch retries it), but only up to
  `maxFetchAttempts` (3). The count is persisted in a sidecar
  (`testdata/corpus/cc-batch/attempts.tsv`, gitignored) so the cap survives
  restarts; once it is reached the reference is retired terminally as
  `fetch:transient-exhausted` — with no further fetch — so the queue strictly
  drains rather than looping for weeks on a flaky tail. (A WARC-CDN throttling
  403 is transient on the archive path but a permanent refusal at a live
  origin; the low-level layer's path-specific retryable flag disambiguates.)

**Non-OOXML bodies are fetch failures, not open failures.** A dead origin can
answer a live refetch with an HTML error page or a login redirect (HTTP 200
with a non-file body). Before handing any fetched payload to the library `Open`,
the worker validates it is a real OPC package (starts with `PK\x03\x04`, opens
as a zip, contains `[Content_Types].xml`); a body that fails is recorded as a
permanent `fetch:not-ooxml` at `stage=fetch`, not as a spurious `open` failure,
and no `Open` attempt is wasted on it. Resume loop:

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
quarantine references files that were already discarded.

**It is committed** (the header-only seed is in the tree; `ccrun` appends to
it), on the same grounds as the manifests next to it: it holds references only
— digest, crawl, the query-stripped url, stage, signature — with no payload and
no credentials, and it is the *only* durable output of the harvest that is not
machine-local. "Regenerable" is technically true and practically false: a
re-run costs days of fetching and cannot recover the files the web has since
lost, so an uncommitted catalog would exist on exactly one machine. The
progress state — the ledger and the attempts sidecar under
`testdata/corpus/cc-batch/` — stays gitignored: it is per-machine bookkeeping,
carries no findings, and is genuinely rebuilt by re-running.

To debug a quarantined reference, refetch it to a named path (it is **not**
deleted):

```sh
go run ./tools/ccrun -refetch <content_digest> -manifest testdata/cc -out /tmp/bad.docx
```

### Scaling beyond 10k

Raise `-t` on `sweep-multi.sh` and list more crawls (`make harvest-sweep
HARVEST_TARGET=20000 HARVEST_CRAWLS="CC-MAIN-2026-25 CC-MAIN-2026-21 …"`),
commit the larger manifests, and keep looping `harvest-batch` — the runner is
unchanged. More workers or a larger `MemoryMax` trade throughput for headroom;
the isolation model is identical at any size.

## Scaling up

- More files per type: raise the fetcher limit (`go run ./tools/ccfetch ...
  -n 3000`). The committed manifests carry ~3x margin over the default goal.
- Beyond the manifest margin: rerun `sweep.sh -t <bigger target>` to emit
  larger manifests (pptx is scarce — roughly one crawl's warc subset yields
  only a few thousand distinct candidates).
- Newer crawls: `sweep.sh -c CC-MAIN-<yyyy-ww>` regenerates the manifests
  from another crawl; commit the new manifests to repin the corpus.
