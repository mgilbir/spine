package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// maxFetchAttempts caps how many times a reference that keeps failing
// transiently (CDN throttling, timeout, 429/5xx, temporary DNS) is deferred
// before it is converted to a terminal fetch:transient-exhausted outcome and
// leaves the queue. A permanent failure (dead host, 4xx, non-OOXML body)
// terminates on the first attempt; this cap only bounds the genuinely flaky
// tail so a batch always makes forward progress and the queue strictly drains.
const maxFetchAttempts = 3

// Attempts is the durable per-reference transient-failure counter kept beside
// the ledger (attempts.tsv). A deferred reference is retried in a later batch,
// but only until its count reaches maxFetchAttempts; persisting the count makes
// the cap survive process restarts so a dead origin cannot loop forever.
//
// It is append-only (one `digest<TAB>count` row per bump) and flushed per bump;
// on open the highest count seen per digest wins, so a crash between the file
// write and the next batch never lowers a count.
type Attempts struct {
	f     *os.File
	count map[string]int
}

// openAttempts opens (creating if needed) the counter file at path and replays
// it into the in-memory map.
func openAttempts(path string) (*Attempts, error) {
	a := &Attempts{count: map[string]int{}}
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
			if len(fields) != 2 {
				continue
			}
			n, err := strconv.Atoi(strings.TrimSpace(fields[1]))
			if err != nil {
				continue
			}
			if n > a.count[fields[0]] {
				a.count[fields[0]] = n
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	if st, err := fh.Stat(); err == nil && st.Size() == 0 {
		if _, err := fh.WriteString("digest\tattempts\n"); err != nil {
			_ = fh.Close()
			return nil, err
		}
	}
	a.f = fh
	return a, nil
}

// Count reports how many transient failures digest has accumulated so far.
func (a *Attempts) Count(digest string) int { return a.count[digest] }

// Bump records one more transient failure for digest and returns the new count.
func (a *Attempts) Bump(digest string) (int, error) {
	n := a.count[digest] + 1
	a.count[digest] = n
	if a.f != nil {
		if _, err := fmt.Fprintf(a.f, "%s\t%d\n", digest, n); err != nil {
			return n, err
		}
	}
	return n, nil
}

// Exhausted reports whether digest has reached the transient retry cap and must
// now be terminated instead of deferred again.
func (a *Attempts) Exhausted(digest string) bool {
	return a.count[digest] >= maxFetchAttempts
}

// Close closes the underlying file.
func (a *Attempts) Close() error {
	if a.f == nil {
		return nil
	}
	return a.f.Close()
}
