package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// roundTripWML parses src into v (with the capture kit armed) and re-marshals
// it inside a w:document root carrying the standard declarations, mirroring
// what the part marshalers do so namespace resolution behaves as it does on
// the real save path.
func roundTripWML(t *testing.T, src string, v xmlb.BuilderMarshaler, ns, local string) string {
	t.Helper()
	if err := xmlb.UnmarshalWithSource([]byte(src), v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewWordprocessingMLBuilder()
	b.StartElementWithNS(xmlb.NSWordprocessingML, "document", xmlb.WordprocessingMLNamespaces())
	v.MarshalToBuilder(b, ns, local)
	b.EndElement(xmlb.NSWordprocessingML, "document")
	if err := b.Finish(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b.Bytes())
}

const wnsDecl = ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// C371: commentRangeStart/End, proofErr, ins/del and permStart/permEnd are all
// schema-valid at body, table, row and cell level (EG_RunLevelElts is a member
// of EG_ContentRowContent, EG_ContentCellContent and EG_BlockLevelElts), and
// Word emits row/cell-level comment ranges when a comment anchors a whole row
// or cell. The four hand-maintained raw-child whitelists carried different
// subsets of these, so the comment survived while its anchor range was deleted.
func TestRawChildCaptureAtEveryBlockLevel(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		local string
		new   func() xmlb.BuilderMarshaler
		want  []string
	}{
		{
			name:  "body-level comment range",
			local: "body",
			new:   func() xmlb.BuilderMarshaler { return &CT_Body{} },
			src: `<w:body` + wnsDecl + `>` +
				`<w:commentRangeStart w:id="1"/><w:p/><w:commentRangeEnd w:id="1"/>` +
				`</w:body>`,
			want: []string{`<w:commentRangeStart w:id="1"/>`, `<w:commentRangeEnd w:id="1"/>`},
		},
		{
			name:  "body-level proofErr and ins",
			local: "body",
			new:   func() xmlb.BuilderMarshaler { return &CT_Body{} },
			src: `<w:body` + wnsDecl + `>` +
				`<w:proofErr w:type="spellStart"/><w:ins w:id="4" w:author="a"><w:p/></w:ins>` +
				`</w:body>`,
			want: []string{`<w:proofErr w:type="spellStart"/>`, `<w:ins w:id="4"`},
		},
		{
			name:  "row-level comment range",
			local: "tr",
			new:   func() xmlb.BuilderMarshaler { return &CT_Tr{} },
			src: `<w:tr` + wnsDecl + `>` +
				`<w:commentRangeStart w:id="2"/><w:tc><w:p/></w:tc><w:commentRangeEnd w:id="2"/>` +
				`</w:tr>`,
			want: []string{`<w:commentRangeStart w:id="2"/>`, `<w:commentRangeEnd w:id="2"/>`},
		},
		{
			name:  "row-level perms and proofErr",
			local: "tr",
			new:   func() xmlb.BuilderMarshaler { return &CT_Tr{} },
			src: `<w:tr` + wnsDecl + `>` +
				`<w:permStart w:id="7" w:edGrp="everyone"/><w:tc><w:p/></w:tc><w:permEnd w:id="7"/><w:proofErr w:type="gramEnd"/>` +
				`</w:tr>`,
			want: []string{`<w:permStart w:id="7"`, `<w:permEnd w:id="7"/>`, `<w:proofErr w:type="gramEnd"/>`},
		},
		{
			name:  "cell-level comment range",
			local: "tc",
			new:   func() xmlb.BuilderMarshaler { return &CT_Tc{} },
			src: `<w:tc` + wnsDecl + `>` +
				`<w:commentRangeStart w:id="3"/><w:p/><w:commentRangeEnd w:id="3"/>` +
				`</w:tc>`,
			want: []string{`<w:commentRangeStart w:id="3"/>`, `<w:commentRangeEnd w:id="3"/>`},
		},
		{
			name:  "table-level comment range",
			local: "tbl",
			new:   func() xmlb.BuilderMarshaler { return &CT_Tbl{} },
			src: `<w:tbl` + wnsDecl + `>` +
				`<w:commentRangeStart w:id="5"/><w:tr><w:tc><w:p/></w:tc></w:tr><w:commentRangeEnd w:id="5"/>` +
				`</w:tbl>`,
			want: []string{`<w:commentRangeStart w:id="5"/>`, `<w:commentRangeEnd w:id="5"/>`},
		},
		{
			name:  "table-level proofErr and ins",
			local: "tbl",
			new:   func() xmlb.BuilderMarshaler { return &CT_Tbl{} },
			src: `<w:tbl` + wnsDecl + `>` +
				`<w:proofErr w:type="spellEnd"/><w:ins w:id="9" w:author="a"><w:tr><w:tc><w:p/></w:tc></w:tr></w:ins>` +
				`</w:tbl>`,
			want: []string{`<w:proofErr w:type="spellEnd"/>`, `<w:ins w:id="9"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := roundTripWML(t, tc.src, tc.new(), xmlb.NSWordprocessingML, tc.local)
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Errorf("child %q dropped on round trip:\n%s", want, out)
				}
			}
		})
	}
}

// TestRawChildCaptureForeignNamespace pins the general form of the fix: any
// child the container does not type is captured, including one in a namespace
// the builder has no registered prefix for. Such an element must not fail the
// save with "no prefix registered".
func TestRawChildCaptureForeignNamespace(t *testing.T) {
	src := `<w:body` + wnsDecl + ` xmlns:x="urn:example:x"><w:p/><x:thing x:a="1"/></w:body>`
	out := roundTripWML(t, src, &CT_Body{}, xmlb.NSWordprocessingML, "body")
	if !strings.Contains(out, "thing") {
		t.Errorf("foreign-namespace child dropped:\n%s", out)
	}
	if !strings.Contains(out, "urn:example:x") {
		t.Errorf("foreign-namespace child emitted without a resolvable declaration:\n%s", out)
	}
}

// TestRawChildCaptureDefaultNamespace covers a foreign child written under a
// default namespace inherited from an ancestor: there is no source prefix to
// replay, so the declaration must be synthesized inline rather than resolved
// against the builder's (empty) registration for that URI.
func TestRawChildCaptureDefaultNamespace(t *testing.T) {
	src := `<w:body` + wnsDecl + ` xmlns="urn:example:d"><w:p/><thing a="1"/></w:body>`
	out := roundTripWML(t, src, &CT_Body{}, xmlb.NSWordprocessingML, "body")
	if !strings.Contains(out, "thing") {
		t.Errorf("default-namespaced foreign child dropped:\n%s", out)
	}
	if !strings.Contains(out, `xmlns="urn:example:d"`) {
		t.Errorf("default-namespaced foreign child lost its declaration:\n%s", out)
	}
}

// C373: CT_SimpleField passed nil slots for permStart/permEnd/ins/del/sdt, and
// the typed case arms match those names before the raw fallback, so the nil
// branch called d.Skip(): a tracked insertion inside a simple field lost all
// its text and a tracked deletion was silently accepted.
func TestSimpleFieldKeepsTrackedContent(t *testing.T) {
	src := `<w:fldSimple` + wnsDecl + ` w:instr=" PAGE ">` +
		`<w:ins w:id="11" w:author="a"><w:r><w:t>inserted</w:t></w:r></w:ins>` +
		`<w:del w:id="12" w:author="a"><w:r><w:delText>gone</w:delText></w:r></w:del>` +
		`<w:permStart w:id="13" w:edGrp="everyone"/><w:permEnd w:id="13"/>` +
		`<w:sdt><w:sdtContent><w:r><w:t>tagged</w:t></w:r></w:sdtContent></w:sdt>` +
		`</w:fldSimple>`
	out := roundTripWML(t, src, &CT_SimpleField{}, xmlb.NSWordprocessingML, "fldSimple")
	for _, want := range []string{"inserted", `w:id="11"`, "gone", `w:id="12"`,
		`<w:permStart w:id="13"`, `<w:permEnd w:id="13"/>`, "tagged"} {
		if !strings.Contains(out, want) {
			t.Errorf("simple-field content %q lost on round trip:\n%s", want, out)
		}
	}
}
