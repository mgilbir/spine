package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// C306: CreateFromTemplate on a template flavor (.potx/.potm) must reset the main
// part to the plain (or macro-enabled) presentation flavor, so the output does
// not open as a template in PowerPoint.
func TestCreateFromTemplate_ResetsFlavor(t *testing.T) {
	cases := []struct {
		name     string
		template string
		want     string
	}{
		{"potx", opc.ContentTypePresentationTemplateMain, opc.ContentTypePresentationMain},
		{"potm", opc.ContentTypePresentationTemplateMacroMain, opc.ContentTypePresentationMacroMain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			templatePath := writeTempDeck(t, retypedDeck(t, tc.template))

			p, err := CreateFromTemplate(templatePath)
			if err != nil {
				t.Fatalf("CreateFromTemplate: %v", err)
			}
			defer func() { _ = p.Close() }()

			if got := p.Flavor(); got != tc.want {
				t.Errorf("Flavor() = %q, want %q", got, tc.want)
			}

			p.AddSlide()
			saved, err := p.SaveBytes()
			if err != nil {
				t.Fatal(err)
			}
			ct := zipPart(t, saved, "[Content_Types].xml")
			if !bytes.Contains(ct, []byte(tc.want)) {
				t.Errorf("saved main part is not the %q flavor:\n%s", tc.want, ct)
			}
			if bytes.Contains(ct, []byte(tc.template)) {
				t.Errorf("saved main part still carries the template flavor %q:\n%s", tc.template, ct)
			}
		})
	}
}
