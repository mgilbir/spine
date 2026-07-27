package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Ledger outcomes.
const (
	outcomePass     = "pass"
	outcomeFail     = "fail"
	outcomeResource = "resource" // worker killed/timed-out/panicked
	// outcomeRetry is a transient fetch failure. It is NOT written to the
	// ledger, so a later run retries the reference instead of burning it.
	outcomeRetry = "retry"
)

// Ledger is the durable per-reference progress journal: one row per processed
// reference so a rerun resumes where it left off. Columns:
//
//	digest  outcome  stage  signature  timestamp
//
// It is append-only and flushed per row so an interrupted run never loses more
// than the in-flight reference.
type Ledger struct {
	f    *os.File
	done map[string]bool
}

// openLedger opens (creating if needed) the ledger at path and replays it into
// the done set.
func openLedger(path string) (*Ledger, error) {
	l := &Ledger{done: map[string]bool{}}
	if data, err := os.ReadFile(path); err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(data)))
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		first := true
		for sc.Scan() {
			line := sc.Text()
			if first {
				first = false
				if strings.HasPrefix(line, "digest\t") {
					continue
				}
			}
			if line == "" {
				continue
			}
			fields := strings.SplitN(line, "\t", 2)
			l.done[fields[0]] = true
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if st, err := fh.Stat(); err == nil && st.Size() == 0 {
		if _, err := fh.WriteString("digest\toutcome\tstage\tsignature\ttimestamp\n"); err != nil {
			_ = fh.Close()
			return nil, err
		}
	}
	l.f = fh
	return l, nil
}

// Has reports whether digest has already been processed.
func (l *Ledger) Has(digest string) bool { return l.done[digest] }

// Append records one processed reference. stage/signature are empty for a pass.
func (l *Ledger) Append(digest, outcome, stage, signature string, now time.Time) error {
	l.done[digest] = true
	_, err := fmt.Fprintf(l.f, "%s\t%s\t%s\t%s\t%s\n",
		digest, outcome, stage, signature, now.UTC().Format(time.RFC3339))
	return err
}

// Close closes the underlying file.
func (l *Ledger) Close() error {
	if l.f == nil {
		return nil
	}
	return l.f.Close()
}

// Quarantine is the growing, reference-keyed failure catalog shared across
// batches and runs. Columns:
//
//	digest  crawl  url  stage  signature
//
// Unlike cctest's sha16-on-disk quarantine (whose files are kept), these rows
// reference files that have already been discarded; re-fetch one with
// `ccrun -refetch <digest>` to debug it.
type Quarantine struct {
	f *os.File
}

// openQuarantine opens (creating if needed) the quarantine at path.
func openQuarantine(path string) (*Quarantine, error) {
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if st, err := fh.Stat(); err == nil && st.Size() == 0 {
		if _, err := fh.WriteString("digest\tcrawl\turl\tstage\tsignature\n"); err != nil {
			_ = fh.Close()
			return nil, err
		}
	}
	return &Quarantine{f: fh}, nil
}

// Append records one failed reference.
func (q *Quarantine) Append(r Ref, stage, signature string) error {
	_, err := fmt.Fprintf(q.f, "%s\t%s\t%s\t%s\t%s\n",
		r.Digest, r.Crawl, r.URL, stage, signature)
	return err
}

// Close closes the underlying file.
func (q *Quarantine) Close() error {
	if q.f == nil {
		return nil
	}
	return q.f.Close()
}
