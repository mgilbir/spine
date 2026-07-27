package docx

import "testing"

// C300: definitions authored this session must be visible to their own read
// APIs before the document is reopened (read-your-writes), mirroring how
// CustomXMLParts() already merges pending session parts.

func TestBuildingBlocks_SessionReadYourWrites(t *testing.T) {
	d := Create()
	if err := d.AddBuildingBlock(BuildingBlockDef{
		Name:        "MyBlock",
		Gallery:     "AutoText",
		Category:    "General",
		Types:       []string{"autoTxt"},
		Description: "desc",
	}); err != nil {
		t.Fatalf("AddBuildingBlock: %v", err)
	}

	var bb *BuildingBlock
	for _, cand := range d.BuildingBlocks() {
		if cand.Name() == "MyBlock" {
			bb = cand
			break
		}
	}
	if bb == nil {
		t.Fatal("BuildingBlocks() does not include the block added this session")
	}
	if bb.Gallery() != "AutoText" || bb.Category() != "General" {
		t.Errorf("block gallery/category = %q/%q, want AutoText/General", bb.Gallery(), bb.Category())
	}
	if len(bb.Types()) != 1 || bb.Types()[0] != "autoTxt" {
		t.Errorf("block types = %v, want [autoTxt]", bb.Types())
	}
}

func TestFrameset_SessionReadYourWrites(t *testing.T) {
	d := Create()
	if err := d.SetFrameset(FramesetDef{
		Layout: "cols",
		Size:   "*,240",
		Frames: []FrameDef{{Name: "left", SourceTarget: "left.html"}},
	}); err != nil {
		t.Fatalf("SetFrameset: %v", err)
	}

	fs := d.Frameset()
	if fs == nil {
		t.Fatal("Frameset() = nil after SetFrameset this session")
	}
	if fs.Layout() != "cols" || fs.Size() != "*,240" {
		t.Errorf("frameset layout/size = %q/%q, want cols/*,240", fs.Layout(), fs.Size())
	}
	if len(fs.Frames()) != 1 {
		t.Fatalf("Frameset().Frames() = %d, want 1", len(fs.Frames()))
	}
	if got := fs.Frames()[0]; got.Name() != "left" || got.SourceTarget() != "left.html" {
		t.Errorf("frame name/target = %q/%q, want left/left.html", got.Name(), got.SourceTarget())
	}
}
