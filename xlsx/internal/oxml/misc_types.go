package oxml

// CT_Comments represents the comments root element (ISO 29500 sml-comments).
type CT_Comments struct {
	Authors     *CT_Authors     `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main authors,omitempty"`
	CommentList *CT_CommentList `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main commentList,omitempty"`
}

// CT_Authors holds a list of comment authors.
type CT_Authors struct {
	Author []string `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main author,omitempty"`
}

// CT_CommentList holds a list of comments.
type CT_CommentList struct {
	Comment []*CT_Comment `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main comment,omitempty"`
}

// CT_Comment represents a single cell comment.
type CT_Comment struct {
	Ref      string  `xml:"ref,attr,omitempty"`
	AuthorId uint32  `xml:"authorId,attr,omitempty"`
	Guid     string  `xml:"guid,attr,omitempty"`
	Text     *CT_Rst `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main text,omitempty"`
}

// CT_CalcChain represents the calculation chain (ISO 29500 sml-calcChain).
type CT_CalcChain struct {
	C []*CT_CalcCell `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main c,omitempty"`
}

// CT_CalcCell represents a single cell in the calculation chain.
type CT_CalcCell struct {
	R string  `xml:"r,attr,omitempty"`
	I *uint32 `xml:"i,attr,omitempty"`
	S *bool   `xml:"s,attr,omitempty"`
	L *bool   `xml:"l,attr,omitempty"`
	T *bool   `xml:"t,attr,omitempty"`
	A *bool   `xml:"a,attr,omitempty"`
}

// CT_Metadata represents the metadata root element (ISO 29500 sml-metadata).
type CT_Metadata struct {
	MetadataTypes   *CT_MetadataTypes    `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main metadataTypes,omitempty"`
	MetadataStrings *CT_MetadataStrings  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main metadataStrings,omitempty"`
	MdxMetadata     *CT_MdxMetadata      `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main mdxMetadata,omitempty"`
	FutureMetadata  []*CT_FutureMetadata `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main futureMetadata,omitempty"`
	CellMetadata    *CT_MetadataBlocks   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main cellMetadata,omitempty"`
	ValueMetadata   *CT_MetadataBlocks   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main valueMetadata,omitempty"`
}

// CT_MetadataTypes holds a collection of metadata type definitions.
type CT_MetadataTypes struct {
	Count        *uint32            `xml:"count,attr,omitempty"`
	MetadataType []*CT_MetadataType `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main metadataType,omitempty"`
}

// CT_MetadataType defines a single metadata type and its capabilities.
type CT_MetadataType struct {
	Name                string  `xml:"name,attr,omitempty"`
	MinSupportedVersion *uint32 `xml:"minSupportedVersion,attr,omitempty"`
	GhostRow            *bool   `xml:"ghostRow,attr,omitempty"`
	GhostCol            *bool   `xml:"ghostCol,attr,omitempty"`
	Edit                *bool   `xml:"edit,attr,omitempty"`
	Delete              *bool   `xml:"delete,attr,omitempty"`
	Copy                *bool   `xml:"copy,attr,omitempty"`
	PasteAll            *bool   `xml:"pasteAll,attr,omitempty"`
	PasteFormulas       *bool   `xml:"pasteFormulas,attr,omitempty"`
	PasteValues         *bool   `xml:"pasteValues,attr,omitempty"`
	PasteFormats        *bool   `xml:"pasteFormats,attr,omitempty"`
	PasteComments       *bool   `xml:"pasteComments,attr,omitempty"`
	PasteDataValidation *bool   `xml:"pasteDataValidation,attr,omitempty"`
	PasteBorders        *bool   `xml:"pasteBorders,attr,omitempty"`
	PasteColWidths      *bool   `xml:"pasteColWidths,attr,omitempty"`
	PasteNumberFormats  *bool   `xml:"pasteNumberFormats,attr,omitempty"`
	Merge               *bool   `xml:"merge,attr,omitempty"`
	SplitFirst          *bool   `xml:"splitFirst,attr,omitempty"`
	SplitAll            *bool   `xml:"splitAll,attr,omitempty"`
	RowColShift         *bool   `xml:"rowColShift,attr,omitempty"`
	ClearAll            *bool   `xml:"clearAll,attr,omitempty"`
	ClearFormats        *bool   `xml:"clearFormats,attr,omitempty"`
	ClearContents       *bool   `xml:"clearContents,attr,omitempty"`
	ClearComments       *bool   `xml:"clearComments,attr,omitempty"`
	Assign              *bool   `xml:"assign,attr,omitempty"`
	Coerce              *bool   `xml:"coerce,attr,omitempty"`
	Adjust              *bool   `xml:"adjust,attr,omitempty"`
	CellMeta            *bool   `xml:"cellMeta,attr,omitempty"`
}

// CT_MetadataStrings holds a collection of metadata string values.
type CT_MetadataStrings struct {
	Count *uint32              `xml:"count,attr,omitempty"`
	S     []*CT_XStringElement `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main s,omitempty"`
}

// CT_XStringElement represents a string element with a value attribute.
type CT_XStringElement struct {
	V string `xml:"v,attr,omitempty"`
}

// CT_MdxMetadata holds a collection of MDX metadata entries.
type CT_MdxMetadata struct {
	Count *uint32    `xml:"count,attr,omitempty"`
	Mdx   []*CT_Mdx `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main mdx,omitempty"`
}

// CT_Mdx represents a single MDX metadata entry.
type CT_Mdx struct {
	N uint32         `xml:"n,attr,omitempty"`
	F string         `xml:"f,attr,omitempty"`
	T []*CT_MdxTuple `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main t,omitempty"`
}

// CT_MdxTuple represents an MDX tuple.
type CT_MdxTuple struct {
	C  *uint32                `xml:"c,attr,omitempty"`
	Ct string                 `xml:"ct,attr,omitempty"`
	Si *uint32                `xml:"si,attr,omitempty"`
	Fi *uint32                `xml:"fi,attr,omitempty"`
	Bc string                 `xml:"bc,attr,omitempty"`
	Fc string                 `xml:"fc,attr,omitempty"`
	I  *bool                  `xml:"i,attr,omitempty"`
	U  *bool                  `xml:"u,attr,omitempty"`
	St *bool                  `xml:"st,attr,omitempty"`
	B  *bool                  `xml:"b,attr,omitempty"`
	N  []*CT_MetadataStringIndex `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main n,omitempty"`
}

// CT_MetadataStringIndex represents a metadata string index reference.
type CT_MetadataStringIndex struct {
	X uint32 `xml:"x,attr,omitempty"`
	S *bool  `xml:"s,attr,omitempty"`
}

// CT_FutureMetadata holds future metadata extensions.
type CT_FutureMetadata struct {
	Name  string  `xml:"name,attr,omitempty"`
	Count *uint32 `xml:"count,attr,omitempty"`
}

// CT_MetadataBlocks holds a collection of metadata blocks.
type CT_MetadataBlocks struct {
	Count *uint32             `xml:"count,attr,omitempty"`
	Bk    []*CT_MetadataBlock `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main bk,omitempty"`
}

// CT_MetadataBlock represents a single metadata block containing records.
type CT_MetadataBlock struct {
	Rc []*CT_MetadataRecord `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main rc,omitempty"`
}

// CT_MetadataRecord represents a single metadata record.
type CT_MetadataRecord struct {
	T uint32 `xml:"t,attr,omitempty"`
	V uint32 `xml:"v,attr,omitempty"`
}

// CT_Order represents an OLAP member sort order.
type CT_Order struct {
	Val string `xml:"val,attr,omitempty"`
}
