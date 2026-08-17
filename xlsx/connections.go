package xlsx

import (
	"bytes"
	xmlb "github.com/mgilbir/spine/common/xml"
	"sort"
	"strings"

	"github.com/mgilbir/spine/opc"
)

// relTypeConnections links the workbook to its external-data connections part
// (xl/connections.xml). Connections are read and preserved but not authored (see
// the deferral note on Connections), so only the relationship type is needed to
// resolve the part.
const relTypeConnections = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/connections"

// Connection describes one external-data connection declared in
// xl/connections.xml: the metadata Excel uses to refresh a query, a pivot
// cache, a data-model table, or a Power Query load. Spine reads and preserves
// connections but does not author or refresh them (authoring a live query, with
// its provider round-trip and credential handling, is out of scope); see the
// note on Workbook.Connections.
type Connection struct {
	// ID is the connection's numeric id attribute.
	ID uint32
	// Name is the connection name (often the query or table name).
	Name string
	// Description is the optional connection description.
	Description string
	// Type is the raw connection type code (ST_ConnectionType): 1=OLE DB,
	// 2=data feed, 4=web query, 5=text, 6=... . 0 when unset.
	Type uint32
	// ConnectionString is the provider connection string from <dbPr connection>,
	// or "" when the connection is not database-backed.
	ConnectionString string
	// Command is the query command (SQL, table name, or query text) from
	// <dbPr command>, or "" when none.
	Command string
	// WebURL is the source URL from <webPr url> for a web query, or "".
	WebURL string
	// SourceFile is the external source file from <textPr sourceFile> or
	// <connection sourceFile>, or "".
	SourceFile string
}

// Connections returns the external-data connections declared in
// xl/connections.xml, ordered by id. It is read-only: the connections part is
// preserved byte-for-byte on save. Returns nil when the workbook declares no
// connections.
//
// Deferred: authoring or refreshing a connection (writing a live query, driving
// its provider, handling credentials) is out of scope. Connections are surfaced
// for inspection and carried through unchanged.
func (w *Workbook) Connections() []Connection {
	data := w.connectionsPartData()
	if len(data) == 0 {
		return nil
	}

	var doc struct {
		Connection []struct {
			ID          uint32 `xml:"id,attr"`
			Name        string `xml:"name,attr"`
			Description string `xml:"description,attr"`
			Type        uint32 `xml:"type,attr"`
			SourceFile  string `xml:"sourceFile,attr"`
			DbPr        *struct {
				Connection string `xml:"connection,attr"`
				Command    string `xml:"command,attr"`
			} `xml:"dbPr"`
			WebPr *struct {
				URL string `xml:"url,attr"`
			} `xml:"webPr"`
			TextPr *struct {
				SourceFile string `xml:"sourceFile,attr"`
			} `xml:"textPr"`
		} `xml:"connection"`
	}
	if err := xmlb.Unmarshal(data, &doc); err != nil {
		return nil
	}

	out := make([]Connection, 0, len(doc.Connection))
	for _, c := range doc.Connection {
		conn := Connection{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Type:        c.Type,
			SourceFile:  c.SourceFile,
		}
		if c.DbPr != nil {
			conn.ConnectionString = c.DbPr.Connection
			conn.Command = c.DbPr.Command
		}
		if c.WebPr != nil {
			conn.WebURL = c.WebPr.URL
		}
		if c.TextPr != nil && c.TextPr.SourceFile != "" {
			conn.SourceFile = c.TextPr.SourceFile
		}
		out = append(out, conn)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// connectionsPartData returns the raw bytes of the workbook's connections part,
// resolved through the workbook relationship of type connections and falling
// back to the conventional /xl/connections.xml name. Returns nil when absent.
func (w *Workbook) connectionsPartData() []byte {
	for _, rel := range w.relationships[w.mainPart()] {
		if rel != nil && rel.Type == relTypeConnections {
			target := opc.ResolvePartName(w.mainPart(), rel.Target)
			if part, ok := w.preservedParts[target]; ok {
				return part.Data
			}
		}
	}
	if part, ok := w.preservedParts["/xl/connections.xml"]; ok {
		return part.Data
	}
	return nil
}

// DataModelInfo reports the presence and locations of a workbook's Power Pivot
// data model and Power Query (Get & Transform) content. Spine reads and
// preserves these parts byte-for-byte but does not author or refresh them; full
// data-model and Power Query authoring (the DataMashup blob, model tables and
// relationships) is out of scope.
type DataModelInfo struct {
	// HasDataModel reports whether the workbook carries a Power Pivot data model
	// (xl/model/ parts).
	HasDataModel bool
	// HasPowerQuery reports whether the workbook carries Power Query definitions
	// (a DataMashup blob in a customXml item).
	HasPowerQuery bool
	// ModelParts are the part names of the data model (xl/model/*), sorted.
	ModelParts []string
	// CustomXMLParts are the customXml item part names carrying Power Query /
	// data-model metadata (DataMashup), sorted.
	CustomXMLParts []string
}

// dataMashupMarker identifies a Power Query mashup blob embedded in a customXml
// item part.
var dataMashupMarker = []byte("DataMashup")

// DataModel reports the presence and locations of the workbook's Power Pivot
// data model and Power Query content. The underlying parts round-trip
// unchanged; this is inspection-only.
//
// Deferred: authoring or refreshing the data model or Power Query definitions
// (editing the DataMashup blob, model tables, or relationships) is out of scope.
func (w *Workbook) DataModel() DataModelInfo {
	var info DataModelInfo
	names := make([]string, 0, len(w.preservedParts))
	for name := range w.preservedParts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lower, "/xl/model/"):
			info.HasDataModel = true
			info.ModelParts = append(info.ModelParts, name)
		case strings.HasPrefix(lower, "/customxml/") && strings.HasSuffix(lower, ".xml"):
			if bytes.Contains(w.preservedParts[name].Data, dataMashupMarker) {
				info.HasPowerQuery = true
				info.CustomXMLParts = append(info.CustomXMLParts, name)
			}
		}
	}
	return info
}
