package diagram

import "testing"

// sampleDataModel is a small SmartArt data part: a doc root (id 0) with two
// top-level nodes "One" (id 1) and "Two" (id 2), where "One" has a child
// "One-A" (id 3). It also carries presentation/transition points and a presOf
// connection to confirm the tree walk ignores them, and it omits type="parOf"
// on the hierarchy connections to exercise the schema default.
const sampleDataModel = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <dgm:ptLst>
    <dgm:pt modelId="0" type="doc"><dgm:prSet loTypeId="urn:diagrams/list"/><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:endParaRPr lang="en-US"/></a:p></dgm:t></dgm:pt>
    <dgm:pt modelId="1"><dgm:prSet phldrT="[Text]"/><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>One</a:t></a:r></a:p></dgm:t></dgm:pt>
    <dgm:pt modelId="2"><dgm:prSet phldrT="[Text]"/><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>Two</a:t></a:r></a:p></dgm:t></dgm:pt>
    <dgm:pt modelId="3"><dgm:prSet phldrT="[Text]"/><dgm:t><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="en-US"/><a:t>One-A</a:t></a:r></a:p></dgm:t></dgm:pt>
    <dgm:pt modelId="p1" type="pres"/>
    <dgm:pt modelId="t1" type="parTrans"/>
    <dgm:pt modelId="t2" type="sibTrans"/>
  </dgm:ptLst>
  <dgm:cxnLst>
    <dgm:cxn modelId="10" srcId="0" destId="2" srcOrd="1" destOrd="0"/>
    <dgm:cxn modelId="11" srcId="0" destId="1" srcOrd="0" destOrd="0"/>
    <dgm:cxn modelId="12" srcId="1" destId="3" srcOrd="0" destOrd="0"/>
    <dgm:cxn modelId="13" type="presOf" srcId="1" destId="p1" srcOrd="0" destOrd="0"/>
  </dgm:cxnLst>
  <dgm:whole/>
</dgm:dataModel>`

func TestParseDataModelAndTextTree(t *testing.T) {
	dm, err := ParseDataModel([]byte(sampleDataModel))
	if err != nil {
		t.Fatalf("ParseDataModel: %v", err)
	}
	tree := dm.TextTree()
	if len(tree) != 2 {
		t.Fatalf("top-level nodes = %d, want 2", len(tree))
	}
	// srcOrd orders siblings: One (ord 0) before Two (ord 1), despite the
	// connections appearing in the reverse order in the source.
	if tree[0].Text != "One" {
		t.Errorf("tree[0].Text = %q, want %q", tree[0].Text, "One")
	}
	if tree[1].Text != "Two" {
		t.Errorf("tree[1].Text = %q, want %q", tree[1].Text, "Two")
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("One children = %d, want 1", len(tree[0].Children))
	}
	if tree[0].Children[0].Text != "One-A" {
		t.Errorf("One child text = %q, want %q", tree[0].Children[0].Text, "One-A")
	}
	if len(tree[1].Children) != 0 {
		t.Errorf("Two children = %d, want 0", len(tree[1].Children))
	}
}

func TestTextTreeNil(t *testing.T) {
	var dm *DataModel
	if got := dm.TextTree(); got != nil {
		t.Errorf("nil DataModel TextTree = %v, want nil", got)
	}
	if got := (&DataModel{}).TextTree(); got != nil {
		t.Errorf("empty DataModel TextTree = %v, want nil", got)
	}
}

// TestTextTreeNoDoc covers a data model with no doc point: content points
// without a parent become the roots.
func TestTextTreeNoDoc(t *testing.T) {
	const noDoc = `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <dgm:ptLst>
    <dgm:pt modelId="1"><dgm:t><a:p><a:r><a:t>A</a:t></a:r></a:p></dgm:t></dgm:pt>
    <dgm:pt modelId="2"><dgm:t><a:p><a:r><a:t>B</a:t></a:r></a:p></dgm:t></dgm:pt>
  </dgm:ptLst>
  <dgm:cxnLst>
    <dgm:cxn modelId="10" srcId="1" destId="2" srcOrd="0" destOrd="0"/>
  </dgm:cxnLst>
</dgm:dataModel>`
	dm, err := ParseDataModel([]byte(noDoc))
	if err != nil {
		t.Fatalf("ParseDataModel: %v", err)
	}
	tree := dm.TextTree()
	if len(tree) != 1 || tree[0].Text != "A" {
		t.Fatalf("roots = %+v, want single root A", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Text != "B" {
		t.Fatalf("A children = %+v, want [B]", tree[0].Children)
	}
}
