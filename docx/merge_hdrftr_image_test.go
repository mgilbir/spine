package docx

import (
	"strings"
	"testing"
)

// Document.partImageBytes is how a merge resolves an image embedded in the
// *source* document's header or footer: the relationship target is relative to
// the header part, not to document.xml, and the bytes may live either in a part
// the source session added or in a preserved part it opened. When it fails to
// resolve, carryHdrFtrRels silently drops the relationship (`continue`) and the
// merged header keeps an r:embed pointing at nothing — Word reports the file as
// corrupt, and nothing in the package notices, because the merged document
// still saves and still reopens.
//
// Both source shapes are covered: a header image added in this session
// (resolved from d.imageParts) and one carried in from an opened package
// (resolved from d.preservedParts). They are separate branches of the function.
func TestMergeCarriesHeaderAndFooterImages(t *testing.T) {
	headerImage := taggedPNG("HEADER-IMAGE")
	footerImage := taggedPNG("FOOTER-IMAGE")

	for _, tc := range []struct {
		name string
		// source builds the document to be appended.
		source func(t *testing.T) *Document
	}{
		{
			name: "session-added source",
			source: func(t *testing.T) *Document {
				t.Helper()
				return buildDocWithHdrFtrImages(t, headerImage, footerImage)
			},
		},
		{
			name: "opened source",
			source: func(t *testing.T) *Document {
				t.Helper()
				// Round-tripping through disk moves the media into the
				// preserved-parts branch of partImageBytes.
				return saveAndReopen(t, buildDocWithHdrFtrImages(t, headerImage, footerImage))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := Create()
			target.AddParagraph().SetText("target body")

			if err := target.Append(tc.source(t)); err != nil {
				t.Fatalf("Append: %v", err)
			}

			parts, names := docParts(t, mustSaveBytes(t, target))

			// Both images must have been carried into the merged package.
			var haveHeader, haveFooter bool
			for _, n := range names {
				if !strings.HasPrefix(n, "word/media/") {
					continue
				}
				switch {
				case strings.Contains(parts[n], "HEADER-IMAGE"):
					haveHeader = true
				case strings.Contains(parts[n], "FOOTER-IMAGE"):
					haveFooter = true
				}
			}
			if !haveHeader {
				t.Errorf("the source header's image was not carried into the merge (%v)", names)
			}
			if !haveFooter {
				t.Errorf("the source footer's image was not carried into the merge (%v)", names)
			}

			// And every r:embed in a carried header/footer must resolve
			// through that part's own .rels: a dangling embed is the exact
			// failure a dropped relationship produces.
			for _, n := range names {
				if !strings.HasPrefix(n, "word/header") && !strings.HasPrefix(n, "word/footer") {
					continue
				}
				for _, embed := range embedIDsIn(parts[n]) {
					relsName := strings.Replace(n, "word/", "word/_rels/", 1) + ".rels"
					rels := parts[relsName]
					if rels == "" {
						t.Errorf("%s embeds %s but has no relationship part", n, embed)
						continue
					}
					if !strings.Contains(rels, `Id="`+embed+`"`) {
						t.Errorf("%s embeds %s, which %s does not declare — the merge dropped the image relationship:\n%s",
							n, embed, relsName, rels)
					}
				}
			}
		})
	}
}

// buildDocWithHdrFtrImages makes a document whose header and footer each embed
// a distinct image.
func buildDocWithHdrFtrImages(t *testing.T, headerImage, footerImage []byte) *Document {
	t.Helper()
	doc := Create()
	doc.AddParagraph().SetText("source body")

	hdr := doc.AddHeader(HeaderDefault)
	if _, err := hdr.AddParagraph().AddRun().AddImageFromBytes(headerImage, "image/png"); err != nil {
		t.Fatalf("adding the header image: %v", err)
	}
	ftr := doc.AddFooter(FooterDefault)
	if _, err := ftr.AddParagraph().AddRun().AddImageFromBytes(footerImage, "image/png"); err != nil {
		t.Fatalf("adding the footer image: %v", err)
	}
	// Append drops the source's *final* section by design, so the furniture has
	// to sit on a paragraph-level section break to be carried at all. Without
	// this the header and footer are never imported and the assertions below
	// would be testing the documented drop, not the image resolution.
	doc.AddSectionBreak()
	doc.AddParagraph().SetText("after the break")
	return doc
}

// embedIDsIn returns every r:embed relationship id in a part.
func embedIDsIn(part string) []string {
	var out []string
	rest := part
	const marker = `r:embed="`
	for {
		i := strings.Index(rest, marker)
		if i < 0 {
			return out
		}
		rest = rest[i+len(marker):]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			return out
		}
		out = append(out, rest[:j])
		rest = rest[j+1:]
	}
}

func TestEmbedIDsIn(t *testing.T) {
	got := embedIDsIn(`<a:blip r:embed="rId7"/><a:blip r:embed="rId9"/>`)
	if len(got) != 2 || got[0] != "rId7" || got[1] != "rId9" {
		t.Errorf("embedIDsIn = %v, want [rId7 rId9]", got)
	}
	if got := embedIDsIn("<w:p/>"); len(got) != 0 {
		t.Errorf("embedIDsIn on a part with no embeds = %v", got)
	}
}
