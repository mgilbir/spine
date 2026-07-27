// Package dml tests for DrawingML table types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestDML_CT_Table tests CT_Table type (a:tbl)
func TestDML_CT_Table(t *testing.T) {
	var v Tbl
	input := `<a:tbl xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:tblPr firstRow="1" bandRow="1"/>
		<a:tblGrid>
			<a:gridCol w="914400"/>
			<a:gridCol w="914400"/>
		</a:tblGrid>
		<a:tr h="370840">
			<a:tc>
				<a:txBody>
					<a:bodyPr/>
					<a:p>
						<a:r>
							<a:t>Cell 1</a:t>
						</a:r>
					</a:p>
				</a:txBody>
				<a:tcPr/>
			</a:tc>
			<a:tc>
				<a:txBody>
					<a:bodyPr/>
					<a:p>
						<a:r>
							<a:t>Cell 2</a:t>
						</a:r>
					</a:p>
				</a:txBody>
				<a:tcPr/>
			</a:tc>
		</a:tr>
	</a:tbl>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.TblPr == nil {
		t.Error("TblPr is nil")
	}
	if v.TblGrid == nil {
		t.Error("TblGrid is nil")
	}
	if len(v.Tr) != 1 {
		t.Errorf("Tr length = %d, want 1", len(v.Tr))
	}
}

// TestDML_CT_TableProperties tests CT_TableProperties type (a:tblPr)
func TestDML_CT_TableProperties(t *testing.T) {
	var v TblPr
	input := `<a:tblPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		rtl="0" firstRow="1" firstCol="1" lastRow="0" lastCol="0" bandRow="1" bandCol="0">
		<a:solidFill>
			<a:srgbClr val="FFFFFF"/>
		</a:solidFill>
	</a:tblPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.FirstRow {
		t.Error("FirstRow should be true")
	}
	if !v.FirstCol {
		t.Error("FirstCol should be true")
	}
	if !v.BandRow {
		t.Error("BandRow should be true")
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
}

// TestDML_TblPr_InlineTableStyle pins C342: CT_TableProperties models an
// xs:choice of an inline <a:tableStyle> (CT_TableStyle) or a <a:tableStyleId>
// GUID reference. Only TableStyleId was modeled, so an inline tableStyle was
// parsed to nothing and dropped on re-marshal. It now round-trips.
func TestDML_TblPr_InlineTableStyle(t *testing.T) {
	var v TblPr
	input := `<a:tblPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" firstRow="1">` +
		`<a:tableStyle styleId="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" styleName="Inline">` +
		`<a:wholeTbl><a:tcStyle><a:noFill/></a:tcStyle></a:wholeTbl>` +
		`</a:tableStyle>` +
		`</a:tblPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.TableStyle == nil {
		t.Fatal("inline tableStyle dropped on unmarshal (TableStyle is nil)")
	}
	if v.TableStyle.StyleId != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("TableStyle.StyleId = %q", v.TableStyle.StyleId)
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, "a")
	b.MarshalElement(NsDrawingML, "tblPr", &v)
	if err := b.Err(); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if out := b.String(); !strings.Contains(out, "<a:tableStyle") || !strings.Contains(out, "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}") {
		t.Errorf("inline tableStyle deleted on re-marshal: %s", out)
	}
}

// TestDML_CT_TableGrid tests CT_TableGrid type (a:tblGrid)
func TestDML_CT_TableGrid(t *testing.T) {
	var v TblGrid
	input := `<a:tblGrid xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:gridCol w="914400"/>
		<a:gridCol w="914400"/>
		<a:gridCol w="914400"/>
	</a:tblGrid>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.GridCol) != 3 {
		t.Errorf("GridCol length = %d, want 3", len(v.GridCol))
	}
}

// TestDML_CT_TableCol tests CT_TableCol type (a:gridCol)
func TestDML_CT_TableCol(t *testing.T) {
	var v GridCol
	input := `<a:gridCol xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" w="1828800"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.W != 1828800 {
		t.Errorf("W = %d, want 1828800", v.W)
	}
}

// TestDML_CT_TableRow tests CT_TableRow type (a:tr)
func TestDML_CT_TableRow(t *testing.T) {
	var v Tr
	input := `<a:tr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" h="370840">
		<a:tc>
			<a:txBody>
				<a:bodyPr/>
				<a:p>
					<a:r>
						<a:t>Cell</a:t>
					</a:r>
				</a:p>
			</a:txBody>
			<a:tcPr/>
		</a:tc>
	</a:tr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.H == nil || *v.H != 370840 {
		t.Errorf("H = %v, want 370840", v.H)
	}
	if len(v.Tc) != 1 {
		t.Errorf("Tc length = %d, want 1", len(v.Tc))
	}
}

// TestDML_CT_TableCell tests CT_TableCell type (a:tc)
func TestDML_CT_TableCell(t *testing.T) {
	var v Tc
	input := `<a:tc xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" rowSpan="2" gridSpan="1">
		<a:txBody>
			<a:bodyPr/>
			<a:p>
				<a:r>
					<a:t>Merged Cell</a:t>
				</a:r>
			</a:p>
		</a:txBody>
		<a:tcPr anchor="ctr"/>
	</a:tc>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.RowSpan != 2 {
		t.Errorf("RowSpan = %d, want 2", v.RowSpan)
	}
	if v.TxBody == nil {
		t.Error("TxBody is nil")
	}
	if v.TcPr == nil {
		t.Error("TcPr is nil")
	}
}

// TestDML_CT_TableCellProperties tests CT_TableCellProperties type (a:tcPr)
func TestDML_CT_TableCellProperties(t *testing.T) {
	var v TcPr
	input := `<a:tcPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		marL="91440" marR="91440" marT="45720" marB="45720" vert="horz" anchor="ctr" anchorCtr="1">
		<a:lnL w="12700">
			<a:solidFill>
				<a:srgbClr val="000000"/>
			</a:solidFill>
		</a:lnL>
		<a:lnT w="12700">
			<a:solidFill>
				<a:srgbClr val="000000"/>
			</a:solidFill>
		</a:lnT>
		<a:solidFill>
			<a:srgbClr val="FFFFFF"/>
		</a:solidFill>
	</a:tcPr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.MarL == nil || *v.MarL != 91440 {
		t.Errorf("MarL = %v, want 91440", v.MarL)
	}
	if v.Anchor != "ctr" {
		t.Errorf("Anchor = %q, want ctr", v.Anchor)
	}
	if v.AnchorCtr == nil || !*v.AnchorCtr {
		t.Error("AnchorCtr should be true")
	}
	if v.LnL == nil {
		t.Error("LnL is nil")
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
}

// TestDML_CT_Cell3D tests CT_Cell3D type (a:cell3D)
func TestDML_CT_Cell3D(t *testing.T) {
	var v Cell3D
	input := `<a:cell3D xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" prstMaterial="matte">
		<a:bevel w="63500" h="25400"/>
		<a:lightRig rig="threePt" dir="t"/>
	</a:cell3D>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.PrstMaterial != "matte" {
		t.Errorf("PrstMaterial = %q, want matte", v.PrstMaterial)
	}
	if v.Bevel == nil {
		t.Error("Bevel is nil")
	}
	if v.LightRig == nil {
		t.Error("LightRig is nil")
	}
}

// TestDML_CT_TableStyle tests CT_TableStyle type (a:tblStyle)
func TestDML_CT_TableStyle(t *testing.T) {
	var v TableStyle
	input := `<a:tblStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		styleId="{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" styleName="Medium Style 2 - Accent 1">
		<a:wholeTbl>
			<a:tcTxStyle>
				<a:fontRef idx="minor">
					<a:schemeClr val="dk1"/>
				</a:fontRef>
			</a:tcTxStyle>
			<a:tcStyle>
				<a:tcBdr>
					<a:left>
						<a:ln w="12700">
							<a:solidFill>
								<a:schemeClr val="accent1"/>
							</a:solidFill>
						</a:ln>
					</a:left>
				</a:tcBdr>
				<a:solidFill>
					<a:schemeClr val="lt1"/>
				</a:solidFill>
			</a:tcStyle>
		</a:wholeTbl>
	</a:tblStyle>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.StyleId != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("StyleId = %q, want {5C22544A-7EE6-4342-B048-85BDC9FD1C3A}", v.StyleId)
	}
	if v.StyleName != "Medium Style 2 - Accent 1" {
		t.Errorf("StyleName = %q, want 'Medium Style 2 - Accent 1'", v.StyleName)
	}
	if v.WholeTbl == nil {
		t.Error("WholeTbl is nil")
	}
}

// TestDML_CT_TablePartStyle tests CT_TablePartStyle type (a:wholeTbl, a:band1H, etc.)
func TestDML_CT_TablePartStyle(t *testing.T) {
	var v TablePartStyle
	input := `<a:band1H xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:tcTxStyle b="on">
			<a:fontRef idx="minor">
				<a:schemeClr val="dk1"/>
			</a:fontRef>
		</a:tcTxStyle>
		<a:tcStyle>
			<a:solidFill>
				<a:schemeClr val="accent1">
					<a:tint val="40000"/>
				</a:schemeClr>
			</a:solidFill>
		</a:tcStyle>
	</a:band1H>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.TcTxStyle == nil {
		t.Error("TcTxStyle is nil")
	}
	if v.TcStyle == nil {
		t.Error("TcStyle is nil")
	}
}

// TestDML_CT_TableStyleTextStyle tests CT_TableStyleTextStyle type (a:tcTxStyle)
func TestDML_CT_TableStyleTextStyle(t *testing.T) {
	var v TcTxStyle
	input := `<a:tcTxStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" b="on" i="off">
		<a:fontRef idx="minor">
			<a:schemeClr val="lt1"/>
		</a:fontRef>
	</a:tcTxStyle>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.B != "on" {
		t.Errorf("B = %q, want on", v.B)
	}
	if v.I != "off" {
		t.Errorf("I = %q, want off", v.I)
	}
	if v.FontRef == nil {
		t.Error("FontRef is nil")
	}
}

// TestDML_CT_TableStyleCellStyle tests CT_TableStyleCellStyle type (a:tcStyle)
func TestDML_CT_TableStyleCellStyle(t *testing.T) {
	var v TcStyle
	input := `<a:tcStyle xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:tcBdr>
			<a:left>
				<a:ln w="12700"/>
			</a:left>
		</a:tcBdr>
		<a:solidFill>
			<a:schemeClr val="accent1"/>
		</a:solidFill>
	</a:tcStyle>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.TcBdr == nil {
		t.Error("TcBdr is nil")
	}
	if v.SolidFill == nil {
		t.Error("SolidFill is nil")
	}
}

// TestDML_CT_TableCellBorderStyle tests CT_TableCellBorderStyle type (a:tcBdr)
func TestDML_CT_TableCellBorderStyle(t *testing.T) {
	var v TcBdr
	input := `<a:tcBdr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:left>
			<a:ln w="12700">
				<a:solidFill>
					<a:srgbClr val="000000"/>
				</a:solidFill>
			</a:ln>
		</a:left>
		<a:right>
			<a:ln w="12700"/>
		</a:right>
		<a:top>
			<a:ln w="12700"/>
		</a:top>
		<a:bottom>
			<a:ln w="12700"/>
		</a:bottom>
	</a:tcBdr>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Left == nil {
		t.Error("Left is nil")
	}
	if v.Right == nil {
		t.Error("Right is nil")
	}
	if v.Top == nil {
		t.Error("Top is nil")
	}
	if v.Bottom == nil {
		t.Error("Bottom is nil")
	}
}
