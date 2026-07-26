package oxml

import (
	"encoding/xml"
	"testing"
)

func TestCommentList_RoundTrip(t *testing.T) {
	xmlStr := `<cmLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cm authorId="0" dt="2024-01-15T10:30:00Z" idx="1">
    <pos x="914400" y="457200"/>
    <text>This is a comment</text>
  </cm>
  <cm authorId="1" dt="2024-01-15T11:00:00Z" idx="2">
    <pos x="1828800" y="914400"/>
    <text>Another comment</text>
  </cm>
</cmLst>`

	var cl CommentList
	if err := xml.Unmarshal([]byte(xmlStr), &cl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(cl.Cm) != 2 {
		t.Errorf("Expected 2 comments, got %d", len(cl.Cm))
	}

	out, err := xml.Marshal(&cl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cl2 CommentList
	if err := xml.Unmarshal(out, &cl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}

	if len(cl2.Cm) != len(cl.Cm) {
		t.Errorf("Comment count mismatch: %d vs %d", len(cl2.Cm), len(cl.Cm))
	}
}

func TestComment_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic comment",
			xml: `<cm xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" authorId="0" idx="1">
  <text>Hello world</text>
</cm>`,
		},
		{
			name: "comment with position",
			xml: `<cm xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" authorId="1" dt="2024-01-15T10:30:00Z" idx="5">
  <pos x="914400" y="457200"/>
  <text>Positioned comment</text>
</cm>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cm Comment
			if err := xml.Unmarshal([]byte(tt.xml), &cm); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&cm)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cm2 Comment
			if err := xml.Unmarshal(out, &cm2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}

			if cm2.AuthorId != cm.AuthorId {
				t.Errorf("AuthorId = %d, want %d", cm2.AuthorId, cm.AuthorId)
			}
			if cm2.Idx != cm.Idx {
				t.Errorf("Idx = %d, want %d", cm2.Idx, cm.Idx)
			}
		})
	}
}

func TestPoint2D_RoundTrip(t *testing.T) {
	tests := []struct {
		x int64
		y int64
	}{
		{0, 0},
		{914400, 457200},
		{-100000, 200000},
	}

	for _, tt := range tests {
		t.Run("point", func(t *testing.T) {
			pt := &Point2D{X: tt.x, Y: tt.y}
			out, err := xml.Marshal(pt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var pt2 Point2D
			if err := xml.Unmarshal(out, &pt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if pt2.X != tt.x {
				t.Errorf("X = %d, want %d", pt2.X, tt.x)
			}
			if pt2.Y != tt.y {
				t.Errorf("Y = %d, want %d", pt2.Y, tt.y)
			}
		})
	}
}

func TestCommentAuthorList_RoundTrip(t *testing.T) {
	xmlStr := `<cmAuthorLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cmAuthor id="0" name="John Doe" initials="JD" lastIdx="3" clrIdx="0"/>
  <cmAuthor id="1" name="Jane Smith" initials="JS" lastIdx="5" clrIdx="1"/>
</cmAuthorLst>`

	var cal CommentAuthorList
	if err := xml.Unmarshal([]byte(xmlStr), &cal); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(cal.CmAuthor) != 2 {
		t.Errorf("Expected 2 authors, got %d", len(cal.CmAuthor))
	}

	out, err := xml.Marshal(&cal)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cal2 CommentAuthorList
	if err := xml.Unmarshal(out, &cal2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestCommentAuthor_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		id       uint32
		author   string
		initials string
		lastIdx  uint32
		clrIdx   uint32
	}{
		{"full author", 0, "John Doe", "JD", 5, 0},
		{"minimal author", 1, "Jane", "", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := &CommentAuthor{
				Id:       tt.id,
				Name:     tt.author,
				Initials: tt.initials,
				LastIdx:  tt.lastIdx,
				ClrIdx:   tt.clrIdx,
			}
			out, err := xml.Marshal(ca)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ca2 CommentAuthor
			if err := xml.Unmarshal(out, &ca2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ca2.Id != tt.id {
				t.Errorf("Id = %d, want %d", ca2.Id, tt.id)
			}
			if ca2.Name != tt.author {
				t.Errorf("Name = %q, want %q", ca2.Name, tt.author)
			}
		})
	}
}
func TestNotesSlide_RoundTrip(t *testing.T) {
	xmlStr := `<notes xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" showMasterSp="true" showMasterPhAnim="true">
  <cSld name="Notes"/>
</notes>`

	var ns NotesSlide
	if err := xml.Unmarshal([]byte(xmlStr), &ns); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&ns)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ns2 NotesSlide
	if err := xml.Unmarshal(out, &ns2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}

	if ns.ShowMasterSp == nil || ns2.ShowMasterSp == nil || *ns2.ShowMasterSp != *ns.ShowMasterSp {
		t.Errorf("ShowMasterSp = %v, want %v", ns2.ShowMasterSp, ns.ShowMasterSp)
	}
}

func TestNotesMaster_RoundTrip(t *testing.T) {
	xmlStr := `<notesMaster xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cSld name="Notes Master"/>
</notesMaster>`

	var nm NotesMaster
	if err := xml.Unmarshal([]byte(xmlStr), &nm); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&nm)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var nm2 NotesMaster
	if err := xml.Unmarshal(out, &nm2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestHandoutMaster_RoundTrip(t *testing.T) {
	xmlStr := `<handoutMaster xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cSld name="Handout Master"/>
</handoutMaster>`

	var hm HandoutMaster
	if err := xml.Unmarshal([]byte(xmlStr), &hm); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&hm)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var hm2 HandoutMaster
	if err := xml.Unmarshal(out, &hm2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestHeaderFooter_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		sldNum *bool
		hdr    *bool
		ftr    *bool
		dt     *bool
	}{
		{"all true", boolPtr(true), boolPtr(true), boolPtr(true), boolPtr(true)},
		{"all false", boolPtr(false), boolPtr(false), boolPtr(false), boolPtr(false)},
		{"mixed", boolPtr(true), boolPtr(false), boolPtr(true), boolPtr(false)},
		{"nil", nil, nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hf := &HeaderFooter{
				SldNum: tt.sldNum,
				Hdr:    tt.hdr,
				Ftr:    tt.ftr,
				Dt:     tt.dt,
			}
			out, err := xml.Marshal(hf)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var hf2 HeaderFooter
			if err := xml.Unmarshal(out, &hf2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if !boolPtrEq(hf2.SldNum, tt.sldNum) {
				t.Errorf("SldNum = %v, want %v", hf2.SldNum, tt.sldNum)
			}
			if !boolPtrEq(hf2.Hdr, tt.hdr) {
				t.Errorf("Hdr = %v, want %v", hf2.Hdr, tt.hdr)
			}
			if !boolPtrEq(hf2.Ftr, tt.ftr) {
				t.Errorf("Ftr = %v, want %v", hf2.Ftr, tt.ftr)
			}
			if !boolPtrEq(hf2.Dt, tt.dt) {
				t.Errorf("Dt = %v, want %v", hf2.Dt, tt.dt)
			}
		})
	}
}

func TestTagList_RoundTrip(t *testing.T) {
	xmlStr := `<tagLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <tag name="CustomTag1" val="Value1"/>
  <tag name="CustomTag2" val="Value2"/>
</tagLst>`

	var tl TagList
	if err := xml.Unmarshal([]byte(xmlStr), &tl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(tl.Tag) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(tl.Tag))
	}

	out, err := xml.Marshal(&tl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var tl2 TagList
	if err := xml.Unmarshal(out, &tl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestStringTag_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"tag1", "value1"},
		{"custom-tag", "custom-value"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &StringTag{Name: tt.name, Val: tt.val}
			out, err := xml.Marshal(st)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var st2 StringTag
			if err := xml.Unmarshal(out, &st2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if st2.Name != tt.name {
				t.Errorf("Name = %q, want %q", st2.Name, tt.name)
			}
			if st2.Val != tt.val {
				t.Errorf("Val = %q, want %q", st2.Val, tt.val)
			}
		})
	}
}

func TestRelationshipRef_RoundTrip(t *testing.T) {
	xmlStr := `<tags xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId1"/>`

	var rr RelationshipRef
	if err := xml.Unmarshal([]byte(xmlStr), &rr); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if rr.Id != "rId1" {
		t.Errorf("Id = %q, want %q", rr.Id, "rId1")
	}

	out, err := xml.Marshal(&rr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var rr2 RelationshipRef
	if err := xml.Unmarshal(out, &rr2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func boolPtr(b bool) *bool { return &b }

func boolPtrEq(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
