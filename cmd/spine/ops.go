package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// opsVersion is the only operations document version this build accepts.
const opsVersion = 1

// fieldSpec describes one operation field. The same definition drives strict
// parsing and the `spine ops-schema` output, so the two cannot drift.
type fieldSpec struct {
	Name        string
	Type        string
	Required    bool
	Enum        []string
	Description string
}

// opDef is one operation: its name, the file kinds it applies to, its fields,
// and the function that applies it to an open document.
type opDef struct {
	Name      string
	AppliesTo []string
	Fields    []fieldSpec
	Run       func(d *document, fields map[string]json.RawMessage) (any, error)
}

// opRegistry is the whole vocabulary, in schema order. `spine edit` dispatches
// over it and `spine ops-schema` prints it; there is no second list.
var opRegistry = []opDef{
	{
		Name:      "set_cell",
		AppliesTo: []string{"xlsx"},
		Fields: []fieldSpec{
			{Name: "selector", Type: "string", Required: true, Description: "xlsx cell selector, e.g. Sheet1!B2"},
			{Name: "value", Type: "string|number|boolean", Required: true, Description: "the new cell value; null is not allowed, use clear_cell to empty a cell"},
			{Name: "type", Type: "string", Enum: []string{"string", "number", "boolean", "formula", "date"}, Description: "force the interpretation; by default a JSON number is a number, a boolean a boolean, and a string follows the same inference as `spine set`"},
		},
		Run: opSetCell,
	},
	{
		Name:      "clear_cell",
		AppliesTo: []string{"xlsx"},
		Fields: []fieldSpec{
			{Name: "selector", Type: "string", Required: true, Description: "xlsx cell selector, e.g. Sheet1!B2; the cell becomes empty (value and formula removed, style kept)"},
		},
		Run: opClearCell,
	},
	{
		Name:      "set_text",
		AppliesTo: []string{"pptx"},
		Fields: []fieldSpec{
			{Name: "selector", Type: "string", Required: true, Description: "pptx text target: slide/N/title, slide/N/body or slide/N/shape/ID"},
			{Name: "text", Type: "string", Required: true, Description: "the new text; \\n separates paragraphs"},
		},
		Run: opSetText,
	},
	{
		Name:      "add_sheet",
		AppliesTo: []string{"xlsx"},
		Fields: []fieldSpec{
			{Name: "name", Type: "string", Description: "name for the new sheet; omitted, the first unused Sheet<N> is chosen. Illegal or already-taken names are rejected, never renamed"},
		},
		Run: opAddSheet,
	},
	{
		Name:      "remove_sheet",
		AppliesTo: []string{"xlsx"},
		Fields: []fieldSpec{
			{Name: "name", Type: "string", Required: true, Description: "DESTRUCTIVE: removes the sheet; formulas referencing it become #REF! permanently"},
		},
		Run: opRemoveSheet,
	},
	{
		Name:      "add_slide",
		AppliesTo: []string{"pptx"},
		Fields: []fieldSpec{
			{Name: "layout", Type: "string", Enum: layoutEnumNames, Description: "add the slide from the master's layout of that type, so it inherits the layout's placeholders; omitted, the slide has none"},
		},
		Run: opAddSlide,
	},
	{
		Name:      "remove_slide",
		AppliesTo: []string{"pptx"},
		Fields: []fieldSpec{
			{Name: "index", Type: "integer", Required: true, Description: "DESTRUCTIVE: removes the 1-based slide"},
		},
		Run: opRemoveSlide,
	},
	{
		Name:      "set_properties",
		AppliesTo: []string{"xlsx", "pptx"},
		Fields: []fieldSpec{
			{Name: "title", Type: "string", Description: "document title; an empty string clears it"},
			{Name: "subject", Type: "string", Description: "document subject; an empty string clears it"},
			{Name: "creator", Type: "string", Description: "document creator; an empty string clears it"},
			{Name: "keywords", Type: "string", Description: "document keywords; an empty string clears it"},
		},
		Run: opSetProperties,
	},
}

// lookupOp returns the opDef for name, or nil when unknown.
func lookupOp(name string) *opDef {
	for i := range opRegistry {
		if opRegistry[i].Name == name {
			return &opRegistry[i]
		}
	}
	return nil
}

// field returns the named field spec of the operation, or nil.
func (op *opDef) field(name string) *fieldSpec {
	for i := range op.Fields {
		if op.Fields[i].Name == name {
			return &op.Fields[i]
		}
	}
	return nil
}

// unknownFieldsOf returns the operation's field names that are not in its
// schema, sorted so the reported one is deterministic (map iteration is not).
func (op *opDef) unknownFieldsOf(fields map[string]json.RawMessage) []string {
	var unknown []string
	for k := range fields {
		if k == "op" {
			continue
		}
		if op.field(k) == nil {
			unknown = append(unknown, k)
		}
	}
	sort.Strings(unknown)
	return unknown
}

// ---- op dispatch ----

// opFailure is one failing operation inside a batch, carrying its 0-based
// index and name for the edit error envelope.
type opFailure struct {
	err   *cliError
	index int
	op    string
}

func (f *opFailure) Error() string { return f.err.message }

// applyOps parses and executes each operation in order; a later op sees the
// effects of earlier ones because they mutate the same open document. The
// first failure aborts the batch (nothing has been saved yet).
func applyOps(d *document, ops []json.RawMessage) error {
	for i, raw := range ops {
		if err := applyOneOp(d, i, raw); err != nil {
			return err
		}
	}
	return nil
}

func applyOneOp(d *document, i int, raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return &opFailure{errf("invalid_value", "operation %d is not a JSON object", i), i, ""}
	}
	nameRaw, ok := fields["op"]
	if !ok {
		return &opFailure{errf("invalid_value", `operation %d has no "op" field`, i), i, ""}
	}
	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil {
		return &opFailure{errf("invalid_value", `operation %d: "op" must be a string`, i), i, ""}
	}
	def := lookupOp(name)
	if def == nil {
		return &opFailure{errf("invalid_value", "unknown operation %q; run `spine ops-schema` for the list", name), i, name}
	}
	applies := false
	for _, k := range def.AppliesTo {
		if k == d.kind {
			applies = true
		}
	}
	if !applies {
		return &opFailure{errf("unsupported", "operation %q applies only to %s files, not %s", name, strings.Join(def.AppliesTo, " and "), d.kind), i, name}
	}
	if unknown := def.unknownFieldsOf(fields); len(unknown) > 0 {
		return &opFailure{errf("invalid_value", `operation %q does not accept field %q`, name, unknown[0]), i, name}
	}
	for _, f := range def.Fields {
		if f.Required {
			if _, ok := fields[f.Name]; !ok {
				return &opFailure{errf("invalid_value", `operation %q is missing required field %q`, name, f.Name), i, name}
			}
		}
	}
	if _, err := def.Run(d, fields); err != nil {
		var ce *cliError
		if asCLIError(err, &ce) {
			return &opFailure{ce, i, name}
		}
		return &opFailure{&cliError{code: "internal", message: err.Error()}, i, name}
	}
	return nil
}

// ---- field decoding helpers ----

// fieldString decodes a required string field.
func fieldString(fields map[string]json.RawMessage, name string) (string, error) {
	var s string
	if err := json.Unmarshal(fields[name], &s); err != nil {
		return "", errf("invalid_value", "field %q must be a string", name)
	}
	return s, nil
}

// fieldOptString decodes an optional string field, reporting presence.
func fieldOptString(fields map[string]json.RawMessage, name string) (string, bool, error) {
	raw, ok := fields[name]
	if !ok {
		return "", false, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", true, errf("invalid_value", "field %q must be a string", name)
	}
	return s, true, nil
}

// fieldInt decodes a required integer field (a JSON number that is whole).
func fieldInt(fields map[string]json.RawMessage, name string) (int, error) {
	var f float64
	if err := json.Unmarshal(fields[name], &f); err != nil || f != float64(int64(f)) {
		return 0, errf("invalid_value", "field %q must be an integer", name)
	}
	return int(f), nil
}

// fieldEnum validates an optional enum string field and returns it.
func fieldEnum(fields map[string]json.RawMessage, name string, allowed []string) (string, error) {
	s, err := fieldString(fields, name)
	if err != nil {
		return "", err
	}
	for _, a := range allowed {
		if a == s {
			return s, nil
		}
	}
	return "", errf("invalid_value", "field %q must be one of %s (got %q)", name, strings.Join(allowed, ", "), s)
}

// decodeCellValue decodes set_cell's value: a JSON string, number or boolean.
// null is invalid (clearing a cell is clear_cell).
func decodeCellValue(raw json.RawMessage) (any, error) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return nil, errf("invalid_value", `field "value" must not be null; use clear_cell to empty a cell`)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, errf("invalid_value", `field "value" must be a string, number or boolean`)
	}
	switch v.(type) {
	case string, float64, bool:
		return v, nil
	default:
		return nil, errf("invalid_value", `field "value" must be a string, number or boolean`)
	}
}

// ---- operations ----

func opSetCell(d *document, fields map[string]json.RawMessage) (any, error) {
	selector, err := fieldString(fields, "selector")
	if err != nil {
		return nil, err
	}
	value, err := decodeCellValue(fields["value"])
	if err != nil {
		return nil, err
	}
	typ := ""
	if _, ok := fields["type"]; ok {
		typ, err = fieldEnum(fields, "type", []string{"string", "number", "boolean", "formula", "date"})
		if err != nil {
			return nil, err
		}
	}
	sheetName, ref, selectsNothing, err := parseCellSelector(selector)
	if err != nil {
		return nil, err
	}
	if selectsNothing {
		return nil, errf("not_found", "sheet not found: %q", sheetName)
	}
	row, col, perr := xlsx.ParseCellRef(ref)
	if perr != nil {
		return nil, errf("invalid_range", "invalid cell reference %q: %v", ref, perr)
	}
	sheet, err := resolveSheet(d.wb, sheetName)
	if err != nil {
		return nil, err
	}
	if err := checkMerged(sheet, ref, row, col); err != nil {
		return nil, err
	}
	typed, err := coerceOpValue(value, typ)
	if err != nil {
		return nil, err
	}
	cell, cerr := sheet.Cell(ref)
	if cerr != nil {
		return nil, errf("io", "cannot address %s: %v", ref, cerr)
	}
	if err := storeCell(cell, ref, typed); err != nil {
		return nil, err
	}
	return setXLSXOutput{Selector: selector, Type: typ, Value: cellValue(cell)}, nil
}

// coerceOpValue interprets a set_cell value. JSON typing is authoritative
// (number→number, boolean→boolean); a string follows the same inference rules
// as `spine set`. The optional type overrides.
func coerceOpValue(value any, typ string) (any, error) {
	if typ == "" {
		switch v := value.(type) {
		case float64:
			return v, nil
		case bool:
			return v, nil
		case string:
			_, typed, err := coerceValue(v, "")
			return typed, err
		}
	}
	switch v := value.(type) {
	case string:
		_, typed, err := coerceValue(v, typ)
		return typed, err
	case float64:
		switch typ {
		case "number":
			return v, nil
		case "string":
			return strconv.FormatFloat(v, 'f', -1, 64), nil
		default:
			return nil, errf("invalid_value", `field "value" must be a string for type %q`, typ)
		}
	case bool:
		switch typ {
		case "boolean":
			return v, nil
		case "string":
			if v {
				return "true", nil
			}
			return "false", nil
		default:
			return nil, errf("invalid_value", `field "value" must be a string for type %q`, typ)
		}
	}
	return nil, errf("internal", "unhandled value type")
}

func opClearCell(d *document, fields map[string]json.RawMessage) (any, error) {
	selector, err := fieldString(fields, "selector")
	if err != nil {
		return nil, err
	}
	sheetName, ref, selectsNothing, err := parseCellSelector(selector)
	if err != nil {
		return nil, err
	}
	if selectsNothing {
		return nil, errf("not_found", "sheet not found: %q", sheetName)
	}
	row, col, perr := xlsx.ParseCellRef(ref)
	if perr != nil {
		return nil, errf("invalid_range", "invalid cell reference %q: %v", ref, perr)
	}
	sheet, err := resolveSheet(d.wb, sheetName)
	if err != nil {
		return nil, err
	}
	if err := checkMerged(sheet, ref, row, col); err != nil {
		return nil, err
	}
	if cell := sheet.FindCell(ref); cell != nil {
		cell.Clear()
	}
	return clearCellOutput{Selector: selector}, nil
}

func opSetText(d *document, fields map[string]json.RawMessage) (any, error) {
	selector, err := fieldString(fields, "selector")
	if err != nil {
		return nil, err
	}
	text, err := fieldString(fields, "text")
	if err != nil {
		return nil, err
	}
	sel, shape, err := resolveTextTarget(d.pres, selector)
	if err != nil {
		return nil, err
	}
	tf := textFrameOf(shape)
	if tf == nil {
		return nil, errf("unsupported", "shape %d on slide %d does not hold text", shape.ID(), sel.slide)
	}
	tf.SetText(text)
	return setPPTXOutput{Selector: selector, ShapeID: shape.ID(), Text: textOf(shape)}, nil
}

func opAddSheet(d *document, fields map[string]json.RawMessage) (any, error) {
	name, present, err := fieldOptString(fields, "name")
	if err != nil {
		return nil, err
	}
	if !present {
		name = firstFreeSheetName(d.wb)
	} else {
		if verr := xlsx.ValidateSheetName(name); verr != nil {
			return nil, errf("invalid_value", "invalid sheet name: %v", verr)
		}
		if sheetNameTaken(d.wb, name) {
			return nil, errf("invalid_value", "sheet name %q is already taken", name)
		}
	}
	sheet, aerr := d.wb.AddSheet(name)
	if aerr != nil {
		return nil, errf("invalid_value", "cannot add sheet %q: %v", name, aerr)
	}
	return addSheetOutput{Index: sheet.Index(), Name: sheet.Name()}, nil
}

// firstFreeSheetName returns the first unused Sheet<N> (Excel's rule, checked
// case-insensitively, from the smallest free N).
func firstFreeSheetName(wb *xlsx.Workbook) string {
	for n := 1; ; n++ {
		cand := fmt.Sprintf("Sheet%d", n)
		if !sheetNameTaken(wb, cand) {
			return cand
		}
	}
}

// sheetNameTaken reports whether a sheet named name already exists,
// case-insensitively.
func sheetNameTaken(wb *xlsx.Workbook, name string) bool {
	for _, s := range wb.Sheets() {
		if strings.EqualFold(s.Name(), name) {
			return true
		}
	}
	return false
}

func opRemoveSheet(d *document, fields map[string]json.RawMessage) (any, error) {
	name, err := fieldString(fields, "name")
	if err != nil {
		return nil, err
	}
	sheet, err := resolveSheet(d.wb, name)
	if err != nil {
		return nil, err
	}
	if d.wb.SheetCount() == 1 {
		return nil, errf("invalid_value", "cannot remove the last remaining sheet")
	}
	if err := d.wb.RemoveSheet(sheet.Index()); err != nil {
		return nil, errf("internal", "cannot remove sheet %q: %v", name, err)
	}
	return removeSheetOutput{Name: name}, nil
}

func opAddSlide(d *document, fields map[string]json.RawMessage) (any, error) {
	layoutName, present, err := fieldOptString(fields, "layout")
	if err != nil {
		return nil, err
	}
	out := addSlideOutput{}
	if !present {
		d.pres.AddSlide()
	} else {
		lt, ok := layoutByFriendlyName[layoutName]
		if !ok {
			return nil, errf("invalid_value", "field %q must be one of %s (got %q)", "layout", strings.Join(layoutEnumNames, ", "), layoutName)
		}
		layout, lerr := d.pres.LayoutByType(lt)
		if lerr != nil {
			return nil, errf("not_found", "deck has no slide layout of type %q", lt)
		}
		d.pres.AddSlideFromLayout(layout)
		used := string(lt)
		out.Layout = &used
	}
	out.Index = d.pres.SlideCount()
	return out, nil
}

func opRemoveSlide(d *document, fields map[string]json.RawMessage) (any, error) {
	index, err := fieldInt(fields, "index")
	if err != nil {
		return nil, err
	}
	if _, err := resolveSlide(d.pres, index); err != nil {
		return nil, err
	}
	if d.pres.SlideCount() == 1 {
		return nil, errf("invalid_value", "cannot remove the last remaining slide")
	}
	if err := d.pres.RemoveSlide(index - 1); err != nil {
		return nil, errf("internal", "cannot remove slide %d: %v", index, err)
	}
	return removeSlideOutput{Index: index}, nil
}

// layoutEnumNames are the add_slide "layout" values, in enum order.
var layoutEnumNames = []string{
	"title", "title_and_content", "section_header", "two_content", "comparison",
	"title_only", "blank", "content_with_caption", "picture_with_caption",
	"title_and_vertical_text", "vertical_title_and_text",
}

// layoutByFriendlyName maps add_slide's "layout" enum to the library's
// SlideLayoutType constants.
var layoutByFriendlyName = map[string]pptx.SlideLayoutType{
	"title":                   pptx.LayoutTitle,
	"title_and_content":       pptx.LayoutTitleAndContent,
	"section_header":          pptx.LayoutSectionHeader,
	"two_content":             pptx.LayoutTwoContent,
	"comparison":              pptx.LayoutComparison,
	"title_only":              pptx.LayoutTitleOnly,
	"blank":                   pptx.LayoutBlank,
	"content_with_caption":    pptx.LayoutContentWithCaption,
	"picture_with_caption":    pptx.LayoutPictureWithCaption,
	"title_and_vertical_text": pptx.LayoutTitleAndVerticalText,
	"vertical_title_and_text": pptx.LayoutVerticalTitleAndText,
}

func opSetProperties(d *document, fields map[string]json.RawMessage) (any, error) {
	type prop struct {
		name  string
		apply func(v string)
	}
	props := []prop{
		{"title", func(v string) { setPropTitle(d, v) }},
		{"subject", func(v string) { setPropSubject(d, v) }},
		{"creator", func(v string) { setPropCreator(d, v) }},
		{"keywords", func(v string) { setPropKeywords(d, v) }},
	}
	out := setPropertiesOutput{}
	for _, p := range props {
		v, present, err := fieldOptString(fields, p.name)
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		p.apply(v)
		switch p.name {
		case "title":
			out.Title = &v
		case "subject":
			out.Subject = &v
		case "creator":
			out.Creator = &v
		case "keywords":
			out.Keywords = &v
		}
	}
	return out, nil
}

func setPropTitle(d *document, v string) {
	if d.kind == "xlsx" {
		d.wb.Properties.Title = v
	} else {
		d.pres.Properties.Title = v
	}
}

func setPropSubject(d *document, v string) {
	if d.kind == "xlsx" {
		d.wb.Properties.Subject = v
	} else {
		d.pres.Properties.Subject = v
	}
}

func setPropCreator(d *document, v string) {
	if d.kind == "xlsx" {
		d.wb.Properties.Creator = v
	} else {
		d.pres.Properties.Creator = v
	}
}

func setPropKeywords(d *document, v string) {
	if d.kind == "xlsx" {
		d.wb.Properties.Keywords = v
	} else {
		d.pres.Properties.Keywords = v
	}
}

// ---- schema generation ----

// schemaField is the JSON description of one field in ops-schema output.
type schemaField struct {
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Enum        []string `json:"enum,omitempty"`
	Description string   `json:"description,omitempty"`
}

// schemaOp is the JSON description of one operation.
type schemaOp struct {
	Op        string    `json:"op"`
	AppliesTo []string  `json:"applies_to"`
	Fields    fieldList `json:"fields"`
}

// schemaDocument is the ops-schema envelope.
type schemaDocument struct {
	Version    int        `json:"version"`
	Operations []schemaOp `json:"operations"`
}

// namedField pairs a field name with its spec so the fields object can be
// emitted in declaration order (deterministic, never a map).
type namedField struct {
	name string
	spec fieldSpec
}

// fieldList marshals as a JSON object keyed by field name, in declaration
// order.
type fieldList []namedField

func (fl fieldList) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range fl {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(f.name)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		spec, err := json.Marshal(schemaField{
			Type:        f.spec.Type,
			Required:    f.spec.Required,
			Enum:        f.spec.Enum,
			Description: f.spec.Description,
		})
		if err != nil {
			return nil, err
		}
		buf.Write(spec)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// schemaDocumentOf builds the ops-schema output from opRegistry.
func schemaDocumentOf() *schemaDocument {
	doc := &schemaDocument{Version: opsVersion}
	for _, op := range opRegistry {
		sop := schemaOp{Op: op.Name, AppliesTo: append([]string(nil), op.AppliesTo...), Fields: fieldList{}}
		for _, f := range op.Fields {
			sop.Fields = append(sop.Fields, namedField{name: f.Name, spec: f})
		}
		doc.Operations = append(doc.Operations, sop)
	}
	return doc
}

// ---- op result envelopes ----

// clearCellOutput is clear_cell's result.
type clearCellOutput struct {
	Selector string `json:"selector"`
}

// addSheetOutput is add_sheet's result.
type addSheetOutput struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

// removeSheetOutput is remove_sheet's result.
type removeSheetOutput struct {
	Name string `json:"name"`
}

// addSlideOutput is add_slide's result: the new 1-based slide index and,
// when added from a layout, the SlideLayoutType string used (null otherwise).
type addSlideOutput struct {
	Index  int     `json:"index"`
	Layout *string `json:"layout"`
}

// removeSlideOutput is remove_slide's result.
type removeSlideOutput struct {
	Index int `json:"index"`
}

// setPropertiesOutput reports the properties that were set.
type setPropertiesOutput struct {
	Title    *string `json:"title,omitempty"`
	Subject  *string `json:"subject,omitempty"`
	Creator  *string `json:"creator,omitempty"`
	Keywords *string `json:"keywords,omitempty"`
}
