package opc

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

// A Reader is safe for concurrent use, which is a promise this package makes in
// several places and tested in one.
//
// The decompression budget's accounting is exercised under contention already
// (TestBudgetChargesEachPartOnce), but that reaches only ReadAll. Everything a
// caller actually does with an open package — resolving a part by name,
// reaching its relationships, asking for the raw zip entry — goes through a
// lazily built index behind its own mutex, and nothing ran two of those at
// once. A lazily built map is the classic place for a data race to sit
// unnoticed: it is correct on every single-goroutine run, and the failure is a
// corrupted map or a torn read, not a wrong answer anyone would spot.
//
// The nightly race job runs this; without -race it still checks that concurrent
// lookups agree with sequential ones.
func TestReaderIsSafeForConcurrentUse(t *testing.T) {
	const parts = 32
	partsMap := make(map[string][]byte, parts)
	contentTypes := make(map[string]string, parts)
	for i := 0; i < parts; i++ {
		name := fmt.Sprintf("/ppt/slides/slide%d.xml", i)
		partsMap[name] = []byte(fmt.Sprintf("<slide n=%q/>", fmt.Sprint(i)))
		contentTypes[name] = "application/xml"
	}
	pkg := createTestPackage(t, partsMap, contentTypes)

	r, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// What each part must look like, established before any concurrency so a
	// mismatch below is attributable to it.
	want := make(map[string]int, parts)
	for name := range partsMap {
		f := r.GetFile(name)
		if f == nil {
			t.Fatalf("%s is missing from the package", name)
		}
		data, err := f.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll(%s): %v", name, err)
		}
		want[name] = len(data)
	}

	// A second reader over the same bytes, so the index is built under
	// contention rather than already warm from the loop above.
	r2, err := NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan string, parts*4)
	for i := 0; i < parts; i++ {
		for _, worker := range []func(string){
			func(name string) {
				f := r2.GetFile(name)
				if f == nil {
					errs <- fmt.Sprintf("GetFile(%s) found nothing", name)
					return
				}
				data, err := f.ReadAll()
				if err != nil {
					errs <- fmt.Sprintf("ReadAll(%s): %v", name, err)
					return
				}
				if len(data) != want[name] {
					errs <- fmt.Sprintf("ReadAll(%s) returned %d bytes, want %d", name, len(data), want[name])
				}
			},
			func(name string) {
				if _, err := r2.GetPartRelationships(name); err != nil {
					errs <- fmt.Sprintf("GetPartRelationships(%s): %v", name, err)
				}
			},
			func(name string) {
				if _, err := r2.GetRawZipFile(name); err != nil {
					errs <- fmt.Sprintf("GetRawZipFile(%s): %v", name, err)
				}
			},
			func(string) {
				_ = r2.GetRelationshipsByType(RelTypeOfficeDocument)
			},
		} {
			wg.Add(1)
			go func(name string, work func(string)) {
				defer wg.Done()
				work(name)
			}(fmt.Sprintf("/ppt/slides/slide%d.xml", i), worker)
		}
	}
	wg.Wait()
	close(errs)

	for msg := range errs {
		t.Error(msg)
	}
}
