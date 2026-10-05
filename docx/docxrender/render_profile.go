package docxrender

import (
	"encoding/xml"
	"fmt"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

const nsW = xmlb.NSWordprocessingML

func wordRenderXML(n core.XMLNode) error {
	if len(n.Path) == 1 && (n.Name.Space != nsW || n.Name.Local != "document") {
		return fmt.Errorf("%w: document XML root", render.ErrInvalid)
	}
	return wordRenderNode(n)
}

// Unreferenced non-default named styles do not participate in this profile.
// Default styles and document defaults are checked both here and in the model;
// paragraph/run style references and default-style inheritance are rejected.
func wordRenderStyleProfile() func(core.XMLNode) error {
	unused := false
	return func(n core.XMLNode) error {
		if len(n.Path) == 1 && (n.Name.Space != nsW || n.Name.Local != "styles") {
			return fmt.Errorf("%w: styles XML root", render.ErrInvalid)
		}
		if len(n.Path) > 1 && n.Path[1] == (xml.Name{Space: nsW, Local: "latentStyles"}) {
			if len(n.Path) == 2 && n.Occurrence > 1 {
				return render.ErrInvalid
			}
			return nil
		}
		if len(n.Path) == 2 && n.Name.Space == nsW && n.Name.Local == "style" {
			unused = true
			for _, a := range n.Attr {
				if a.Name.Space == nsW && a.Name.Local == "styleId" && (a.Value == "Normal" || a.Value == "DefaultParagraphFont") {
					unused = false
				}
				if a.Name.Space == nsW && a.Name.Local == "default" {
					switch a.Value {
					case "1", "true", "on":
						unused = false
					case "0", "false", "off":
					default:
						return fmt.Errorf("%w: default style flag", render.ErrInvalid)
					}
				}
			}
		}
		if len(n.Path) > 2 && n.Path[1] == (xml.Name{Space: nsW, Local: "style"}) && unused {
			return nil
		}
		return wordRenderNode(n)
	}
}

func wordRenderNode(n core.XMLNode) error {
	if n.Text {
		if len(n.Path) > 0 && n.Path[len(n.Path)-1] == (xml.Name{Space: nsW, Local: "t"}) {
			return nil
		}
		return fmt.Errorf("%w: unexpected XML text", render.ErrUnsupported)
	}
	if n.Name.Space != nsW {
		return fmt.Errorf("%w: XML namespace", render.ErrUnsupported)
	}
	spec, ok := wordRenderElements[n.Name.Local]
	if !ok {
		return fmt.Errorf("%w: XML %s", render.ErrUnsupported, n.Name.Local)
	}
	if len(n.Path) > 1 {
		parent := n.Path[len(n.Path)-2]
		if parent.Space != nsW || !strings.Contains(" "+spec.parents+" ", " "+parent.Local+" ") {
			return fmt.Errorf("%w: XML placement", render.ErrUnsupported)
		}
	}
	if n.Occurrence > 1 && !spec.repeat {
		return fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, n.Name.Local)
	}
	for _, a := range n.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if n.Name.Local == "t" && a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "space" && (a.Value == "preserve" || a.Value == "default") {
			continue
		}
		if a.Name.Space != nsW || !strings.Contains(" "+spec.attrs+" ", " "+a.Name.Local+" ") {
			return fmt.Errorf("%w: XML attribute %s", render.ErrUnsupported, a.Name.Local)
		}
		if a.Name.Local == "val" && strings.Contains(" b i strike keepNext keepLines pageBreakBefore widowControl qFormat ", " "+n.Name.Local+" ") {
			switch a.Value {
			case "1", "true", "on", "0", "false", "off":
			default:
				return fmt.Errorf("%w: boolean XML", render.ErrInvalid)
			}
		}
	}
	return nil
}

var wordRenderElements = map[string]struct {
	parents, attrs string
	repeat         bool
}{
	"document": {"", "conformance", false}, "body": {"document", "", false}, "p": {"body", "rsidR rsidRPr rsidRDefault rsidP rsidDel", true}, "pPr": {"p pPrDefault style", "", false}, "r": {"p", "rsidR rsidRPr rsidDel", true}, "rPr": {"r rPrDefault style", "", false}, "t": {"r", "", true},
	"rFonts": {"rPr", "ascii hAnsi eastAsia cs", false}, "sz": {"rPr", "val", false}, "szCs": {"rPr", "val", false}, "b": {"rPr", "val", false}, "i": {"rPr", "val", false}, "strike": {"rPr", "val", false}, "u": {"rPr", "val", false}, "color": {"rPr", "val", false},
	"spacing": {"pPr", "before after line lineRule", false}, "jc": {"pPr", "val", false}, "keepNext": {"pPr", "val", false}, "keepLines": {"pPr", "val", false}, "pageBreakBefore": {"pPr", "val", false}, "widowControl": {"pPr", "val", false},
	"sectPr": {"body", "rsidR rsidRPr rsidSect", false}, "pgSz": {"sectPr", "w h orient", false}, "pgMar": {"sectPr", "top right bottom left header footer gutter", false},
	"styles": {}, "docDefaults": {"styles", "", false}, "rPrDefault": {"docDefaults", "", false}, "pPrDefault": {"docDefaults", "", false}, "style": {"styles", "type styleId default customStyle", true}, "name": {"style", "val", false}, "qFormat": {"style", "val", false},
}
