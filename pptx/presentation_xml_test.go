package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// C4: presentation.xml attributes and child elements that the model parses are
// no longer dropped when it is regenerated on save.
func TestMarshalPresentationXML_PreservesAttrsAndChildren(t *testing.T) {
	firstSlide := 5
	rtl := true
	seed := uint32(2048)
	spin := uint32(100000)

	pres := &oxml.Presentation{
		FirstSlideNum:  &firstSlide,
		Rtl:            &rtl,
		BookmarkIdSeed: &seed,
		Conformance:    "strict",
		CustShowLst: &oxml.CustomShowList{
			CustShow: []oxml.CustomShow{{Name: "Show1", ID: 1}},
		},
		ModifyVerifier: &oxml.ModifyVerifier{
			CryptProviderType: "rsaAES",
			SpinCount:         &spin,
		},
	}

	out := string(marshalPresentationXML(pres, false))

	for _, want := range []string{
		`firstSlideNum="5"`,
		`rtl="1"`,
		`bookmarkIdSeed="2048"`,
		`conformance="strict"`,
		"custShowLst",
		`name="Show1"`,
		"modifyVerifier",
		`cryptProviderType="rsaAES"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("presentation.xml is missing %q:\n%s", want, out)
		}
	}
}
