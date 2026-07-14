package validate

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

func TestReport_HasErrorsAndFilters(t *testing.T) {
	c := New()
	c.Warnf("w1", "/a", "a warning")
	c.Errorf("e1", "/b", "an error")
	r := c.Report()

	if !r.HasErrors() {
		t.Fatal("HasErrors should be true when an error-severity finding is present")
	}
	if got := len(r.Errors()); got != 1 {
		t.Fatalf("Errors() = %d, want 1", got)
	}
	if got := len(r.Warnings()); got != 1 {
		t.Fatalf("Warnings() = %d, want 1", got)
	}

	warnOnly := New()
	warnOnly.Warnf("w2", "", "just advice")
	if warnOnly.Report().HasErrors() {
		t.Fatal("a warning-only report must not report HasErrors")
	}
}

func TestError_String(t *testing.T) {
	e := Error{Severity: SeverityError, Code: "dangling-rel", Part: "/ppt/x.xml", Detail: "boom"}
	s := e.Error()
	for _, want := range []string{"dangling-rel", "error", "/ppt/x.xml", "boom"} {
		if !strings.Contains(s, want) {
			t.Errorf("Error() = %q, missing %q", s, want)
		}
	}
}

func TestCheckDuplicateParts(t *testing.T) {
	c := New()
	CheckDuplicateParts(c, []string{"/A.xml", "/b.xml", "/a.xml"})
	if got := len(c.Report().Errors()); got != 1 {
		t.Fatalf("expected 1 duplicate-part error, got %d: %v", got, c.Report())
	}
}

func TestCheckRelationshipTargets_WarnsOnMissing(t *testing.T) {
	c := New()
	rels := map[string][]*opc.Relationship{
		"/word/document.xml": {
			{ID: "rId1", Type: opc.RelTypeImage, Target: "media/image1.png", TargetMode: opc.TargetModeInternal},
			{ID: "rId2", Type: opc.RelTypeHyperlink, Target: "http://example.com", TargetMode: opc.TargetModeExternal},
		},
	}
	exists := func(name string) bool { return false } // nothing exists
	CheckRelationshipTargets(c, rels, exists)
	r := c.Report()
	if r.HasErrors() {
		t.Fatalf("missing internal target must be a warning, not an error: %v", r)
	}
	if got := len(r.Warnings()); got != 1 {
		t.Fatalf("expected 1 warning (internal only; external skipped), got %d: %v", got, r)
	}
}
