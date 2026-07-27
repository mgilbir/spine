package docx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// C352: a settings-edit regeneration must preserve every root attribute. The
// old marshalSettingsXML only re-emitted the namespace declarations and
// mc:Ignorable, dropping other root attributes (e.g. xml:space). Adopting the
// OriginalRootAttrs verbatim-replay pattern (as footnotes.xml does) keeps them.
func TestMarshalSettingsXML_PreservesExtraRootAttr(t *testing.T) {
	src := `<w:settings ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xml:space="preserve" mc:Ignorable="w14">` +
		`<w:zoom w:percent="100"/>` +
		`</w:settings>`
	var s oxml.CT_Settings
	if err := xmlb.Unmarshal([]byte(src), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Force a settings edit so regeneration (marshalSettingsXML) runs.
	if !s.EnsureEvenAndOddHeaders() {
		t.Fatalf("expected EnsureEvenAndOddHeaders to modify settings")
	}
	data, err := marshalSettingsXML(&s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `xml:space="preserve"`) {
		t.Errorf("xml:space root attribute dropped on regeneration: %s", out)
	}
	if !strings.Contains(out, `mc:Ignorable="w14"`) {
		t.Errorf("mc:Ignorable root attribute dropped on regeneration: %s", out)
	}
	if !strings.Contains(out, "evenAndOddHeaders") {
		t.Errorf("edit not applied: %s", out)
	}
}
