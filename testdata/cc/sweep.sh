#!/usr/bin/env bash
# sweep.sh — sweep the Common Crawl columnar index for OOXML documents.
#
# Queries the cc-index parquet table (via DuckDB + httpfs) for pptx/xlsx/docx
# responses and writes per-type candidate manifests next to this script:
#
#   manifest-{pptx,xlsx,docx}.tsv            complete payloads (WARC-fetchable)
#   manifest-{pptx,xlsx,docx}-truncated.tsv  payloads Common Crawl truncated at
#                                            1 MiB; refetched from the live web
#                                            by the fetcher's gated live mode
#
# Columns: url, warc_filename, warc_record_offset, warc_record_length,
# content_digest. The manifests are committed; the fetcher (tools/ccfetch)
# turns them into a local, gitignored corpus. Contributors never need to run
# this script — only rerun it to refresh the manifests from a newer crawl.
#
# Usage: sweep.sh [-c CRAWL_ID] [-t TARGET_PER_TYPE] [-o OUTDIR]
#   -c  crawl ID (default CC-MAIN-2026-25)
#   -t  candidates to keep per type (default 3000; ~3x the fetch goal so the
#       fetcher can lose rows to dead WARC records and still hit its target)
#   -o  output directory for the manifests (default: this script's directory)
#
# Requires: duckdb (v1.x CLI), curl, gzip.

set -euo pipefail

CRAWL="CC-MAIN-2026-25"
TARGET=3000
OUTDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOMAIN_CAP=5
BATCH=10        # index parts per DuckDB invocation
MAX_RETRIES=5   # per-batch retries (data.commoncrawl.org throttles with 403/503)

while getopts "c:t:o:h" opt; do
  case "$opt" in
    c) CRAWL="$OPTARG" ;;
    t) TARGET="$OPTARG" ;;
    o) OUTDIR="$(cd "$OPTARG" && pwd)" ;;
    h) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "usage: $0 [-c crawl] [-t target] [-o outdir]" >&2; exit 2 ;;
  esac
done

command -v duckdb >/dev/null || { echo "error: duckdb CLI not found" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
DB="$WORK/sweep.db"

MIME_PPTX="application/vnd.openxmlformats-officedocument.presentationml.presentation"
MIME_XLSX="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
MIME_DOCX="application/vnd.openxmlformats-officedocument.wordprocessingml.document"

# httpfs + retry settings prepended to every DuckDB invocation.
SETTINGS="INSTALL httpfs; LOAD httpfs;
SET http_retries = 6;
SET http_retry_wait_ms = 1000;
SET http_retry_backoff = 2;"

echo "== crawl: $CRAWL  target/type: $TARGET  domain cap: $DOMAIN_CAP  out: $OUTDIR"

# 1. Part list for the warc subset of the columnar index. Common Crawl
# throttles with 403/503, so retry with a growing pause.
attempt=1
until curl -sSfL "https://data.commoncrawl.org/crawl-data/${CRAWL}/cc-index-table.paths.gz" \
        -o "$WORK/paths.gz"; do
  if [ "$attempt" -ge "$MAX_RETRIES" ]; then
    echo "error: could not download the cc-index part list" >&2
    exit 1
  fi
  sleep $((15 * attempt))
  attempt=$((attempt + 1))
  echo "   part list: retry $attempt"
done
gzip -dc "$WORK/paths.gz" | grep "/subset=warc/" > "$WORK/parts.txt"
NPARTS="$(wc -l < "$WORK/parts.txt")"
echo "== $NPARTS warc-subset parquet parts"

duckdb "$DB" "CREATE TABLE candidates (
  url VARCHAR, url_host_registered_domain VARCHAR, content_digest VARCHAR,
  mime VARCHAR, warc_filename VARCHAR,
  warc_record_offset BIGINT, warc_record_length BIGINT, is_truncated BOOLEAN);"

# 2. Scan the parts in small batches, each retried on transient HTTP errors.
# A single INSERT is atomic in DuckDB, so a failed batch leaves no partial
# rows behind and can simply be rerun. Predicate pushdown on the MIME/status
# columns keeps each batch cheap.
#
# Complete payloads are selected by the *detected* MIME (Tika sniffing the
# actual bytes). For truncated payloads detection is useless — a zip cut at
# 1 MiB has no central directory, so Tika reports x-tika-ooxml at best —
# hence truncated candidates are selected by the *served* Content-Type
# (content_mime_type) instead.
batch_no=0
split -l "$BATCH" "$WORK/parts.txt" "$WORK/batch-"
for batch in "$WORK"/batch-*; do
  batch_no=$((batch_no + 1))
  {
    echo "$SETTINGS"
    echo "INSERT INTO candidates"
    echo "SELECT url, url_host_registered_domain, content_digest,"
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
      echo "error: batch $batch_no failed after $MAX_RETRIES attempts" >&2
      exit 1
    fi
    sleep $((10 * attempt))
    attempt=$((attempt + 1))
    echo "   batch $batch_no: retry $attempt"
  done
  echo "   batch $batch_no/$(( (NPARTS + BATCH - 1) / BATCH )) done"
done

# 3. Candidate counts, then per-type dedup / domain cap / stable cut —
# once for complete payloads (WARC manifests) and once for truncated ones
# (live-refetch manifests).
duckdb "$DB" "SELECT mime, is_truncated, count(*) AS raw,
       count(DISTINCT content_digest) AS distinct_digests
FROM candidates GROUP BY 1, 2 ORDER BY 1, 2;"

for spec in "pptx:$MIME_PPTX" "xlsx:$MIME_XLSX" "docx:$MIME_DOCX"; do
  ext="${spec%%:*}"; mime="${spec#*:}"
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
  SELECT url, warc_filename, warc_record_offset, warc_record_length, content_digest
  FROM capped
  WHERE dn <= $DOMAIN_CAP
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
