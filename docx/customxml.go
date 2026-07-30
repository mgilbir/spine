package docx

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
)

// Custom-XML data parts (customXml/itemN.xml) carry structured data stored
// alongside the document; content controls bind to nodes of these parts through
// w:sdtPr/w:dataBinding. These identifiers name the parts and relationships and
// are defined locally so the opc package stays free of WordprocessingML-specific
// constants.
const (
	relTypeCustomXML          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXml"
	relTypeCustomXMLProps     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/customXmlProps"
	contentTypeCustomXMLProps = "application/vnd.openxmlformats-officedocument.customXmlProperties+xml"
	contentTypeCustomXMLData  = "application/xml"
	nsDatastore               = "http://schemas.openxmlformats.org/officeDocument/2006/customXml"
)

// CustomXMLPart is a custom-XML data part of the document — a
// customXml/itemN.xml part holding structured XML that content controls can
// bind to. The zero value is not used; obtain instances from
// Document.CustomXMLParts or Document.AddCustomXMLPart.
type CustomXMLPart struct {
	partName   string
	propsName  string
	data       []byte
	itemID     string
	schemaRefs []string
}

// PartName returns the package part name of the data part (e.g.
// "/customXml/item1.xml").
func (c *CustomXMLPart) PartName() string { return c.partName }

// Data returns the raw XML bytes of the custom-XML data part.
func (c *CustomXMLPart) Data() []byte { return c.data }

// ItemID returns the datastore item id (storeItemID) declared by the part's
// itemProps, in the "{GUID}" form a content control's data binding references,
// or "" when the part has no properties.
func (c *CustomXMLPart) ItemID() string { return c.itemID }

// SchemaRefs returns the schema URIs the part's itemProps associates with the
// data (ds:schemaRef), or nil when none are declared.
func (c *CustomXMLPart) SchemaRefs() []string { return c.schemaRefs }

// CustomXMLParts returns the document's custom-XML data parts
// (customXml/itemN.xml) in part-name order, including parts added this session.
// Each part exposes its raw data, its datastore item id (the storeItemID a
// content control binds to), and any declared schema references.
func (d *Document) CustomXMLParts() []*CustomXMLPart {
	seen := make(map[string]bool)
	var out []*CustomXMLPart

	names := make([]string, 0, len(d.preservedParts))
	for name := range d.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !isCustomXMLDataPartName(name) {
			continue
		}
		part := d.preservedParts[name]
		out = append(out, d.newCustomXMLPartView(name, part.Data))
		seen[name] = true
	}

	for _, p := range d.pendingCustomXML {
		if seen[p.itemName] {
			continue
		}
		out = append(out, &CustomXMLPart{
			partName:   p.itemName,
			propsName:  p.propsName,
			data:       p.itemData,
			itemID:     p.itemID,
			schemaRefs: append([]string(nil), p.schemaRefs...),
		})
	}
	return out
}

// newCustomXMLPartView builds a read view of an opened custom-XML data part,
// resolving its itemProps through the part's relationships to recover the
// datastore item id and schema references.
func (d *Document) newCustomXMLPartView(name string, data []byte) *CustomXMLPart {
	c := &CustomXMLPart{partName: name, data: data}
	for _, rel := range d.relationships[name] {
		if rel.Type != relTypeCustomXMLProps {
			continue
		}
		propsName := opc.ResolvePartName(name, rel.Target)
		c.propsName = propsName
		if props, ok := d.preservedParts[propsName]; ok {
			c.itemID, c.schemaRefs = parseDatastoreItem(props.Data)
		}
		break
	}
	return c
}

// isCustomXMLDataPartName reports whether name is a customXml/itemN.xml data
// part (as opposed to its itemPropsN.xml companion).
func isCustomXMLDataPartName(name string) bool {
	const prefix = "/customXml/item"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".xml") {
		return false
	}
	mid := name[len(prefix) : len(name)-len(".xml")]
	if mid == "" {
		return false
	}
	_, err := strconv.Atoi(mid)
	return err == nil
}

// parseDatastoreItem extracts the ds:itemID and ds:schemaRef URIs from an
// itemProps part.
func parseDatastoreItem(data []byte) (itemID string, schemaRefs []string) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "datastoreItem":
			for _, a := range se.Attr {
				if a.Name.Local == "itemID" {
					itemID = a.Value
				}
			}
		case "schemaRef":
			for _, a := range se.Attr {
				if a.Name.Local == "uri" {
					schemaRefs = append(schemaRefs, a.Value)
				}
			}
		}
	}
	return itemID, schemaRefs
}

// pendingCustomXMLPart is a custom-XML data part added this session, written
// (with its itemProps and relationships) on save.
type pendingCustomXMLPart struct {
	itemName   string
	propsName  string
	itemData   []byte
	propsData  []byte
	itemID     string
	schemaRefs []string
	relID      string // document.xml relationship id
}

// AddCustomXMLPart adds a custom-XML data part carrying data, generating its
// itemProps (with a fresh datastore item id) and wiring the package
// relationships. The returned part's ItemID is the storeItemID to pass to
// ContentControl.SetDataBinding. The data must be a well-formed XML document;
// its root-element namespace, when present, is recorded as a schema reference.
func (d *Document) AddCustomXMLPart(data []byte) (*CustomXMLPart, error) {
	root, err := rootElementName(data)
	if err != nil {
		return nil, fmt.Errorf("docx: custom-XML data is not well-formed: %w", err)
	}
	var schemaRefs []string
	if root.Space != "" {
		schemaRefs = []string{root.Space}
	}

	n := d.nextCustomXMLNumber()
	itemName := fmt.Sprintf("/customXml/item%d.xml", n)
	propsName := fmt.Sprintf("/customXml/itemProps%d.xml", n)
	itemID := newGUID()

	p := &pendingCustomXMLPart{
		itemName:   itemName,
		propsName:  propsName,
		itemData:   append([]byte(nil), data...),
		propsData:  buildItemProps(itemID, schemaRefs),
		itemID:     itemID,
		schemaRefs: schemaRefs,
	}

	rel := &opc.Relationship{
		ID:     fmt.Sprintf("rId%d", d.nextRelID()),
		Type:   relTypeCustomXML,
		Target: fmt.Sprintf("../customXml/item%d.xml", n),
	}
	p.relID = rel.ID
	d.addDocRelationship(rel)
	d.pendingCustomXML = append(d.pendingCustomXML, p)
	// A new package part is a content change, and this one reaches no
	// flag-gated model, so it records itself (see modified.go).
	d.markEdited()

	return &CustomXMLPart{
		partName:   itemName,
		propsName:  propsName,
		data:       p.itemData,
		itemID:     itemID,
		schemaRefs: append([]string(nil), schemaRefs...),
	}, nil
}

// nextCustomXMLNumber returns the smallest positive N for which no
// /customXml/itemN.xml part exists among the opened parts or the parts added
// this session.
func (d *Document) nextCustomXMLNumber() int {
	used := make(map[int]bool)
	mark := func(name string) {
		if isCustomXMLDataPartName(name) {
			n, _ := strconv.Atoi(name[len("/customXml/item") : len(name)-len(".xml")])
			used[n] = true
		}
	}
	for name := range d.preservedParts {
		mark(name)
	}
	for _, p := range d.pendingCustomXML {
		mark(p.itemName)
	}
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// buildItemProps serializes a customXml/itemPropsN.xml part for the given
// datastore item id and schema references.
func buildItemProps(itemID string, schemaRefs []string) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>` + "\r\n")
	b.WriteString(`<ds:datastoreItem ds:itemID="`)
	b.WriteString(xmlb.EscapeAttrValue(itemID))
	b.WriteString(`" xmlns:ds="`)
	b.WriteString(nsDatastore)
	b.WriteString(`">`)
	b.WriteString(`<ds:schemaRefs>`)
	for _, uri := range schemaRefs {
		b.WriteString(`<ds:schemaRef ds:uri="`)
		b.WriteString(xmlb.EscapeAttrValue(uri))
		b.WriteString(`"/>`)
	}
	b.WriteString(`</ds:schemaRefs>`)
	b.WriteString(`</ds:datastoreItem>`)
	return []byte(b.String())
}

// rootElementName returns the qualified name of the first element in an XML
// document, erroring when the bytes are not well-formed enough to reach one.
func rootElementName(data []byte) (xml.Name, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.Name{}, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name, nil
		}
	}
}

// newGUID returns a random RFC 4122 version-4 GUID in the uppercase
// "{XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}" form Word uses for datastore item ids.
func newGUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("{%08X-%04X-%04X-%04X-%012X}",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// writePendingCustomXMLParts writes the custom-XML data parts added this
// session, each with its itemProps part and the item→itemProps relationship.
// The document.xml relationship was registered at AddCustomXMLPart time.
func (d *Document) writePendingCustomXMLParts(writer *opc.Writer) error {
	for _, p := range d.pendingCustomXML {
		if err := writer.WritePart(p.itemName, contentTypeCustomXMLData, p.itemData); err != nil {
			return err
		}
		if err := writer.WritePart(p.propsName, contentTypeCustomXMLProps, p.propsData); err != nil {
			return err
		}
		rels := []*opc.Relationship{{
			ID:     "rId1",
			Type:   relTypeCustomXMLProps,
			Target: strings.TrimPrefix(p.propsName, "/customXml/"),
		}}
		if err := writer.WritePartRelationships(p.itemName, rels); err != nil {
			return err
		}
	}
	return nil
}

// SetDataBinding binds the content control to a node of a custom-XML data part.
// xpath is an XPath expression selecting the node; storeItemID is the datastore
// item id of the target part (the "{GUID}" from CustomXMLPart.ItemID). An
// existing binding is replaced. Word keeps the control's displayed value in sync
// with the bound node.
func (c *ContentControl) SetDataBinding(xpath, storeItemID string) {
	c.ensureProps().SetDataBinding(xpath, storeItemID, "")
}

// SetDataBindingWithPrefixMappings is SetDataBinding with an explicit
// w:prefixMappings string declaring the namespace prefixes used by xpath (e.g.
// `xmlns:ns0='http://example.com/data'`).
func (c *ContentControl) SetDataBindingWithPrefixMappings(xpath, storeItemID, prefixMappings string) {
	c.ensureProps().SetDataBinding(xpath, storeItemID, prefixMappings)
}

// DataBinding returns the content control's data binding (xpath, storeItemID,
// prefixMappings) and whether one is present.
func (c *ContentControl) DataBinding() (xpath, storeItemID, prefixMappings string, ok bool) {
	return c.props().DataBinding()
}

// RemoveDataBinding removes the content control's data binding, reporting
// whether one was present.
func (c *ContentControl) RemoveDataBinding() bool {
	pr := c.props()
	if pr == nil {
		return false
	}
	c.touch()
	return pr.RemoveDataBinding()
}
