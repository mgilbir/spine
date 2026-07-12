package oxml

import "testing"

// C147: only the "_rels" directory component and the ".rels" suffix are
// removed; folder names that merely end in "_rels" must survive.
func TestRelsPathToSourcePart(t *testing.T) {
	cases := map[string]string{
		"/ppt/_rels/presentation.xml.rels":  "/ppt/presentation.xml",
		"/word/_rels/document.xml.rels":     "/word/document.xml",
		"/_rels/.rels":                      "/",
		"/custom_rels/_rels/part.xml.rels":  "/custom_rels/part.xml",
		"/a/_rels_b/_rels/part.xml.rels":    "/a/_rels_b/part.xml",
		"/ppt/slides/_rels/slide1.xml.rels": "/ppt/slides/slide1.xml",
		"/not-a-rels/part.xml":              "/not-a-rels/part.xml",
	}
	for in, want := range cases {
		if got := RelsPathToSourcePart(in); got != want {
			t.Errorf("RelsPathToSourcePart(%q) = %q, want %q", in, got, want)
		}
	}
}
