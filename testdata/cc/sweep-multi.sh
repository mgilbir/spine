#!/usr/bin/env bash
# sweep-multi.sh — sweep MULTIPLE Common Crawl crawls for OOXML documents and
# emit reference-only manifests deduplicated across all of them.
#
# This is the scaled sibling of sweep.sh. A single crawl's warc subset yields
# only a few thousand distinct pptx candidates, so reaching a 10k/type corpus
# needs several recent crawls swept together and deduplicated globally by
# content_digest. Each output row is a *self-contained reference*: a leading
# `crawl` column is prepended so a row names the crawl it came from without any
# external index.
#
#   manifest-{pptx,xlsx,docx}.tsv            complete payloads (WARC-fetchable)
#   manifest-{pptx,xlsx,docx}-truncated.tsv  payloads Common Crawl truncated at
#                                            1 MiB; refetched from the live web
#
# Columns: crawl, url, warc_filename, warc_record_offset, warc_record_length,
# content_digest. The manifests are committed; tools/ccrun (or tools/ccfetch)
# turns them into a transient local corpus — only the references live in git.
#
# Usage: sweep-multi.sh [-t TARGET] [-o OUTDIR] [-d DOMAIN_CAP] [-w WORKDIR] [CRAWL ...]
#   -t  distinct candidates to keep per complete manifest (default 10000)
#   -o  output directory for the manifests (default: this script's directory)
#   -d  per-registered-domain cap applied globally (default 5)
#   -p  pptx-only per-registered-domain cap (default: same as -d). pptx is
#       scarcer, so it can take a higher cap without touching the other types.
#   -w  persistent work dir for the accumulating DuckDB db + scan progress
#       (default: $TMPDIR/spine-sweep-work). See "Resumable" below.
#   CRAWL ...  crawl ids to sweep, most-recent first. With none given, a recent
#              default list is used. Crawls are swept in order and scanning
#              stops early once every complete manifest has reached TARGET
#              distinct candidates, so listing extras is harmless.
#
# Resumable: sweeping many crawls guarantees heavy CDN throttling and can take
# a long time, so progress is durable. The accumulated candidate table lives in
# WORKDIR/sweep.db and every scanned batch is recorded in WORKDIR/done.txt; a
# rerun with the same -w skips completed batches and continues where it left
# off (each parquet-scan INSERT is atomic, so an interrupted run loses at most
# the in-flight batch). Re-run the script until it prints "manifests written".
# Delete WORKDIR to start clean.
#
# Get the current crawl list from https://index.commoncrawl.org/collinfo.json.
#
# Requires: duckdb (v1.x CLI), curl, gzip.

set -euo pipefail

TARGET=10000
OUTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOMAIN_CAP=5
PPTX_CAP=""     # optional pptx-only domain cap; empty => use DOMAIN_CAP
WORKDIR="${TMPDIR:-/tmp}/spine-sweep-work"
BATCH=10        # index parts per DuckDB invocation
MAX_RETRIES=12  # per-batch retries: sweeping many crawls guarantees heavy
                # data.commoncrawl.org throttling (403/503/500), so be patient

while getopts "t:o:d:p:w:h" opt; do
  case "$opt" in
    t) TARGET="$OPTARG" ;;
    o) OUTDIR="$(cd "$OPTARG" && pwd)" ;;
    d) DOMAIN_CAP="$OPTARG" ;;
    p) PPTX_CAP="$OPTARG" ;;
    w) WORKDIR="$OPTARG" ;;
    h) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "usage: $0 [-t target] [-o outdir] [-d domain_cap] [-p pptx_cap] [-w workdir] [CRAWL ...]" >&2; exit 2 ;;
  esac
done
shift $((OPTIND - 1))
: "${PPTX_CAP:=$DOMAIN_CAP}"

CRAWLS=("$@")
if [ "${#CRAWLS[@]}" -eq 0 ]; then
  CRAWLS=(CC-MAIN-2026-25 CC-MAIN-2026-21 CC-MAIN-2026-17 \
          CC-MAIN-2026-12 CC-MAIN-2026-08 CC-MAIN-2026-04 \
          CC-MAIN-2025-51 CC-MAIN-2025-47)
fi

command -v duckdb >/dev/null || { echo "error: duckdb CLI not found" >&2; exit 1; }

# Per-run scratch (part lists, batch splits) is disposable; the accumulating
# candidate DB and scan-progress ledger persist in WORKDIR so a killed run
# resumes.
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORKDIR"
DB="$WORKDIR/sweep.db"
DONE="$WORKDIR/done.txt"
touch "$DONE"

MIME_PPTX="application/vnd.openxmlformats-officedocument.presentationml.presentation"
MIME_XLSX="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
MIME_DOCX="application/vnd.openxmlformats-officedocument.wordprocessingml.document"

# httpfs + retry settings prepended to every DuckDB invocation.
SETTINGS="INSTALL httpfs; LOAD httpfs;
SET http_retries = 6;
SET http_retry_wait_ms = 1000;
SET http_retry_backoff = 2;"

echo "== crawls: ${CRAWLS[*]}"
echo "== target/type: $TARGET  domain cap: $DOMAIN_CAP (pptx: $PPTX_CAP)  out: $OUTDIR  work: $WORKDIR"
if [ -s "$DONE" ]; then
  echo "== resuming: $(wc -l < "$DONE") batches already scanned"
fi

duckdb "$DB" "CREATE TABLE IF NOT EXISTS candidates (
  crawl VARCHAR, url VARCHAR, url_host_registered_domain VARCHAR,
  content_digest VARCHAR, mime VARCHAR, warc_filename VARCHAR,
  warc_record_offset BIGINT, warc_record_length BIGINT, is_truncated BOOLEAN);"

# enough_complete reports whether every complete-payload type has reached
# TARGET distinct content_digests, so we can stop sweeping further crawls.
enough_complete() {
  local n
  n="$(duckdb "$DB" -noheader -list "
    SELECT count(*) FROM (
      SELECT mime FROM candidates
      WHERE NOT is_truncated
        AND mime IN ('$MIME_PPTX', '$MIME_XLSX', '$MIME_DOCX')
      GROUP BY mime
      HAVING count(DISTINCT content_digest) >= $TARGET
    );")"
  [ "$n" = "3" ]
}

for CRAWL in "${CRAWLS[@]}"; do
  if enough_complete; then
    echo "== all complete types already at $TARGET distinct; skipping $CRAWL"
    continue
  fi
  echo "== sweeping $CRAWL"
  # 1. Part list for the warc subset of the columnar index (throttled: retry).
  attempt=1
  until curl -sSfL "https://data.commoncrawl.org/crawl-data/${CRAWL}/cc-index-table.paths.gz" \
          -o "$WORK/paths.gz"; do
    if [ "$attempt" -ge "$MAX_RETRIES" ]; then
      echo "error: could not download the cc-index part list for $CRAWL" >&2
      exit 1
    fi
    sleep $((15 * attempt))
    attempt=$((attempt + 1))
    echo "   part list: retry $attempt"
  done
  gzip -dc "$WORK/paths.gz" | grep "/subset=warc/" > "$WORK/parts.txt"
  NPARTS="$(wc -l < "$WORK/parts.txt")"
  echo "   $NPARTS warc-subset parquet parts"

  # 2. Scan the parts in small batches, each retried on transient HTTP errors.
  # A single INSERT is atomic in DuckDB, so a failed batch leaves no partial
  # rows and can simply be rerun. Predicate pushdown keeps each batch cheap.
  # Complete payloads are selected by the *detected* MIME (Tika sniffing the
  # bytes); truncated payloads have no central directory to sniff, so they are
  # selected by the *served* Content-Type instead.
  rm -f "$WORK"/batch-*
  batch_no=0
  split -l "$BATCH" "$WORK/parts.txt" "$WORK/batch-"
  for batch in "$WORK"/batch-*; do
    batch_no=$((batch_no + 1))
    # Resume: skip a batch already recorded as scanned in a previous run.
    key="$CRAWL batch $batch_no"
    if grep -qxF "$key" "$DONE"; then
      continue
    fi
    {
      echo "$SETTINGS"
      echo "INSERT INTO candidates"
      echo "SELECT '$CRAWL', url, url_host_registered_domain, content_digest,"
      echo "       CASE WHEN content_truncated IS NULL"
      echo "            THEN content_mime_detected ELSE content_mime_type END,"
      echo "       warc_filename,"
      echo "       warc_record_offset, warc_record_length,"
      echo "       content_truncated IS NOT NULL"
      echo "FROM read_parquet(["
      sed -e "s|^|  'https://data.commoncrawl.org/|" -e "s|\$|',|" "$batch" \
        | sed -e '$ s/,$//'
      echo "])"
      echo "WHERE fetch_status = 200"
      echo "  AND ((content_truncated IS NULL AND content_mime_detected IN ('$MIME_PPTX', '$MIME_XLSX', '$MIME_DOCX'))"
      echo "    OR (content_truncated IS NOT NULL AND content_mime_type IN ('$MIME_PPTX', '$MIME_XLSX', '$MIME_DOCX')))"
      # Manifests are unquoted TSV: drop the vanishingly rare URL that would
      # break that framing.
      echo "  AND NOT regexp_matches(url, '[\\t\\n\\r\"]');"
    } > "$WORK/batch.sql"

    attempt=1
    until duckdb "$DB" < "$WORK/batch.sql" > /dev/null; do
      if [ "$attempt" -ge "$MAX_RETRIES" ]; then
        echo "error: $CRAWL batch $batch_no failed after $MAX_RETRIES attempts" >&2
        exit 1
      fi
      # Growing backoff capped at 60s, plus jitter, to ride out throttling.
      wait=$((10 * attempt)); [ "$wait" -gt 60 ] && wait=60
      sleep $((wait + RANDOM % 10))
      attempt=$((attempt + 1))
      echo "   batch $batch_no: retry $attempt"
    done
    # Record only after the atomic INSERT committed, so a kill mid-batch
    # leaves the batch un-done and it is re-scanned on the next run.
    echo "$key" >> "$DONE"
    if [ $((batch_no % 10)) -eq 0 ]; then
      echo "   batch $batch_no/$(( (NPARTS + BATCH - 1) / BATCH )) done"
    fi
  done

  duckdb "$DB" "SELECT '$CRAWL' AS crawl, mime, is_truncated,
         count(DISTINCT content_digest) AS running_distinct
  FROM candidates GROUP BY 2, 3 ORDER BY 2, 3;"

  if enough_complete; then
    echo "== all complete types reached $TARGET distinct candidates; stopping early"
    break
  fi
done

# 3. Per-type dedup (global, by content_digest) / domain cap (global) / stable
# cut — once for complete payloads and once for truncated ones. The winning
# reference for a digest is chosen by a deterministic order, independent of
# which crawl contributed it.
for spec in "pptx:$MIME_PPTX" "xlsx:$MIME_XLSX" "docx:$MIME_DOCX"; do
  ext="${spec%%:*}"; mime="${spec#*:}"
  # pptx is scarcer than xlsx/docx, so it accepts its own (usually higher)
  # per-domain cap: distinct pptx are diversity-limited, and a higher cap trades
  # some source diversity for volume without affecting the target-limited types.
  cap="$DOMAIN_CAP"; [ "$ext" = "pptx" ] && cap="$PPTX_CAP"
  for variant in "complete" "truncated"; do
    if [ "$variant" = "truncated" ]; then
      suffix="-truncated"; pred="is_truncated"
    else
      suffix=""; pred="NOT is_truncated"
    fi
    duckdb "$DB" <<SQL
COPY (
  WITH dedup AS (
    SELECT *, row_number() OVER (
      PARTITION BY content_digest
      ORDER BY url, warc_filename, warc_record_offset
    ) AS rn
    FROM candidates
    WHERE mime = '$mime' AND $pred
  ),
  capped AS (
    SELECT *, row_number() OVER (
      PARTITION BY coalesce(url_host_registered_domain, '')
      ORDER BY url, content_digest
    ) AS dn
    FROM dedup
    WHERE rn = 1
  )
  -- Strip the entire query string from the url. Crawled presigned S3/MinIO
  -- document URLs carry AWS credentials there (AWSAccessKeyId / X-Amz-Signature
  -- / Signature=), which must never be committed or redistributed. The query is
  -- not needed to fetch: WARC-complete refs read by warc_filename+offset+length
  -- and are verified by content_digest, and a truncated live-refetch presigned
  -- URL has almost always expired anyway (documented in README). Rows are kept
  -- (sanitized), not dropped, so counts are unaffected. A vanishingly rare
  -- access-key id embedded in the path (not the query) still drops the row.
  SELECT crawl, regexp_replace(url, '[?].*', '') AS url,
         warc_filename, warc_record_offset, warc_record_length, content_digest
  FROM capped
  WHERE dn <= $cap
    AND NOT regexp_matches(regexp_replace(url, '[?].*', ''), '(AKIA|ASIA)[0-9A-Z]{16}')
  ORDER BY content_digest, url
  LIMIT $TARGET
) TO '$OUTDIR/manifest-$ext$suffix.tsv' (FORMAT csv, DELIMITER '\\t', HEADER, QUOTE '');
SQL
  done
done

echo "== manifests written:"
for ext in pptx xlsx docx pptx-truncated xlsx-truncated docx-truncated; do
  f="$OUTDIR/manifest-$ext.tsv"
  rows=$(( $(wc -l < "$f") - 1 ))
  echo "   $f: $rows candidates"
done
