package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

// The live-fetch path (truncated candidates refetched from their origin)
// resolves every contacted host through a filtering DNS-over-HTTPS resolver
// (RFC 8484) before any HTTP request is made. The resolver URL is supplied
// at runtime (-doh-url flag or SPINE_DOH_URL env var; e.g. a NextDNS profile
// endpoint) and is never baked into the repository. A host whose answer is
// the unspecified address (0.0.0.0 / ::) is blocked by the resolver's
// filter and skipped; NXDOMAIN/SERVFAIL hosts are dead and equally skipped.

type verdict int

const (
	verdictAllow verdict = iota
	verdictBlocked
	verdictDead
)

func (v verdict) String() string {
	switch v {
	case verdictAllow:
		return "allow"
	case verdictBlocked:
		return "blocked"
	default:
		return "dead"
	}
}

// Sentinel errors surfaced from redirect re-gating, so the caller can
// classify a blocked/dead redirect target (http.Client wraps them in
// *url.Error, which errors.Is unwraps).
var (
	errGateBlocked = errors.New("host is blocked by the DNS filter")
	errGateDead    = errors.New("host does not resolve")
)

// encodeDNSQuery builds a wire-format DNS query (RD set, one question) for
// an A record of host.
func encodeDNSQuery(host string) ([]byte, error) {
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return nil, errors.New("dns: empty host")
	}
	var b bytes.Buffer
	// ID 0 (recommended for DoH cacheability), flags RD, QDCOUNT 1.
	b.Write([]byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return nil, fmt.Errorf("dns: bad label in host %q", host)
		}
		b.WriteByte(byte(len(label)))
		b.WriteString(label)
	}
	b.WriteByte(0x00)
	b.Write([]byte{0x00, 0x01, 0x00, 0x01}) // QTYPE=A, QCLASS=IN
	return b.Bytes(), nil
}

// skipName advances past a (possibly compressed) DNS name starting at off
// and returns the offset of the following byte.
func skipName(msg []byte, off int) (int, error) {
	for {
		if off >= len(msg) {
			return 0, errors.New("dns: truncated name")
		}
		b := msg[off]
		switch {
		case b == 0x00:
			return off + 1, nil
		case b&0xC0 == 0xC0: // compression pointer: two bytes, ends the name
			if off+2 > len(msg) {
				return 0, errors.New("dns: truncated compression pointer")
			}
			return off + 2, nil
		case b&0xC0 != 0:
			return 0, fmt.Errorf("dns: reserved label type 0x%02x", b&0xC0)
		default:
			off += 1 + int(b)
		}
	}
}

// parseDNSResponse returns the response code and every A/AAAA address in the
// answer section. Non-address records (e.g. CNAMEs) are skipped.
func parseDNSResponse(msg []byte) (rcode int, addrs []netip.Addr, err error) {
	if len(msg) < 12 {
		return 0, nil, errors.New("dns: message shorter than header")
	}
	rcode = int(msg[3] & 0x0F)
	qdCount := int(binary.BigEndian.Uint16(msg[4:6]))
	anCount := int(binary.BigEndian.Uint16(msg[6:8]))

	off := 12
	for i := 0; i < qdCount; i++ {
		if off, err = skipName(msg, off); err != nil {
			return 0, nil, err
		}
		off += 4 // QTYPE + QCLASS
	}
	for i := 0; i < anCount; i++ {
		if off, err = skipName(msg, off); err != nil {
			return 0, nil, err
		}
		if off+10 > len(msg) {
			return 0, nil, errors.New("dns: truncated resource record")
		}
		rrType := binary.BigEndian.Uint16(msg[off : off+2])
		rdLen := int(binary.BigEndian.Uint16(msg[off+8 : off+10]))
		off += 10
		if off+rdLen > len(msg) {
			return 0, nil, errors.New("dns: truncated rdata")
		}
		switch {
		case rrType == 1 && rdLen == 4: // A
			addrs = append(addrs, netip.AddrFrom4([4]byte(msg[off:off+4])))
		case rrType == 28 && rdLen == 16: // AAAA
			addrs = append(addrs, netip.AddrFrom16([16]byte(msg[off:off+16])))
		}
		off += rdLen
	}
	return rcode, addrs, nil
}

// classifyAnswer maps a parsed DNS response to a gate verdict.
func classifyAnswer(rcode int, addrs []netip.Addr) verdict {
	if rcode != 0 { // NXDOMAIN, SERVFAIL, ...
		return verdictDead
	}
	if len(addrs) == 0 {
		return verdictDead
	}
	for _, a := range addrs {
		if a.IsUnspecified() { // filtering resolvers answer 0.0.0.0 / :: for blocked hosts
			return verdictBlocked
		}
	}
	return verdictAllow
}

// hostGate resolves hosts through the configured DoH endpoint and caches the
// verdict per host for the length of the run.
type hostGate struct {
	client   *http.Client
	endpoint string

	mu    sync.Mutex
	cache map[string]verdict
}

func newHostGate(client *http.Client, endpoint string) *hostGate {
	return &hostGate{client: client, endpoint: endpoint, cache: map[string]verdict{}}
}

func (g *hostGate) check(ctx context.Context, host string) (verdict, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	g.mu.Lock()
	if v, ok := g.cache[host]; ok {
		g.mu.Unlock()
		return v, nil
	}
	g.mu.Unlock()

	query, err := encodeDNSQuery(host)
	if err != nil {
		return verdictDead, err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint, bytes.NewReader(query))
		if err != nil {
			return verdictDead, err
		}
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")
		req.Header.Set("User-Agent", userAgent)

		resp, err := g.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("doh: status %s", resp.Status)
			continue
		}
		rcode, addrs, err := parseDNSResponse(body)
		if err != nil {
			lastErr = err
			continue
		}
		v := classifyAnswer(rcode, addrs)
		g.mu.Lock()
		g.cache[host] = v
		g.mu.Unlock()
		return v, nil
	}
	return verdictDead, fmt.Errorf("doh: query for %s failed: %w", host, lastErr)
}
