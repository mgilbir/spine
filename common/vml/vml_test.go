package vml

import (
	"encoding/xml"
	"testing"
)

func TestShape_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic shape",
			xml:  `<shape xmlns="urn:schemas-microsoft-com:vml" id="shape1" style="position:absolute;left:0;top:0;width:100pt;height:50pt"/>`,
		},
		{
			name: "filled shape",
			xml:  `<shape xmlns="urn:schemas-microsoft-com:vml" id="shape2" style="width:200pt;height:100pt" filled="t" fillcolor="#FF0000" stroked="t" strokecolor="#000000" strokeweight="1pt"/>`,
		},
		{
			name: "shape with path",
			xml:  `<shape xmlns="urn:schemas-microsoft-com:vml" id="shape3" path="m 0,0 l 100,0, 100,100, 0,100 x e" coordsize="100,100"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Shape
			if err := xml.Unmarshal([]byte(tt.xml), &s); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&s)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var s2 Shape
			if err := xml.Unmarshal(out, &s2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}

			if s2.ID != s.ID {
				t.Errorf("ID = %q, want %q", s2.ID, s.ID)
			}
		})
	}
}

func TestShapetype_RoundTrip(t *testing.T) {
	xmlStr := `<shapetype xmlns="urn:schemas-microsoft-com:vml" id="_x0000_t75" coordsize="21600,21600" spt="75" path="m@4@5l@4@11@9@11@9@5xe" filled="f" stroked="f"/>`

	var st Shapetype
	if err := xml.Unmarshal([]byte(xmlStr), &st); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&st)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var st2 Shapetype
	if err := xml.Unmarshal(out, &st2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}

	if st2.ID != st.ID {
		t.Errorf("ID = %q, want %q", st2.ID, st.ID)
	}
	if st2.Spt != st.Spt {
		t.Errorf("Spt = %d, want %d", st2.Spt, st.Spt)
	}
}

func TestRect_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		style     string
		filled    string
		fillColor string
	}{
		{"basic rect", "rect1", "width:100pt;height:50pt", "t", "#FF0000"},
		{"unfilled rect", "rect2", "width:50pt;height:50pt", "f", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Rect{ID: tt.id, Style: tt.style, Filled: tt.filled, FillColor: tt.fillColor}
			out, err := xml.Marshal(r)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var r2 Rect
			if err := xml.Unmarshal(out, &r2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if r2.ID != tt.id {
				t.Errorf("ID = %q, want %q", r2.ID, tt.id)
			}
		})
	}
}

func TestRoundRect_RoundTrip(t *testing.T) {
	rr := &RoundRect{ID: "rr1", Style: "width:100pt;height:50pt", ArcSize: "10923f"}
	out, err := xml.Marshal(rr)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var rr2 RoundRect
	if err := xml.Unmarshal(out, &rr2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if rr2.ArcSize != "10923f" {
		t.Errorf("ArcSize = %q, want %q", rr2.ArcSize, "10923f")
	}
}

func TestOval_RoundTrip(t *testing.T) {
	o := &Oval{ID: "oval1", Style: "width:100pt;height:75pt", Filled: "t", FillColor: "#00FF00"}
	out, err := xml.Marshal(o)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var o2 Oval
	if err := xml.Unmarshal(out, &o2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if o2.ID != "oval1" {
		t.Errorf("ID = %q, want %q", o2.ID, "oval1")
	}
}

func TestLine_RoundTrip(t *testing.T) {
	tests := []struct {
		from string
		to   string
	}{
		{"0,0", "100,100"},
		{"50,0", "50,200"},
		{"0pt,0pt", "300pt,0pt"},
	}

	for _, tt := range tests {
		t.Run(tt.from+"_"+tt.to, func(t *testing.T) {
			l := &Line{From: tt.from, To: tt.to, StrokeColor: "#000000"}
			out, err := xml.Marshal(l)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var l2 Line
			if err := xml.Unmarshal(out, &l2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if l2.From != tt.from {
				t.Errorf("From = %q, want %q", l2.From, tt.from)
			}
			if l2.To != tt.to {
				t.Errorf("To = %q, want %q", l2.To, tt.to)
			}
		})
	}
}

func TestPolyline_RoundTrip(t *testing.T) {
	pl := &Polyline{
		ID:     "pl1",
		Points: "0,0 50,50 100,0 150,50",
		Filled: "f",
	}
	out, err := xml.Marshal(pl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var pl2 Polyline
	if err := xml.Unmarshal(out, &pl2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if pl2.Points != pl.Points {
		t.Errorf("Points = %q, want %q", pl2.Points, pl.Points)
	}
}

func TestCurve_RoundTrip(t *testing.T) {
	c := &Curve{
		From:     "0,0",
		Control1: "25,50",
		Control2: "75,50",
		To:       "100,0",
	}
	out, err := xml.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var c2 Curve
	if err := xml.Unmarshal(out, &c2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if c2.From != c.From {
		t.Errorf("From = %q, want %q", c2.From, c.From)
	}
	if c2.To != c.To {
		t.Errorf("To = %q, want %q", c2.To, c.To)
	}
}

func TestArc_RoundTrip(t *testing.T) {
	a := &Arc{
		ID:         "arc1",
		StartAngle: "0",
		EndAngle:   "90",
		Filled:     "t",
		FillColor:  "#0000FF",
	}
	out, err := xml.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var a2 Arc
	if err := xml.Unmarshal(out, &a2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if a2.StartAngle != "0" {
		t.Errorf("StartAngle = %q, want %q", a2.StartAngle, "0")
	}
}

func TestFill_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		fill    Fill
	}{
		{"solid", Fill{Type: "solid", Color: "#FF0000", On: "t"}},
		{"gradient", Fill{Type: "gradient", Color: "#FF0000", Color2: "#0000FF", Angle: "90", Focus: "100%"}},
		{"tile", Fill{Type: "tile", Src: "image.png"}},
		{"pattern", Fill{Type: "pattern", Color: "#FF0000", Color2: "#FFFFFF"}},
		{"with opacity", Fill{Type: "solid", Color: "#FF0000", Opacity: "0.5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(&tt.fill)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var f2 Fill
			if err := xml.Unmarshal(out, &f2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if f2.Type != tt.fill.Type {
				t.Errorf("Type = %q, want %q", f2.Type, tt.fill.Type)
			}
		})
	}
}

func TestStroke_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		stroke Stroke
	}{
		{"basic", Stroke{Color: "#000000", Weight: "1pt"}},
		{"dashed", Stroke{Color: "#FF0000", DashStyle: "dash", Weight: "2pt"}},
		{"with arrows", Stroke{StartArrow: "block", EndArrow: "classic", StartArrowWidth: "medium", EndArrowWidth: "wide"}},
		{"double line", Stroke{LineStyle: "thinThin", Weight: "3pt"}},
		{"with join", Stroke{JoinStyle: "round", EndCap: "round"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(&tt.stroke)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var s2 Stroke
			if err := xml.Unmarshal(out, &s2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestShadow_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		shadow Shadow
	}{
		{"basic", Shadow{On: "t", Color: "#808080", Offset: "3pt,3pt"}},
		{"double", Shadow{On: "t", Type: "double", Color: "#808080", Color2: "#C0C0C0", Offset: "3pt,3pt", Offset2: "6pt,6pt"}},
		{"emboss", Shadow{On: "t", Type: "emboss", Color: "#808080"}},
		{"perspective", Shadow{On: "t", Type: "perspective", Color: "#808080", Origin: "0.5,0.5", Matrix: "1,0,0,0.5,0,0"}},
		{"with opacity", Shadow{On: "t", Color: "#000000", Opacity: "0.5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(&tt.shadow)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var s2 Shadow
			if err := xml.Unmarshal(out, &s2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if s2.On != tt.shadow.On {
				t.Errorf("On = %q, want %q", s2.On, tt.shadow.On)
			}
		})
	}
}

func TestTextbox_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		style string
		inset string
	}{
		{"basic", "", ""},
		{"with style", "mso-direction-alt:auto", ""},
		{"with inset", "", "7.2pt,3.6pt,7.2pt,3.6pt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &Textbox{Style: tt.style, Inset: tt.inset}
			out, err := xml.Marshal(tb)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tb2 Textbox
			if err := xml.Unmarshal(out, &tb2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestTextPath_RoundTrip(t *testing.T) {
	tp := &TextPath{
		On:       "t",
		FitShape: "t",
		String:   "WordArt Text",
		Style:    "font-family:\"Arial\";font-size:36pt",
	}
	out, err := xml.Marshal(tp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var tp2 TextPath
	if err := xml.Unmarshal(out, &tp2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if tp2.String != "WordArt Text" {
		t.Errorf("String = %q, want %q", tp2.String, "WordArt Text")
	}
}

func TestImageData_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		id   ImageData
	}{
		{"basic", ImageData{Src: "image.png"}},
		{"with crop", ImageData{Src: "photo.jpg", CropTop: "10000f", CropBottom: "10000f", CropLeft: "5000f", CropRight: "5000f"}},
		{"with adjustments", ImageData{Src: "img.png", Gain: "1.5", BlackLevel: "0.1", Gamma: "1.2"}},
		{"grayscale", ImageData{Src: "img.png", GrayScale: "t"}},
		{"bilevel", ImageData{Src: "img.png", BiLevel: "t"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(&tt.id)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var id2 ImageData
			if err := xml.Unmarshal(out, &id2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if id2.Src != tt.id.Src {
				t.Errorf("Src = %q, want %q", id2.Src, tt.id.Src)
			}
		})
	}
}

func TestPathEl_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		v    string
	}{
		{"simple path", "m 0,0 l 100,0 100,100 0,100 x e"},
		{"with arcs", "m@4@5l@4@11@9@11@9@5xe"},
		{"complex", "m 0,0 c 50,0,100,50,100,100 l 0,100 x e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PathEl{V: tt.v}
			out, err := xml.Marshal(p)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var p2 PathEl
			if err := xml.Unmarshal(out, &p2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if p2.V != tt.v {
				t.Errorf("V = %q, want %q", p2.V, tt.v)
			}
		})
	}
}

func TestFormulas_RoundTrip(t *testing.T) {
	f := &Formulas{
		F: []*Formula{
			{Eqn: "if lineDrawn pixelLineWidth 0"},
			{Eqn: "sum @0 1 0"},
			{Eqn: "sum 0 0 @1"},
			{Eqn: "prod @2 1 2"},
		},
	}
	out, err := xml.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var f2 Formulas
	if err := xml.Unmarshal(out, &f2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(f2.F) != 4 {
		t.Errorf("Expected 4 formulas, got %d", len(f2.F))
	}
}

func TestHandle_RoundTrip(t *testing.T) {
	h := &Handle{
		Position: "#0,bottomRight",
		XRange:   "6629,14971",
	}
	out, err := xml.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var h2 Handle
	if err := xml.Unmarshal(out, &h2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if h2.Position != h.Position {
		t.Errorf("Position = %q, want %q", h2.Position, h.Position)
	}
}

func TestLock_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		aspectRatio string
		text        string
		rotation    string
	}{
		{"lock aspect ratio", "t", "f", "f"},
		{"lock text", "f", "t", "f"},
		{"lock rotation", "f", "f", "t"},
		{"lock all", "t", "t", "t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Lock{AspectRatio: tt.aspectRatio, Text: tt.text, Rotation: tt.rotation}
			out, err := xml.Marshal(l)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var l2 Lock
			if err := xml.Unmarshal(out, &l2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if l2.AspectRatio != tt.aspectRatio {
				t.Errorf("AspectRatio = %q, want %q", l2.AspectRatio, tt.aspectRatio)
			}
		})
	}
}

func TestExtrusion_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		ext    Extrusion
	}{
		{"parallel", Extrusion{On: "t", Type: "parallel", BackDepth: "1in"}},
		{"perspective", Extrusion{On: "t", Type: "perspective", ViewPoint: "0,0,200", ViewPointOrigin: "0.5,0.5"}},
		{"with lighting", Extrusion{On: "t", LightPosition: "-50000,-50000,20000", LightLevel: "52000", LightHarsh: "t"}},
		{"metallic", Extrusion{On: "t", Metal: "t", Shininess: "50000", Specularity: "80000"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := xml.Marshal(&tt.ext)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var e2 Extrusion
			if err := xml.Unmarshal(out, &e2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if e2.On != tt.ext.On {
				t.Errorf("On = %q, want %q", e2.On, tt.ext.On)
			}
		})
	}
}

func TestCallout_RoundTrip(t *testing.T) {
	c := &Callout{
		On:      "t",
		Type:    "rectangle",
		Gap:     "10",
		Angle:   "auto",
		DropAuto: "t",
	}
	out, err := xml.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var c2 Callout
	if err := xml.Unmarshal(out, &c2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if c2.On != "t" {
		t.Errorf("On = %q, want %q", c2.On, "t")
	}
}

func TestSignatureLine_RoundTrip(t *testing.T) {
	sl := &SignatureLine{
		ID:              "{00000000-0000-0000-0000-000000000001}",
		SuggestedSigner: "John Doe",
		SuggestedSignerEmail: "john@example.com",
		ShowSignDate:    "t",
	}
	out, err := xml.Marshal(sl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sl2 SignatureLine
	if err := xml.Unmarshal(out, &sl2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if sl2.SuggestedSigner != "John Doe" {
		t.Errorf("SuggestedSigner = %q, want %q", sl2.SuggestedSigner, "John Doe")
	}
}

func TestWrap_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		wrapType string
		side    string
	}{
		{"square both", "square", "both"},
		{"tight left", "tight", "left"},
		{"top and bottom", "topAndBottom", ""},
		{"through right", "through", "right"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &Wrap{Type: tt.wrapType, Side: tt.side}
			out, err := xml.Marshal(w)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var w2 Wrap
			if err := xml.Unmarshal(out, &w2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if w2.Type != tt.wrapType {
				t.Errorf("Type = %q, want %q", w2.Type, tt.wrapType)
			}
		})
	}
}

func TestGroup_RoundTrip(t *testing.T) {
	xmlStr := `<group xmlns="urn:schemas-microsoft-com:vml" id="g1" style="width:500pt;height:400pt" coordorigin="0,0" coordsize="5000,4000"/>`

	var g Group
	if err := xml.Unmarshal([]byte(xmlStr), &g); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&g)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var g2 Group
	if err := xml.Unmarshal(out, &g2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}

	if g2.ID != "g1" {
		t.Errorf("ID = %q, want %q", g2.ID, "g1")
	}
}

func TestImageEl_RoundTrip(t *testing.T) {
	img := &ImageEl{
		ID:    "img1",
		Style: "width:300pt;height:200pt",
		Src:   "photo.jpg",
	}
	out, err := xml.Marshal(img)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var img2 ImageEl
	if err := xml.Unmarshal(out, &img2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if img2.Src != "photo.jpg" {
		t.Errorf("Src = %q, want %q", img2.Src, "photo.jpg")
	}
}

func TestClientData_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		objectType string
	}{
		{"note", "Note"},
		{"checkbox", "Check"},
		{"button", "Button"},
		{"dropdown", "Drop"},
		{"spinner", "Spin"},
		{"listbox", "List"},
		{"radio", "Radio"},
		{"scroll", "Scroll"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := &ClientData{ObjectType: tt.objectType}
			out, err := xml.Marshal(cd)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cd2 ClientData
			if err := xml.Unmarshal(out, &cd2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cd2.ObjectType != tt.objectType {
				t.Errorf("ObjectType = %q, want %q", cd2.ObjectType, tt.objectType)
			}
		})
	}
}

func TestAnchorLock_RoundTrip(t *testing.T) {
	al := &AnchorLock{}
	out, err := xml.Marshal(al)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var al2 AnchorLock
	if err := xml.Unmarshal(out, &al2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestBorderTop_RoundTrip(t *testing.T) {
	bt := &BorderTop{Type: "single", Width: 4, Color: "#000000"}
	out, err := xml.Marshal(bt)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var bt2 BorderTop
	if err := xml.Unmarshal(out, &bt2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if bt2.Type != "single" {
		t.Errorf("Type = %q, want %q", bt2.Type, "single")
	}
}

// C148: mismapped VML attribute names — Callout minusx, Extrusion facet, and
// the o:signatureline attributes must map to the names in vml-officeDrawing.xsd.
func TestVMLAttributeMappings(t *testing.T) {
	var co Callout
	if err := xml.Unmarshal([]byte(`<callout minusx="t" minusy="f"/>`), &co); err != nil {
		t.Fatalf("unmarshal callout: %v", err)
	}
	if co.MinusX != "t" || co.MinusY != "f" {
		t.Errorf("callout minusx/minusy not captured: %+v", co)
	}

	var ex Extrusion
	if err := xml.Unmarshal([]byte(`<extrusion facet="30000"/>`), &ex); err != nil {
		t.Fatalf("unmarshal extrusion: %v", err)
	}
	if ex.Facet != "30000" {
		t.Errorf("extrusion facet not captured: %+v", ex)
	}

	var sl SignatureLine
	src := `<signatureline issignatureline="t" signinginstructionsset="t" allowcomments="f" showsigndate="t"/>`
	if err := xml.Unmarshal([]byte(src), &sl); err != nil {
		t.Fatalf("unmarshal signatureline: %v", err)
	}
	if sl.IsSignatureLine != "t" || sl.SigningInstructionsSet != "t" || sl.AllowComments != "f" || sl.ShowSignDate != "t" {
		t.Errorf("signatureline attributes not captured: %+v", sl)
	}
}
