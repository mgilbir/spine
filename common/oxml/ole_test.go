package oxml

import "testing"

func TestExtractOLEProgID(t *testing.T) {
	tests := []struct {
		name  string
		xml   string
		relID string
		want  string
	}{
		{
			name:  "vml progid before id",
			xml:   `<o:OLEObject Type="Embed" ProgID="Excel.Sheet.12" ShapeID="s1" r:id="rId5"/>`,
			relID: "rId5",
			want:  "Excel.Sheet.12",
		},
		{
			name:  "pml progid on parent before embed id",
			xml:   `<p:oleObj name="Obj" progId="Word.Document.12" imgW="1" imgH="1"><p:embed r:id="rId9"/></p:oleObj>`,
			relID: "rId9",
			want:  "Word.Document.12",
		},
		{
			name:  "missing progid",
			xml:   `<o:OLEObject Type="Embed" r:id="rId3"/>`,
			relID: "rId3",
			want:  "",
		},
		{
			name:  "id not present",
			xml:   `<o:OLEObject ProgID="Excel.Sheet.12" r:id="rId1"/>`,
			relID: "rId7",
			want:  "",
		},
		{
			name:  "empty inputs",
			xml:   ``,
			relID: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractOLEProgID([]byte(tt.xml), tt.relID); got != tt.want {
				t.Errorf("ExtractOLEProgID = %q, want %q", got, tt.want)
			}
		})
	}
}
