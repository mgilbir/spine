package opc

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestLegacyBinaryIsNotReportedAsEncrypted is the D54 regression: an open
// recognized any compound (CFB) file as an encrypted OOXML package, so a
// legacy binary .doc, .xls or .ppt returned ErrEncrypted — and a caller asked
// the user for a password that does not exist. With a password it then failed
// as a corrupted package. Such a file now returns ErrLegacyBinary either way,
// naming the format when its main stream says which.
func TestLegacyBinaryIsNotReportedAsEncrypted(t *testing.T) {
	for _, tc := range []struct{ stream, kind string }{
		{"WordDocument", "Word"},
		{"Workbook", "Excel"},
		{"Book", "Excel"},
		{"PowerPoint Document", "PowerPoint"},
		{"Contents", ""},
	} {
		var cfb bytes.Buffer
		if err := writeCFB(&cfb, []cfbStream{{name: tc.stream, data: bytes.Repeat([]byte{1}, 600)}}); err != nil {
			t.Fatal(err)
		}
		data := cfb.Bytes()
		for _, opts := range [][]ReaderOption{nil, {WithPassword("anything")}} {
			_, err := NewReader(bytes.NewReader(data), int64(len(data)), opts...)
			if !errors.Is(err, ErrLegacyBinary) {
				t.Errorf("%s stream, %d options: err = %v, want ErrLegacyBinary", tc.stream, len(opts), err)
				continue
			}
			if errors.Is(err, ErrEncrypted) {
				t.Errorf("%s stream: a legacy file still matches ErrEncrypted", tc.stream)
			}
			if tc.kind != "" && !strings.Contains(err.Error(), tc.kind) {
				t.Errorf("%s stream: %q does not name %s", tc.stream, err, tc.kind)
			}
		}
	}

	// A real encrypted package is still ErrEncrypted without a password.
	var enc bytes.Buffer
	if err := SaveEncrypted(&enc, buildMinimalPackage(t), "pw"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewReader(bytes.NewReader(enc.Bytes()), int64(enc.Len())); !errors.Is(err, ErrEncrypted) || errors.Is(err, ErrLegacyBinary) {
		t.Errorf("encrypted package without a password: %v, want ErrEncrypted only", err)
	}
}
