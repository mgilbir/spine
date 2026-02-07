package spectest

import (
	"reflect"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/dml/chart"
	"github.com/mgilbir/spine/common/dml/diagram"
	"github.com/mgilbir/spine/common/vml"
)

// DMLTypeMap maps DrawingML element local names to Go types.
// These types are from common/dml/ and are publicly accessible.
var DMLTypeMap = map[string]reflect.Type{
	// Theme types (xml_theme.go)
	"theme":          reflect.TypeOf(dml.Theme{}),
	"themeElements":  reflect.TypeOf(dml.ThemeElements{}),
	"clrScheme":      reflect.TypeOf(dml.ClrScheme{}),
	"fontScheme":     reflect.TypeOf(dml.FontScheme{}),
	"fmtScheme":      reflect.TypeOf(dml.FmtScheme{}),
	"fillStyleLst":   reflect.TypeOf(dml.FillStyleLst{}),
	"lnStyleLst":     reflect.TypeOf(dml.LnStyleLst{}),
	"effectStyleLst": reflect.TypeOf(dml.EffectStyleLst{}),
	"bgFillStyleLst": reflect.TypeOf(dml.BgFillStyleLst{}),

	// Shape types (xml_shape.go)
	"spPr":        reflect.TypeOf(dml.SpPr{}),
	"grpSpPr":     reflect.TypeOf(dml.GrpSpPr{}),
	"spLocks":     reflect.TypeOf(dml.SpLocks{}),
	"grpSpLocks":  reflect.TypeOf(dml.GrpSpLocks{}),
	"picLocks":    reflect.TypeOf(dml.PicLocks{}),
	"cxnSpLocks":  reflect.TypeOf(dml.CxnSpLocks{}),
	"cNvPr":       reflect.TypeOf(dml.CNvPr{}),

	// Text types (xml_text.go)
	"txBody":  reflect.TypeOf(dml.TxBody{}),
	"bodyPr":  reflect.TypeOf(dml.BodyPr{}),
	"lstStyle": reflect.TypeOf(dml.LstStyle{}),
	"p":        reflect.TypeOf(dml.P{}),
	"r":        reflect.TypeOf(dml.R{}),
	"br":       reflect.TypeOf(dml.Br{}),
	"fld":      reflect.TypeOf(dml.Fld{}),

	// Geometry types (xml_geometry.go)
	"prstGeom": reflect.TypeOf(dml.PrstGeom{}),
	"custGeom": reflect.TypeOf(dml.CustGeom{}),
	"avLst":    reflect.TypeOf(dml.AvLst{}),
	"gdLst":    reflect.TypeOf(dml.GdLst{}),
	"pathLst":  reflect.TypeOf(dml.PathLst{}),
	"xfrm":     reflect.TypeOf(dml.Xfrm{}),

	// Line types (xml_line.go)
	"ln":       reflect.TypeOf(dml.Ln{}),
	"prstDash": reflect.TypeOf(dml.PrstDash{}),
	"custDash": reflect.TypeOf(dml.CustDash{}),
	"lnRef":    reflect.TypeOf(dml.LnRef{}),
	"fillRef":  reflect.TypeOf(dml.FillRef{}),

	// Fill types (xml_types.go)
	"solidFill": reflect.TypeOf(dml.SolidFill{}),
	"gradFill":  reflect.TypeOf(dml.GradFill{}),
	"pattFill":  reflect.TypeOf(dml.PattFill{}),
	"blipFill":  reflect.TypeOf(dml.BlipFillXML{}),
	"noFill":    reflect.TypeOf(dml.NoFillXML{}),

	// Color types (xml_types.go)
	"srgbClr":  reflect.TypeOf(dml.SrgbClr{}),
	"schemeClr": reflect.TypeOf(dml.SchemeClr{}),
	"sysClr":   reflect.TypeOf(dml.SystemClr{}),
	"hslClr":   reflect.TypeOf(dml.HslClr{}),
	"prstClr":  reflect.TypeOf(dml.PrstClr{}),
	"scrgbClr": reflect.TypeOf(dml.ScRgbClr{}),

	// Effect types (xml_effect.go)
	"effectLst": reflect.TypeOf(dml.EffectLst{}),
	"outerShdw": reflect.TypeOf(dml.OuterShdw{}),
	"innerShdw": reflect.TypeOf(dml.InnerShdw{}),

	// Table types (xml_table.go)
	"tbl":    reflect.TypeOf(dml.Tbl{}),
	"tblPr":  reflect.TypeOf(dml.TblPr{}),
	"tr":     reflect.TypeOf(dml.Tr{}),
	"tc":     reflect.TypeOf(dml.Tc{}),
	"tcPr":   reflect.TypeOf(dml.TcPr{}),
	"tblGrid": reflect.TypeOf(dml.TblGrid{}),
	"gridCol": reflect.TypeOf(dml.GridCol{}),

	// 3D types (xml_3d.go)
	"scene3d": reflect.TypeOf(dml.Scene3d{}),
	"sp3d":    reflect.TypeOf(dml.Sp3d{}),

	// Picture types (xml_picture.go)
	"blip": reflect.TypeOf(dml.Blip{}),

	// Media types (xml_media.go)
	"graphic":     reflect.TypeOf(dml.Graphic{}),
	"graphicData": reflect.TypeOf(dml.GraphicData{}),

	// Extension types (xml_extension.go)
	"extLst": reflect.TypeOf(dml.ExtLst{}),
}

// ChartTypeMap maps Chart element local names to Go types.
var ChartTypeMap = map[string]reflect.Type{
	"chartSpace": reflect.TypeOf(chart.ChartSpace{}),
	"chart":      reflect.TypeOf(chart.Chart{}),
	"plotArea":   reflect.TypeOf(chart.PlotArea{}),
	"barChart":   reflect.TypeOf(chart.BarChart{}),
	"lineChart":  reflect.TypeOf(chart.LineChart{}),
	"pieChart":   reflect.TypeOf(chart.PieChart{}),
	"areaChart":  reflect.TypeOf(chart.AreaChart{}),
}

// DiagramTypeMap maps Diagram element local names to Go types.
var DiagramTypeMap = map[string]reflect.Type{
	"dataModel": reflect.TypeOf(diagram.DataModel{}),
}

// WML/SML/PML type-mapped tests are in their respective internal packages:
//   - docx/internal/oxml/spec_test.go
//   - xlsx/internal/oxml/spec_test.go
//   - pptx/internal/oxml/spec_test.go

// VMLTypeMap maps VML element local names to Go types.
// These types are from common/vml/ and are publicly accessible.
var VMLTypeMap = map[string]reflect.Type{
	"group":     reflect.TypeOf(vml.Group{}),
	"shape":     reflect.TypeOf(vml.Shape{}),
	"shapetype": reflect.TypeOf(vml.Shapetype{}),
	"rect":      reflect.TypeOf(vml.Rect{}),
	"roundrect": reflect.TypeOf(vml.RoundRect{}),
	"oval":      reflect.TypeOf(vml.Oval{}),
	"line":      reflect.TypeOf(vml.Line{}),
	"polyline":  reflect.TypeOf(vml.Polyline{}),
	"curve":     reflect.TypeOf(vml.Curve{}),
	"arc":       reflect.TypeOf(vml.Arc{}),
	"image":     reflect.TypeOf(vml.ImageEl{}),
	"fill":      reflect.TypeOf(vml.Fill{}),
	"stroke":    reflect.TypeOf(vml.Stroke{}),
	"shadow":    reflect.TypeOf(vml.Shadow{}),
	"textbox":   reflect.TypeOf(vml.Textbox{}),
	"textpath":  reflect.TypeOf(vml.TextPath{}),
	"imagedata": reflect.TypeOf(vml.ImageData{}),
	"path":      reflect.TypeOf(vml.PathEl{}),
	"formulas":  reflect.TypeOf(vml.Formulas{}),
	"handles":   reflect.TypeOf(vml.Handles{}),
	"lock":      reflect.TypeOf(vml.Lock{}),
	"callout":   reflect.TypeOf(vml.Callout{}),
	"extrusion": reflect.TypeOf(vml.Extrusion{}),
	"signatureline": reflect.TypeOf(vml.SignatureLine{}),
	"wrap":      reflect.TypeOf(vml.Wrap{}),
	"anchorlock": reflect.TypeOf(vml.AnchorLock{}),
	"ClientData": reflect.TypeOf(vml.ClientData{}),
}
