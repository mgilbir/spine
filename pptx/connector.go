package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// ConnectorKind identifies a connector's routing geometry.
type ConnectorKind int

const (
	// ConnectorStraight is a straight-line connector (a:prstGeom straightConnector1).
	ConnectorStraight ConnectorKind = iota
	// ConnectorElbow is a right-angle (bent) connector (a:prstGeom bentConnector3).
	ConnectorElbow
	// ConnectorCurved is a curved connector (a:prstGeom curvedConnector3).
	ConnectorCurved
)

// String returns the kind's name.
func (k ConnectorKind) String() string {
	switch k {
	case ConnectorElbow:
		return "elbow"
	case ConnectorCurved:
		return "curved"
	default:
		return "straight"
	}
}

// presetGeom maps a ConnectorKind to the DrawingML preset connector geometry
// PowerPoint uses by default for that routing.
func (k ConnectorKind) presetGeom() string {
	switch k {
	case ConnectorElbow:
		return "bentConnector3"
	case ConnectorCurved:
		return "curvedConnector3"
	default:
		return "straightConnector1"
	}
}

// connectorKindFromPreset maps a DrawingML preset geometry name back to a
// ConnectorKind. Unrecognized presets fall back to straight.
func connectorKindFromPreset(prst string) ConnectorKind {
	switch {
	case len(prst) >= 4 && prst[:4] == "bent":
		return ConnectorElbow
	case len(prst) >= 6 && prst[:6] == "curved":
		return ConnectorCurved
	default:
		return ConnectorStraight
	}
}

// Connector is a connection shape (p:cxnSp): a line that links two shapes (or
// two free points) with a straight, elbow, or curved route. Endpoint bindings
// made to shapes are resolved to their cNvPr ids at save time, so a connector
// can be drawn to API-created shapes whose ids are only assigned on save (see
// Shape.ID).
type Connector struct {
	BaseShape

	kind ConnectorKind
	// preset is the DrawingML preset geometry name. For an API-created
	// connector it derives from kind; for a materialized one it is read back
	// verbatim, so an unrecognized preset still reports faithfully.
	preset string

	// spPr accumulates line styling set through SetLine and friends; it is
	// merged into the marshaled a:spPr like the other shapes' style buffers.
	spPr dml.SpPr

	// Endpoint bindings. A non-nil startShape/endShape binds that end to a
	// shape's connection site (resolved to its id at save); a nil one leaves
	// the end free, positioned by the connector's own xfrm.
	startShape Shape
	startSite  uint32
	endShape   Shape
	endSite    uint32
	// startBoundID/endBoundID hold the bound cNvPr id read back from a
	// materialized connector (valid only when the has*Bound flag is set).
	startBoundID  uint32
	startBoundIdx uint32
	endBoundID    uint32
	endBoundIdx   uint32
	hasStartBound bool
	hasEndBound   bool

	// flipH/flipV orient a free-point connector so its route runs from the
	// first point to the second regardless of their relative position.
	flipH bool
	flipV bool

	// sourceCxn is the parsed p:cxnSp node this connector was materialized from,
	// or the node built for it on the first save (nil until then). Dirty edits
	// are flushed into it in place, and endpoint bindings are resolved into its
	// cNvCxnSpPr, so caller-held pointers keep working across saves.
	sourceCxn *oxml.ConnectionShape
}

// NewConnector creates a connector with the given routing geometry. Bind its
// ends with Connect / SetStartShape / SetEndShape, or place it freely with
// SetPoints, then add it to a slide with Slide.AddConnector or Slide.AddShape.
func NewConnector(kind ConnectorKind) *Connector {
	return &Connector{kind: kind, preset: kind.presetGeom()}
}

// AddConnector adds a connector (a p:cxnSp) inside the group and returns it for
// configuration, mirroring Slide.AddConnector. The connector becomes a group
// child: place it with SetPoints (in the group's child coordinate space) or bind
// its ends to shapes with Connect / SetStartShape / SetEndShape (the cNvPr ids
// resolve on save, so it can target other group children or slide shapes). On a
// group loaded from a file it is appended to the parsed p:grpSp on save without
// disturbing the existing children.
func (g *GroupShape) AddConnector(kind ConnectorKind) *Connector {
	c := NewConnector(kind)
	g.AddChild(c)
	return c
}

// Connectors returns the connectors (p:cxnSp) that are direct children of the
// group, in child order.
func (g *GroupShape) Connectors() []*Connector {
	var out []*Connector
	for _, child := range g.children {
		if c, ok := child.(*Connector); ok {
			out = append(out, c)
		}
	}
	return out
}

// ShapeType returns ShapeTypeConnector.
func (c *Connector) ShapeType() ShapeType { return ShapeTypeConnector }

// Kind returns the connector's routing geometry.
func (c *Connector) Kind() ConnectorKind { return c.kind }

// SetKind changes the connector's routing geometry.
func (c *Connector) SetKind(kind ConnectorKind) {
	c.kind = kind
	c.preset = kind.presetGeom()
	c.dirty = true
}

// Connect binds both ends of the connector to shapes: the start to startSite on
// start, the end to endSite on end. Connection-site indexes identify the
// anchor points a shape exposes (0 is typically the top; the exact set depends
// on the shape's geometry). The bindings are written on save, once the target
// shapes have ids.
func (c *Connector) Connect(start Shape, startSite uint32, end Shape, endSite uint32) {
	c.SetStartShape(start, startSite)
	c.SetEndShape(end, endSite)
}

// SetStartShape binds the connector's start to a connection site on a shape.
func (c *Connector) SetStartShape(shape Shape, site uint32) {
	c.startShape = shape
	c.startSite = site
	c.dirty = true
}

// SetEndShape binds the connector's end to a connection site on a shape.
func (c *Connector) SetEndShape(shape Shape, site uint32) {
	c.endShape = shape
	c.endSite = site
	c.dirty = true
}

// SetPoints places a free connector between two absolute points (in EMUs),
// leaving either end unbound. The connector's frame is the bounding box of the
// two points; flip flags orient the route from the first point to the second.
func (c *Connector) SetPoints(x1, y1, x2, y2 dml.EMU) {
	minX, minY := x1, y1
	if x2 < minX {
		minX = x2
	}
	if y2 < minY {
		minY = y2
	}
	w, h := x2-x1, y2-y1
	if w < 0 {
		w = -w
	}
	if h < 0 {
		h = -h
	}
	c.x, c.y, c.width, c.height = minX, minY, w, h
	c.flipH = x2 < x1
	c.flipV = y2 < y1
	c.dirty = true
}

// StartConnection reports the cNvPr id and connection-site index the start is
// bound to, and whether the start is bound at all (false = free endpoint). For
// a start bound to an API-created shape it reports that shape's current id,
// which is 0 until the deck is saved.
func (c *Connector) StartConnection() (id, idx uint32, bound bool) {
	if c.startShape != nil {
		return c.startShape.ID(), c.startSite, true
	}
	if c.hasStartBound {
		return c.startBoundID, c.startBoundIdx, true
	}
	return 0, 0, false
}

// EndConnection reports the cNvPr id and connection-site index the end is bound
// to (see StartConnection).
func (c *Connector) EndConnection() (id, idx uint32, bound bool) {
	if c.endShape != nil {
		return c.endShape.ID(), c.endSite, true
	}
	if c.hasEndBound {
		return c.endBoundID, c.endBoundIdx, true
	}
	return 0, 0, false
}

// SetLine sets the connector's line (color, width in points, dash).
func (c *Connector) SetLine(line dml.Line) {
	line.ApplyToSpPr(&c.spPr)
	c.dirty = true
}

// SetLineWidth sets the line width in points, preserving the current color and
// dash.
func (c *Connector) SetLineWidth(points float64) {
	l := c.line()
	l.Width = points
	c.SetLine(l)
}

// SetLineColor sets the line color, preserving the current width and dash.
func (c *Connector) SetLineColor(color dml.Color) {
	l := c.line()
	l.Color = color
	c.SetLine(l)
}

// SetLineDash sets the line dash pattern, preserving the current width and
// color.
func (c *Connector) SetLineDash(dash dml.DashStyle) {
	l := c.line()
	l.Dash = dash
	c.SetLine(l)
}

// line reconstructs the current line settings from the accumulated a:ln so the
// per-property setters can layer onto each other.
func (c *Connector) line() dml.Line {
	var l dml.Line
	ln := c.spPr.Ln
	if ln == nil {
		return l
	}
	if ln.W != nil {
		l.Width = float64(*ln.W) / 12700.0
	}
	if col := oxmlToColor(ln.SolidFill); col != nil {
		l.Color = *col
	}
	if ln.PrstDash != nil {
		l.Dash = dml.DashStyle(ln.PrstDash.Val)
	}
	return l
}

// LineWidth returns the connector's line width in points (0 when unset).
func (c *Connector) LineWidth() float64 { return c.line().Width }

// LineColor returns the connector's line color, or nil when the line has no
// explicit solid color.
func (c *Connector) LineColor() *dml.Color {
	if c.spPr.Ln == nil {
		return nil
	}
	return oxmlToColor(c.spPr.Ln.SolidFill)
}

// LineDash returns the connector's line dash pattern ("" when unset/solid).
func (c *Connector) LineDash() dml.DashStyle { return c.line().Dash }

// connectorToOxml builds a p:cxnSp node for an API-created connector. Endpoint
// bindings are left to resolveConnectorBindings, which runs once every shape on
// the slide has an id.
func connectorToOxml(c *Connector, id uint32) *oxml.ConnectionShape {
	name := c.Name()
	if name == "" {
		name = "Connector"
	}
	preset := c.preset
	if preset == "" {
		preset = c.kind.presetGeom()
	}

	spPr := &dml.SpPr{
		Xfrm: &dml.Xfrm{
			FlipH: c.flipH,
			FlipV: c.flipV,
			Off:   &dml.OffXML{X: int64(c.x), Y: int64(c.y)},
			Ext:   &dml.ExtXML{Cx: int64(c.width), Cy: int64(c.height)},
		},
		PrstGeom: &dml.PrstGeom{
			Prst:  preset,
			AvLst: &dml.AvLst{},
		},
	}
	applyShapeStyle(spPr, &c.spPr)

	node := &oxml.ConnectionShape{
		NvCxnSpPr: &oxml.NvCxnSpPr{
			CNvPr:      &dml.CNvPr{Id: id, Name: name},
			CNvCxnSpPr: &dml.CNvCxnSpPr{},
			NvPr:       &oxml.NvPr{},
		},
		SpPr: spPr,
	}
	c.sourceCxn = node
	c.sourceID = id
	return node
}

// updateConnectorNode flushes a dirty materialized connector into its parsed
// p:cxnSp node: name, geometry (kind + xfrm), and line styling. Endpoint
// bindings are handled separately by resolveConnectorBindings. Everything the
// model does not represent (style refs, extLst) is left untouched.
func updateConnectorNode(cs *oxml.ConnectionShape, c *Connector) {
	if !c.dirty {
		return
	}
	if cs.NvCxnSpPr != nil && cs.NvCxnSpPr.CNvPr != nil && c.name != "" {
		cs.NvCxnSpPr.CNvPr.Name = c.name
	}
	if cs.SpPr == nil {
		cs.SpPr = &dml.SpPr{}
	}
	if c.preset != "" {
		if cs.SpPr.PrstGeom == nil {
			cs.SpPr.PrstGeom = &dml.PrstGeom{AvLst: &dml.AvLst{}}
		}
		cs.SpPr.PrstGeom.Prst = c.preset
	}
	updateConnectorXfrm(cs.SpPr, c)
	applyShapeStyle(cs.SpPr, &c.spPr)
}

// updateConnectorXfrm writes the connector's position, size, and flips into its
// xfrm, creating one only when the connector carries an explicit placement (a
// parsed connector without an xfrm keeps inheriting its geometry).
func updateConnectorXfrm(spPr *dml.SpPr, c *Connector) {
	if spPr.Xfrm == nil {
		if c.x == 0 && c.y == 0 && c.width == 0 && c.height == 0 && !c.flipH && !c.flipV {
			return
		}
		spPr.Xfrm = &dml.Xfrm{}
	}
	spPr.Xfrm.FlipH = c.flipH
	spPr.Xfrm.FlipV = c.flipV
	spPr.Xfrm.Off = &dml.OffXML{X: int64(c.x), Y: int64(c.y)}
	spPr.Xfrm.Ext = &dml.ExtXML{Cx: int64(c.width), Cy: int64(c.height)}
}

// oxmlCxnSpToGoConnector materializes a parsed p:cxnSp into a Connector.
func oxmlCxnSpToGoConnector(cs *oxml.ConnectionShape) *Connector {
	if cs == nil {
		return nil
	}
	c := &Connector{sourceCxn: cs}

	if cs.NvCxnSpPr != nil {
		if cn := cs.NvCxnSpPr.CNvPr; cn != nil {
			c.name = cn.Name
			c.sourceID = cn.Id
		}
		if cnv := cs.NvCxnSpPr.CNvCxnSpPr; cnv != nil {
			if cnv.StCxn != nil {
				c.startBoundID, c.startBoundIdx, c.hasStartBound = cnv.StCxn.Id, cnv.StCxn.Idx, true
			}
			if cnv.EndCxn != nil {
				c.endBoundID, c.endBoundIdx, c.hasEndBound = cnv.EndCxn.Id, cnv.EndCxn.Idx, true
			}
		}
	}

	if cs.SpPr != nil {
		if cs.SpPr.PrstGeom != nil {
			c.preset = cs.SpPr.PrstGeom.Prst
			c.kind = connectorKindFromPreset(c.preset)
		}
		if cs.SpPr.Xfrm != nil {
			xf := cs.SpPr.Xfrm
			c.flipH, c.flipV = xf.FlipH, xf.FlipV
			if xf.Off != nil {
				c.x, c.y = dml.EMU(xf.Off.X), dml.EMU(xf.Off.Y)
			}
			if xf.Ext != nil {
				c.width, c.height = dml.EMU(xf.Ext.Cx), dml.EMU(xf.Ext.Cy)
			}
		}
		// Mirror the parsed line into the style buffer so the per-property
		// setters (and readers) operate on it without disturbing the node until
		// an edit marks the connector dirty.
		if cs.SpPr.Ln != nil {
			c.spPr.Ln = cs.SpPr.Ln
		}
	}

	return c
}
