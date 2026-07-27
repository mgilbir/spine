package diagram

import (
	"fmt"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Kind identifies a diagram (SmartArt) layout family that this package can
// generate a complete, schema-valid set of definition parts for. The value
// selects the layout algorithm embedded in the generated dgm:layoutDef; the
// data model and the quick-style/colors parts are shared across kinds.
type Kind int

const (
	// KindList is a simple top-to-bottom list: every top-level node becomes a
	// rounded rectangle stacked vertically (dgm:alg type="lin").
	KindList Kind = iota
	// KindHierarchy is a top-down organization chart / hierarchy: the tree of
	// nodes laid out with the hierarchy algorithms (dgm:alg type="hierRoot"
	// and "hierChild"), children arranged in a row beneath their parent.
	KindHierarchy
	// KindProcess is a left-to-right process: every top-level node becomes a
	// rounded rectangle laid out horizontally (dgm:alg type="lin" linDir="fromL").
	// It shares the list's structure, differing only in direction.
	KindProcess
	// KindCycle is a radial cycle: every top-level node becomes an ellipse
	// arranged around a circle (dgm:alg type="cycle").
	KindCycle
)

// BuildNode is one node of a diagram's text outline: its text and its child
// nodes, in order. It is the input to Build; a flat list passes nodes with no
// children, a hierarchy nests them. It mirrors the forest that DataModel.TextTree
// returns when reading a diagram back.
type BuildNode struct {
	Text     string
	Children []BuildNode
}

// Parts holds the four serialized XML parts that together make up a SmartArt
// diagram: the data model (dgm:dataModel), the layout definition
// (dgm:layoutDef), the quick-style definition (dgm:styleDef), and the color
// transform (dgm:colorsDef). Each is a complete part payload including the XML
// declaration, ready to be stored under ppt/diagrams/ (or word/, xl/) and wired
// to a graphicFrame's dgm:relIds.
type Parts struct {
	Data       []byte
	Layout     []byte
	QuickStyle []byte
	Colors     []byte
}

// Build generates the four diagram parts for the given kind and text outline.
// The data part carries a doc root plus one content point per node with its
// text, and the parent-of connections that encode the hierarchy, exactly the
// shape DataModel.TextTree reads back. The layout/quickStyle/colors parts are
// minimal-but-valid definitions that Office renders: the layout provides the
// chosen algorithm, the quick-style a subtle-fill node style, and the colors a
// theme-accent color cycle.
//
// It returns an error only when serializing the data model fails, which would
// otherwise ship a truncated part.
func Build(kind Kind, nodes []BuildNode) (Parts, error) {
	data, err := buildDataModel(nodes)
	if err != nil {
		return Parts{}, err
	}
	return Parts{
		Data:       data,
		Layout:     layoutDefXML(kind),
		QuickStyle: []byte(quickStyleXML),
		Colors:     []byte(colorsXML),
	}, nil
}

// buildDataModel serializes a dgm:dataModel from the node outline. It assigns a
// modelId to the doc root ("0") and to each node in depth-first order, and emits
// a parOf connection from each node's parent (the doc root for a top-level node)
// carrying the child's ordinal among its siblings, so the reader reconstructs
// the same forest.
func buildDataModel(nodes []BuildNode) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDiagram, xmlb.PrefixDrawingMLDiagram)
	b.RegisterNamespace(xmlb.NSDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	b.WriteHeader()
	b.StartElementWithNS(NsDiagram, "dataModel", []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingMLDiagram, URI: NsDiagram},
		{Prefix: xmlb.PrefixDrawingML, URI: xmlb.NSDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	})

	// A running counter yields modelIds: the doc root is 0, nodes follow in
	// depth-first order. Connection ids are allocated after every point id so
	// the two ranges never collide.
	next := 1
	type ptEntry struct {
		id   string
		text string
		isPt bool
	}
	var pts []ptEntry
	type cxnEntry struct {
		src, dest string
		ord       int
	}
	var cxns []cxnEntry

	var walk func(parentID string, siblings []BuildNode)
	walk = func(parentID string, siblings []BuildNode) {
		for i, n := range siblings {
			id := strconv.Itoa(next)
			next++
			pts = append(pts, ptEntry{id: id, text: n.Text, isPt: true})
			cxns = append(cxns, cxnEntry{src: parentID, dest: id, ord: i})
			walk(id, n.Children)
		}
	}
	walk("0", nodes)

	// Points: the doc root first, then every content node.
	b.StartElement(NsDiagram, "ptLst")
	b.StartElement(NsDiagram, "pt", xmlb.StrAttr("modelId", "0"), xmlb.StrAttr("type", "doc"))
	writePtText(b, "")
	b.EndElement(NsDiagram, "pt")
	for _, p := range pts {
		b.StartElement(NsDiagram, "pt", xmlb.StrAttr("modelId", p.id))
		writePtText(b, p.text)
		b.EndElement(NsDiagram, "pt")
	}
	b.EndElement(NsDiagram, "ptLst")

	// Connections: one parent-of link per node.
	b.StartElement(NsDiagram, "cxnLst")
	for _, c := range cxns {
		id := strconv.Itoa(next)
		next++
		b.EmptyElement(NsDiagram, "cxn",
			xmlb.StrAttr("modelId", id),
			xmlb.StrAttr("srcId", c.src),
			xmlb.StrAttr("destId", c.dest),
			xmlb.StrAttr("srcOrd", strconv.Itoa(c.ord)),
			xmlb.StrAttr("destOrd", "0"),
		)
	}
	b.EndElement(NsDiagram, "cxnLst")

	b.EmptyElement(NsDiagram, "whole")
	b.EndElement(NsDiagram, "dataModel")
	// Finish reports unbalanced elements and any deferred write error. Without
	// this check a Builder failure silently shipped a truncated dgm:dataModel
	// part (ThemeEditor.Marshal has always checked; this did not).
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("diagram: build data model: %w", err)
	}
	return b.Bytes(), nil
}

// writePtText emits a data point's text body (dgm:t). A non-empty text becomes a
// single run; an empty text (the doc root, or a blank node) becomes an empty
// paragraph with just an endParaRPr, mirroring what PowerPoint writes.
func writePtText(b *xmlb.Builder, text string) {
	b.StartElement(NsDiagram, "t")
	b.EmptyElement(xmlb.NSDrawingML, "bodyPr")
	b.EmptyElement(xmlb.NSDrawingML, "lstStyle")
	b.StartElement(xmlb.NSDrawingML, "p")
	if text == "" {
		b.EmptyElement(xmlb.NSDrawingML, "endParaRPr", xmlb.StrAttr("lang", "en-US"))
	} else {
		b.StartElement(xmlb.NSDrawingML, "r")
		b.EmptyElement(xmlb.NSDrawingML, "rPr", xmlb.StrAttr("lang", "en-US"))
		b.WriteElement(xmlb.NSDrawingML, "t", text)
		b.EndElement(xmlb.NSDrawingML, "r")
	}
	b.EndElement(xmlb.NSDrawingML, "p")
	b.EndElement(NsDiagram, "t")
}

// layoutDefXML returns the dgm:layoutDef part for a kind.
func layoutDefXML(kind Kind) []byte {
	switch kind {
	case KindHierarchy:
		return []byte(layoutHierarchyXML)
	case KindProcess:
		return []byte(layoutProcessXML)
	case KindCycle:
		return []byte(layoutCycleXML)
	default:
		return []byte(layoutListXML)
	}
}

const xmlHeader = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n"

// layoutListXML is a vertical-list layout (dgm:alg type="lin"): each content
// node is drawn as a rounded rectangle, stacked top to bottom with even spacing.
// It is intentionally compact — PowerPoint supplies sensible defaults for any
// constraint the definition omits — while remaining schema-valid.
const layoutListXML = xmlHeader +
	`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:spine/diagram/layout/verticalList">` +
	`<dgm:title val="Vertical List"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="list" pri="1000"/></dgm:catLst>` +
	`<dgm:layoutNode name="diagram">` +
	`<dgm:varLst><dgm:chMax val="0"/><dgm:chPref val="0"/><dgm:dir val="norm"/><dgm:animLvl val="lvl"/><dgm:resizeHandles val="exact"/></dgm:varLst>` +
	`<dgm:alg type="lin"><dgm:param type="linDir" val="fromT"/></dgm:alg>` +
	`<dgm:shape><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="w" for="ch" ptType="node" op="equ"/>` +
	`<dgm:constr type="h" for="ch" ptType="node" op="equ"/>` +
	`<dgm:constr type="sibSp" refType="h" refPtType="node" fact="0.1"/>` +
	`<dgm:constr type="primFontSz" for="ch" ptType="node" op="equ" val="65"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`<dgm:forEach name="nodesForEach" axis="ch" ptType="node">` +
	`<dgm:layoutNode name="node" styleLbl="node1">` +
	`<dgm:alg type="tx"/>` +
	`<dgm:shape type="roundRect" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="desOrSelf" ptType="node"/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="lMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="rMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="tMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="bMarg" refType="primFontSz" fact="0.3"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`<dgm:forEach name="sibTransForEach" axis="followSib" ptType="sibTrans" cnt="1">` +
	`<dgm:layoutNode name="sibTrans"><dgm:alg type="sp"/><dgm:shape/><dgm:presOf/><dgm:constrLst/><dgm:ruleLst/></dgm:layoutNode>` +
	`</dgm:forEach>` +
	`</dgm:forEach>` +
	`</dgm:layoutNode>` +
	`</dgm:layoutDef>`

// layoutHierarchyXML is a top-down hierarchy / organization-chart layout: the
// root node sits above a row of its children (dgm:alg type="hierRoot"), and the
// hierChild algorithm recurses over descendants so multi-level trees fan out.
// Connector shapes join parents to children. Like the list layout it leans on
// PowerPoint's algorithm defaults and stays schema-valid.
const layoutHierarchyXML = xmlHeader +
	`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:spine/diagram/layout/hierarchy">` +
	`<dgm:title val="Hierarchy"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="hierarchy" pri="1000"/><dgm:cat type="list" pri="3000"/></dgm:catLst>` +
	`<dgm:layoutNode name="hierRoot">` +
	`<dgm:varLst><dgm:orgChart val="1"/><dgm:chMax val="0"/><dgm:chPref val="0"/><dgm:dir val="norm"/><dgm:animLvl val="lvl"/><dgm:resizeHandles val="exact"/></dgm:varLst>` +
	`<dgm:alg type="hierRoot"><dgm:param type="hierAlign" val="tCtrCh"/></dgm:alg>` +
	`<dgm:shape/>` +
	`<dgm:presOf/>` +
	`<dgm:constrLst><dgm:constr type="sibSp" val="20"/></dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`<dgm:forEach name="rootNodeForEach" axis="root" ptType="node" st="1" cnt="1">` +
	`<dgm:layoutNode name="rootNode" styleLbl="node1">` +
	`<dgm:alg type="tx"/>` +
	`<dgm:shape type="roundRect" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="self"/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="lMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="rMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="tMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="bMarg" refType="primFontSz" fact="0.3"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`<dgm:forEach name="hierChildForEach" axis="ch" ptType="node">` +
	`<dgm:layoutNode name="hierChild">` +
	`<dgm:alg type="hierChild"><dgm:param type="linDir" val="fromL"/></dgm:alg>` +
	`<dgm:shape/>` +
	`<dgm:presOf/>` +
	`<dgm:constrLst/>` +
	`<dgm:ruleLst/>` +
	`<dgm:forEach name="Name0" axis="self">` +
	`<dgm:layoutNode name="childNode" styleLbl="node1">` +
	`<dgm:alg type="tx"/>` +
	`<dgm:shape type="roundRect" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="self"/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="lMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="rMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="tMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="bMarg" refType="primFontSz" fact="0.3"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`<dgm:forEach name="connForEach" axis="par" ptType="parTrans" cnt="1">` +
	`<dgm:layoutNode name="connNode">` +
	`<dgm:alg type="conn"><dgm:param type="dim" val="1D"/><dgm:param type="endSty" val="noArr"/></dgm:alg>` +
	`<dgm:shape type="conn" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="self"/>` +
	`<dgm:constrLst/>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`</dgm:forEach>` +
	`<dgm:forEach name="recurse" ref="hierChildForEach"/>` +
	`</dgm:forEach>` + // Name0
	`</dgm:layoutNode>` + // hierChild
	`</dgm:forEach>` + // hierChildForEach
	`</dgm:forEach>` + // rootNodeForEach
	`</dgm:layoutNode>` + // hierRoot
	`</dgm:layoutDef>`

// layoutProcessXML is a left-to-right process layout: the same structure as the
// vertical list (dgm:alg type="lin"), but flowing horizontally (linDir="fromL")
// so each content node reads as a step in a process. It leans on PowerPoint's
// algorithm defaults and stays schema-valid.
const layoutProcessXML = xmlHeader +
	`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:spine/diagram/layout/process">` +
	`<dgm:title val="Basic Process"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="process" pri="1000"/><dgm:cat type="list" pri="2000"/></dgm:catLst>` +
	`<dgm:layoutNode name="diagram">` +
	`<dgm:varLst><dgm:chMax val="0"/><dgm:chPref val="0"/><dgm:dir val="norm"/><dgm:animLvl val="lvl"/><dgm:resizeHandles val="exact"/></dgm:varLst>` +
	`<dgm:alg type="lin"><dgm:param type="linDir" val="fromL"/></dgm:alg>` +
	`<dgm:shape><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="w" for="ch" ptType="node" op="equ"/>` +
	`<dgm:constr type="h" for="ch" ptType="node" op="equ"/>` +
	`<dgm:constr type="sibSp" refType="w" refPtType="node" fact="0.1"/>` +
	`<dgm:constr type="primFontSz" for="ch" ptType="node" op="equ" val="65"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`<dgm:forEach name="nodesForEach" axis="ch" ptType="node">` +
	`<dgm:layoutNode name="node" styleLbl="node1">` +
	`<dgm:alg type="tx"/>` +
	`<dgm:shape type="roundRect" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="desOrSelf" ptType="node"/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="lMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="rMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="tMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="bMarg" refType="primFontSz" fact="0.3"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`<dgm:forEach name="sibTransForEach" axis="followSib" ptType="sibTrans" cnt="1">` +
	`<dgm:layoutNode name="sibTrans"><dgm:alg type="sp"/><dgm:shape/><dgm:presOf/><dgm:constrLst/><dgm:ruleLst/></dgm:layoutNode>` +
	`</dgm:forEach>` +
	`</dgm:forEach>` +
	`</dgm:layoutNode>` +
	`</dgm:layoutDef>`

// layoutCycleXML is a radial cycle layout (dgm:alg type="cycle"): each content
// node becomes an ellipse arranged evenly around a full circle. Connector
// shapes between steps are omitted deliberately — they are optional for schema
// validity and PowerPoint renders the positioned nodes without them — keeping
// the definition compact and robust.
const layoutCycleXML = xmlHeader +
	`<dgm:layoutDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" uniqueId="urn:spine/diagram/layout/cycle">` +
	`<dgm:title val="Basic Cycle"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="cycle" pri="1000"/><dgm:cat type="relationship" pri="4000"/></dgm:catLst>` +
	`<dgm:layoutNode name="diagram">` +
	`<dgm:varLst><dgm:chMax val="0"/><dgm:chPref val="0"/><dgm:dir val="norm"/><dgm:animLvl val="lvl"/><dgm:resizeHandles val="exact"/></dgm:varLst>` +
	`<dgm:alg type="cycle"><dgm:param type="stAng" val="0"/><dgm:param type="spanAng" val="360"/><dgm:param type="ctrShpMap" val="none"/></dgm:alg>` +
	`<dgm:shape><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="sibSp" refType="w" refPtType="node" fact="0.1"/>` +
	`<dgm:constr type="primFontSz" for="ch" ptType="node" op="equ" val="65"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`<dgm:forEach name="nodesForEach" axis="ch" ptType="node">` +
	`<dgm:layoutNode name="node" styleLbl="node1">` +
	`<dgm:alg type="tx"/>` +
	`<dgm:shape type="ellipse" blipPhldr="0"><dgm:adjLst/></dgm:shape>` +
	`<dgm:presOf axis="desOrSelf" ptType="node"/>` +
	`<dgm:constrLst>` +
	`<dgm:constr type="lMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="rMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="tMarg" refType="primFontSz" fact="0.3"/>` +
	`<dgm:constr type="bMarg" refType="primFontSz" fact="0.3"/>` +
	`</dgm:constrLst>` +
	`<dgm:ruleLst/>` +
	`</dgm:layoutNode>` +
	`</dgm:forEach>` +
	`</dgm:layoutNode>` +
	`</dgm:layoutDef>`

// quickStyleXML is a minimal quick-style definition (dgm:styleDef): a single
// "node1" label giving every node a subtle theme fill, matching line, and a
// light font. Labels the layout does not reference simply fall back to
// PowerPoint's defaults, so one entry is enough to render styled shapes.
var quickStyleXML = xmlHeader +
	`<dgm:styleDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" uniqueId="urn:spine/diagram/quickStyle/simple">` +
	`<dgm:title val="Simple"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="simple" pri="10100"/></dgm:catLst>` +
	`<dgm:scene3d><a:camera prst="orthographicFront"/><a:lightRig rig="threePt" dir="t"/></dgm:scene3d>` +
	styleLabelXML("node0") +
	styleLabelXML("node1") +
	styleLabelXML("lnNode1") +
	`</dgm:styleDef>`

// styleLabelXML renders one dgm:styleLbl with a solid-fill/matching-line/light-
// font style reference into theme accent 1.
func styleLabelXML(name string) string {
	return `<dgm:styleLbl name="` + name + `">` +
		`<dgm:scene3d><a:camera prst="orthographicFront"/><a:lightRig rig="threePt" dir="t"/></dgm:scene3d>` +
		`<dgm:sp3d/>` +
		`<dgm:txPr/>` +
		`<dgm:style>` +
		`<a:lnRef idx="2"><a:schemeClr val="accent1"><a:shade val="50000"/></a:schemeClr></a:lnRef>` +
		`<a:fillRef idx="1"><a:schemeClr val="accent1"/></a:fillRef>` +
		`<a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef>` +
		`<a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef>` +
		`</dgm:style>` +
		`</dgm:styleLbl>`
}

// colorsXML is a minimal color transform (dgm:colorsDef): a single "node1"
// label that cycles node fills through the theme accent colors and keeps text
// light. As with the quick style, unreferenced labels fall back to defaults.
var colorsXML = xmlHeader +
	`<dgm:colorsDef xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" uniqueId="urn:spine/diagram/colors/accent1">` +
	`<dgm:title val="Colorful"/>` +
	`<dgm:desc val=""/>` +
	`<dgm:catLst><dgm:cat type="colorful" pri="10100"/></dgm:catLst>` +
	colorLabelXML("node0") +
	colorLabelXML("node1") +
	colorLabelXML("lnNode1") +
	`</dgm:colorsDef>`

// colorLabelXML renders one dgm:styleLbl whose fill/line cycle through the
// theme accent colors and whose text stays light.
func colorLabelXML(name string) string {
	return `<dgm:styleLbl name="` + name + `">` +
		`<dgm:fillClrLst meth="repeat"><a:schemeClr val="accent1"/><a:schemeClr val="accent2"/><a:schemeClr val="accent3"/><a:schemeClr val="accent4"/></dgm:fillClrLst>` +
		`<dgm:linClrLst meth="repeat"><a:schemeClr val="accent1"><a:shade val="60000"/></a:schemeClr></dgm:linClrLst>` +
		`<dgm:effectClrLst/>` +
		`<dgm:txLinClrLst/>` +
		`<dgm:txFillClrLst meth="repeat"><a:schemeClr val="lt1"/></dgm:txFillClrLst>` +
		`<dgm:txEffectClrLst/>` +
		`</dgm:styleLbl>`
}
