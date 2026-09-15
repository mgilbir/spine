package oxml

// cellType is the cell's t attribute held as one byte rather than a string.
//
// As a string it cost 16 bytes of header in every cell, for a value drawn from
// a closed schema set (ST_CellType) that is empty on three quarters of them.
// Counted over the corpus's 25,698,034 cells there are seven distinct values
// and no surprises:
//
//	""           75.242%
//	"s"          20.024%
//	"n"           2.462%
//	"str"         1.104%
//	"inlineStr"   1.082%
//	"e"           0.083%
//	"b"           0.003%
//
// "d" (ISO 8601 date) is the eighth schema value and appears nowhere in the
// corpus, but is included because it is legal.
//
// Anything outside that set is cellTypeOther, with the text kept verbatim in
// the cell's rare block so it round-trips. No corpus cell needs that.
type cellType uint8

const (
	cellTypeNone cellType = iota // no t attribute
	cellTypeB
	cellTypeD
	cellTypeE
	cellTypeInlineStr
	cellTypeN
	cellTypeS
	cellTypeStr
	cellTypeOther
)

// cellTypeFor maps the attribute text onto the enum. Anything unrecognised —
// including a value differing only in case, which is not the same attribute as
// far as the schema is concerned — becomes cellTypeOther and is kept verbatim.
func cellTypeFor(s string) cellType {
	switch s {
	case "":
		return cellTypeNone
	case "b":
		return cellTypeB
	case "d":
		return cellTypeD
	case "e":
		return cellTypeE
	case "inlineStr":
		return cellTypeInlineStr
	case "n":
		return cellTypeN
	case "s":
		return cellTypeS
	case "str":
		return cellTypeStr
	default:
		return cellTypeOther
	}
}

// String returns the attribute text for a known type. The returned strings are
// constants, so this allocates nothing. cellTypeOther yields "" — its text
// lives in the rare block and is read through CT_Cell.Type.
func (t cellType) String() string {
	switch t {
	case cellTypeB:
		return "b"
	case cellTypeD:
		return "d"
	case cellTypeE:
		return "e"
	case cellTypeInlineStr:
		return "inlineStr"
	case cellTypeN:
		return "n"
	case cellTypeS:
		return "s"
	case cellTypeStr:
		return "str"
	default:
		return ""
	}
}
