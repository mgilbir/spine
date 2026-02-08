package oxml

// CT_Connection represents an external data connection.
type CT_Connection struct {
	Id              uint32  `xml:"id,attr"`
	SourceFile      string  `xml:"sourceFile,attr,omitempty"`
	OdcFile         string  `xml:"odcFile,attr,omitempty"`
	KeepAlive       *bool   `xml:"keepAlive,attr,omitempty"`
	Interval        *uint32 `xml:"interval,attr,omitempty"`
	Name            string  `xml:"name,attr,omitempty"`
	Description     string  `xml:"description,attr,omitempty"`
	Type            *uint32 `xml:"type,attr,omitempty"`
	ReconnectionMethod *uint32 `xml:"reconnectionMethod,attr,omitempty"`
	RefreshedVersion *uint32 `xml:"refreshedVersion,attr,omitempty"`
	MinRefreshableVersion *uint32 `xml:"minRefreshableVersion,attr,omitempty"`
	SavePassword    *bool   `xml:"savePassword,attr,omitempty"`
	New             *bool   `xml:"new,attr,omitempty"`
	Deleted         *bool   `xml:"deleted,attr,omitempty"`
	OnlyUseConnectionFile *bool `xml:"onlyUseConnectionFile,attr,omitempty"`
	Background      *bool   `xml:"background,attr,omitempty"`
	RefreshOnLoad   *bool   `xml:"refreshOnLoad,attr,omitempty"`
	SaveData        *bool   `xml:"saveData,attr,omitempty"`
	Credentials     string  `xml:"credentials,attr,omitempty"`
	SingleSignOnId  string  `xml:"singleSignOnId,attr,omitempty"`
}

// CT_QueryTable represents a query table definition.
type CT_QueryTable struct {
	Name                  string  `xml:"name,attr,omitempty"`
	Headers               *bool   `xml:"headers,attr,omitempty"`
	RowNumbers            *bool   `xml:"rowNumbers,attr,omitempty"`
	DisableRefresh        *bool   `xml:"disableRefresh,attr,omitempty"`
	BackgroundRefresh     *bool   `xml:"backgroundRefresh,attr,omitempty"`
	FirstBackgroundRefresh *bool  `xml:"firstBackgroundRefresh,attr,omitempty"`
	RefreshOnLoad         *bool   `xml:"refreshOnLoad,attr,omitempty"`
	GrowShrinkType        string  `xml:"growShrinkType,attr,omitempty"`
	FillFormulas          *bool   `xml:"fillFormulas,attr,omitempty"`
	RemoveDataOnSave      *bool   `xml:"removeDataOnSave,attr,omitempty"`
	DisableEdit           *bool   `xml:"disableEdit,attr,omitempty"`
	PreserveFormatting    *bool   `xml:"preserveFormatting,attr,omitempty"`
	AdjustColumnWidth     *bool   `xml:"adjustColumnWidth,attr,omitempty"`
	Intermediate          *bool   `xml:"intermediate,attr,omitempty"`
	ConnectionId          uint32  `xml:"connectionId,attr"`
	AutoFormatId          *uint32 `xml:"autoFormatId,attr,omitempty"`
	ApplyNumberFormats    *bool   `xml:"applyNumberFormats,attr,omitempty"`
	ApplyBorderFormats    *bool   `xml:"applyBorderFormats,attr,omitempty"`
	ApplyFontFormats      *bool   `xml:"applyFontFormats,attr,omitempty"`
	ApplyPatternFormats   *bool   `xml:"applyPatternFormats,attr,omitempty"`
	ApplyAlignmentFormats *bool   `xml:"applyAlignmentFormats,attr,omitempty"`
	ApplyWidthHeightFormats *bool `xml:"applyWidthHeightFormats,attr,omitempty"`

	QueryTableDeletedFields *CT_QueryTableDeletedFields `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main queryTableDeletedFields,omitempty"`
}

// CT_QueryTableDeletedFields represents deleted fields in a query table.
type CT_QueryTableDeletedFields struct {
	Count          *uint32                    `xml:"count,attr,omitempty"`
	DeletedField   []*CT_DeletedField         `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main deletedField,omitempty"`
}

// CT_DeletedField represents a deleted field.
type CT_DeletedField struct {
	Name string `xml:"name,attr,omitempty"`
}

// CT_DdeLink represents a DDE link.
type CT_DdeLink struct {
	DdeService string         `xml:"ddeService,attr,omitempty"`
	DdeTopic   string         `xml:"ddeTopic,attr,omitempty"`
	DdeItems   *CT_DdeItems   `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main ddeItems,omitempty"`
}

// CT_DdeItems represents DDE items.
type CT_DdeItems struct {
	DdeItem []*CT_DdeItem `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main ddeItem,omitempty"`
}

// CT_DdeItem represents a DDE item.
type CT_DdeItem struct {
	Name    string `xml:"name,attr,omitempty"`
	Ole     *bool  `xml:"ole,attr,omitempty"`
	Advise  *bool  `xml:"advise,attr,omitempty"`
	PreferPic *bool `xml:"preferPic,attr,omitempty"`
}

// CT_OleLink represents an OLE link.
type CT_OleLink struct {
	RID     string       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ProgId  string       `xml:"progId,attr,omitempty"`
}

// CT_ExternalLink represents an external link.
type CT_ExternalLink struct {
	ExternalBook *CT_ExternalBook `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main externalBook,omitempty"`
	DdeLink      *CT_DdeLink      `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main ddeLink,omitempty"`
	OleLink      *CT_OleLink      `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main oleLink,omitempty"`
}

// CT_ExternalBook represents an external book reference.
type CT_ExternalBook struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// CT_VolTypes represents volatile dependency types.
type CT_VolTypes struct {
	VolType []*CT_VolType `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main volType,omitempty"`
}

// CT_VolType represents a volatile type.
type CT_VolType struct {
	Type string        `xml:"type,attr,omitempty"`
	Main []*CT_VolMain `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main main,omitempty"`
}

// CT_VolMain represents a volatile main entry.
type CT_VolMain struct {
	First string       `xml:"first,attr,omitempty"`
	Tp    []*CT_VolTopic `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main tp,omitempty"`
}

// CT_VolTopic represents a volatile topic.
type CT_VolTopic struct {
	T  string       `xml:"t,attr,omitempty"`
	V  string       `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main v,omitempty"`
	Tr []*CT_VolTr  `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main tr,omitempty"`
}

// CT_VolTr represents a volatile topic reference.
type CT_VolTr struct {
	R string `xml:"r,attr,omitempty"`
	S *uint32 `xml:"s,attr,omitempty"`
}

// CT_ExternalCell represents an external cell data (section 18.14.1).
type CT_ExternalCell struct {
	R  string `xml:"r,attr,omitempty"`
	T  string `xml:"t,attr,omitempty"`
	Vm *uint32 `xml:"vm,attr,omitempty"`
	V  string `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main v,omitempty"`
}
