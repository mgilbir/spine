package oxml

// CT_BookmarkStart represents a bookmark start marker.
type CT_BookmarkStart struct {
	Id   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Name string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr"`
	ColFirst string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main colFirst,attr,omitempty"`
	ColLast  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main colLast,attr,omitempty"`
	DisplacedByCustomXml string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main displacedByCustomXml,attr,omitempty"`
}

// CT_BookmarkEnd represents a bookmark end marker.
type CT_BookmarkEnd struct {
	Id string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	DisplacedByCustomXml string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main displacedByCustomXml,attr,omitempty"`
}

// CT_ProofErr represents a proofing error marker (w:proofErr).
type CT_ProofErr struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr"`
}

// CT_PermStart represents a permission range start.
type CT_PermStart struct {
	Id              string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	EdGrp           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main edGrp,attr,omitempty"`
	Ed              string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ed,attr,omitempty"`
	ColFirst        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main colFirst,attr,omitempty"`
	ColLast         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main colLast,attr,omitempty"`
	DisplacedByCustomXml string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main displacedByCustomXml,attr,omitempty"`
}

// CT_PermEnd represents a permission range end.
type CT_PermEnd struct {
	Id string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	DisplacedByCustomXml string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main displacedByCustomXml,attr,omitempty"`
}

// CT_CommentRangeStart represents a comment range start marker.
type CT_CommentRangeStart struct {
	Id string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
}

// CT_CommentRangeEnd represents a comment range end marker.
type CT_CommentRangeEnd struct {
	Id string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
}
