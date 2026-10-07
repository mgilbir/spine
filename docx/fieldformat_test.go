package docx

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

func TestFormatNumberPicture(t *testing.T) {
	cases := []struct {
		value, picture, want string
		de                   bool
	}{
		// Word's documented examples (Format field results).
		{"9", "00.00", "09.00", false},
		{"15", "$###", "$ 15", false},
		{"492492", "x##", "492", false},
		{"0.125", "0.00x", "0.125", false},
		{"495.47", "$###.00", "$495.47", false},
		{"2456800", "$#,###,###", "$2,456,800", false},
		{"-80", "-##", "-80", false},
		{"10", "+##", "+10", false},
		{"-10", "+##", "-10", false},
		{"33", "##%", "33%", false},
		{"347.4372", "$##0.00 'is sales tax'", "$347.44 is sales tax", false},
		{"1245.65", "$#,##0.00;($#,##0.00);$0", "$1,245.65", false},
		// # pads with a space, as in Word's own "-$ 345.56".
		{"-345.56", "$#,##0.00;($#,##0.00);$0", "($ 345.56)", false},
		{"-345.56", "$#,##0.00;-$#,##0.00", "-$ 345.56", false},
		{"0", "$#,##0.00;($#,##0.00);$0", "$0", false},
		// Rounding is decimal and half away from zero.
		{"2.345", "0.00", "2.35", false},
		{"-2.345", "0.00", "-2.35", false},
		{"0.995", "0.00", "1.00", false},
		{"999.5", "0", "1000", false},
		{"1.005", "0.00", "1.01", false},
		// A negative number with no sign item gets a minus sign.
		{"-5", "0", "-5", false},
		{"-0.001", "0.00", "0.00", false},
		// Digits that do not fit, and integers with no placeholder.
		{"123456", "00", "123456", false},
		{"12.5", ".00", "12.50", false},
		{"5", "-0", " 5", false},
		{"0", "+0", " 0", false},
		// The locale's symbols read and write the picture.
		{"1234567.891", "#.##0,00", "1.234.567,89", true},
		{"0.5", "0,00 €", "0,50 €", true},
		// Checked in Word: the grouping symbol anywhere before the decimal
		// point turns grouping on and is not written, even between quoted
		// texts; the first decimal symbol is the decimal point.
		{"12345", "0 'items', 'x'", "12,345 items x", false},
		{"1234.5", "# ##0 €", "1 235€", false},
		{"5", "0 Kč.", "5 Kč.", false},
		// Checked in Word: sections are chosen by the exact number.
		{"0.001", "0;-0;'zero'", "0", false},
		{"-0.001", "0.00;(0.00)", "(0.00)", false},
		{"-0.004", "0.00;(0.00);'nil'", "(0.00)", false},
		{"-0.001", "0.00;-0.00", " 0.00", false},
		// Checked in Word: # after the decimal point shows a space for a
		// trailing zero, and the automatic minus goes right before the
		// number, after text and x placeholders.
		{"-5.5", "#,##0.0#", "-   5.5 ", false},
		{"5.5", "0.0#", "5.5 ", false},
		{"-5", "x##", " - 5", false},
		// Checked in Word: the decimal symbol is written with no digit
		// after it, and empty formats and sections show nothing.
		{"5", "0.", "5.", false},
		{"5", "0.##", "5.  ", false},
		{"0.5", "#.##", " .5 ", false},
		{"5.5", "0.#0", "5.50", false},
		{"5", "", "", false},
		{"-5", "0;", "", false},
		{"0", "0;-0;", "", false},
		// Checked in Word: rounding is half away from zero, and literals
		// around the number stay.
		{"2.5", "0", "3", false},
		{"-2.5", "0", "-3", false},
		{"0.125", "0.00", "0.13", false},
		{"-5", "(0)", "(-5)", false},
		{"5.25", "0.0x", "5.25", false},
		{"1234.5", "'Total: '#,##0.00' EUR'", "Total: 1,234.50 EUR", false},
		{"5.25", "0.0#", "5.25", false},
		{"-5", "$#,##0", "$-   5", false},
		// Checked in Word: a format with no digit placeholder shows only its
		// text, and a doubled apostrophe is no escape.
		{"5", "$", "$", false},
		{"5", "'it''s' 0", "its 5", false},
	}
	for _, c := range cases {
		d, ok := parseDecimal(c.value)
		if !ok {
			t.Fatalf("parseDecimal(%q) failed", c.value)
		}
		dec, grp := ".", ","
		if c.de {
			dec, grp = ",", "."
		}
		if strings.HasPrefix(c.picture, "# ##0") {
			dec, grp = ",", " "
		}
		got, reason := formatNumberPicture(d, c.picture, dec, grp)
		if reason != "" || got != c.want {
			t.Errorf("formatNumberPicture(%s, %q) = %q (%s), want %q", c.value, c.picture, got, reason, c.want)
		}
	}
}

func TestFormatDatePicture(t *testing.T) {
	en, de := builtinDateNames["en"], builtinDateNames["de"]
	sat := time.Date(2022, 11, 26, 10, 0, 5, 0, time.UTC)
	cases := []struct {
		at      time.Time
		picture string
		names   DateNames
		want    string
	}{
		// Word's documented examples.
		{sat, "dddd, MMMM d", en, "Saturday, November 26"},
		// Word writes am/pm in the case of the format (checked in Word; its
		// documentation says upper case).
		{sat, "h:mm AM/PM, dddd, MMMM d", en, "10:00 AM, Saturday, November 26"},
		{sat, "h:mm am/pm", en, "10:00 am"},
		{time.Date(2022, 1, 1, 12, 45, 0, 0, time.UTC), "HH:mm 'Greenwich mean time'", en, "12:45 Greenwich mean time"},
		{time.Date(1999, 11, 6, 11, 15, 0, 0, time.UTC), "HH:mm MMM-d, ''yy", en, "11:15 Nov-6, 99"},
		{time.Date(2006, 7, 6, 9, 2, 5, 0, time.UTC), "M MM MMM d dd ddd yy yyyy h hh m mm s ss", en, "7 07 Jul 6 06 Thu 06 2006 9 09 2 02 5 05"},
		// 12- and 24-hour clocks.
		{time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC), "h:mm AM/PM", en, "12:30 AM"},
		{time.Date(2026, 1, 1, 13, 5, 0, 0, time.UTC), "h:mm AM/PM|HH:mm", en, "1:05 PM|13:05"},
		// Names follow the language.
		{time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC), "dddd, d. MMMM yyyy", de, "Freitag, 17. April 2026"},
		{time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), "ddd dd.MMM.yyyy", de, "So 01.Mär.2026"},
	}
	for _, c := range cases {
		got, reason := formatDatePicture(c.at, c.picture, c.names)
		if reason != "" || got != c.want {
			t.Errorf("formatDatePicture(%v, %q) = %q (%s), want %q", c.at, c.picture, got, reason, c.want)
		}
	}
}

func TestFormatValueParsing(t *testing.T) {
	o := &fieldFormat{}
	for _, v := range []string{"2026-04-17", "2026-04-17T09:30", "2026-04-17 09:30:00", "2026-04-17T09:30:00+02:00", "2026-04-17T09:30:00.5Z", "09:30"} {
		if _, ok := o.parseDate(v); !ok {
			t.Errorf("parseDate(%q) failed", v)
		}
	}
	for _, v := range []string{"17.04.2026", "04/17/2026", "tomorrow", ""} {
		if _, ok := o.parseDate(v); ok {
			t.Errorf("parseDate(%q) succeeded, want the ISO forms only", v)
		}
	}
	for _, v := range []string{"1234.5", "-0.25", "+3", ".5", "7."} {
		if _, ok := o.parseNumber(v); !ok {
			t.Errorf("parseNumber(%q) failed", v)
		}
	}
	for _, v := range []string{"1,234.5", "1.234,5", "1e3", "", "-", "."} {
		if _, ok := o.parseNumber(v); ok {
			t.Errorf("parseNumber(%q) succeeded", v)
		}
	}
	custom := &fieldFormat{ParseNumber: func(v string) (string, bool) {
		return strings.ReplaceAll(strings.ReplaceAll(v, ".", ""), ",", "."), true
	}}
	if d, ok := custom.parseNumber("1.234,5"); !ok || d.int != "1234" || d.frac != "5" {
		t.Errorf("custom parseNumber = %+v, %v", d, ok)
	}
}

func TestFillMergeFieldsFormatSwitches(t *testing.T) {
	lang := func(l string) string { return `<w:rPr><w:lang w:val="` + l + `"/></w:rPr>` }
	body := `<w:body><w:p>` +
		wordMergeField(` MERGEFIELD Born \@ "d. MMMM yyyy" `, `«Born»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD Born \@ "MMMM d, yyyy" `, `«Born»`, lang("en-US")) + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD Total \# "#.##0,00 €" \b "Summe: " `, `«Total»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD Born \@ "dd.MM.yyyy" `, `«Born»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD Note \@ "dd.MM.yyyy" \* Upper `, `«Note»`, "") +
		`</w:p></w:body>`
	doc := openDocFixture(t, body)
	var warnings []string
	unfilled := doc.FillMergeFieldsWith(
		map[string]string{"Born": "1970-01-31", "Total": "1234.5", "Note": "unknown"},
		MergeOptions{
			FormatSwitches: true,
			Locale:         "de-DE",
			Warn:           func(field, value, reason string) { warnings = append(warnings, field+"="+value) },
		},
	)
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	want := "31. Januar 1970|January 31, 1970|Summe: 1.234,50 €|31.01.1970|UNKNOWN"
	if got := doc.Paragraphs()[0].Text(); got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(warnings, []string{"Note=unknown"}) {
		t.Errorf("warnings = %v, want [Note=unknown]", warnings)
	}
}

// TestFillMergeFieldsFormatSwitchesOff checks that without FormatSwitches the values
// go in as given, as before.
func TestFillMergeFieldsFormatSwitchesOff(t *testing.T) {
	body := `<w:body><w:p>` + wordMergeField(` MERGEFIELD Born \@ "d. MMMM yyyy" `, `«Born»`, "") + `</w:p></w:body>`
	doc := openDocFixture(t, body)
	doc.FillMergeFields(map[string]string{"Born": "1970-01-31"})
	if got := doc.Paragraphs()[0].Text(); got != "1970-01-31" {
		t.Errorf("paragraph text = %q, want the value as given", got)
	}
}

// TestFillMergeFieldsFormatDocumentLanguage checks that month names follow
// the document's default language when the field sets none.
func TestFillMergeFieldsFormatDocumentLanguage(t *testing.T) {
	doc := openDocFixture(t, `<w:body><w:p>`+wordMergeField(` MERGEFIELD D \@ "MMMM" `, `«D»`, "")+`</w:p></w:body>`)
	doc.styles = &oxml.CT_Styles{DocDefaults: &oxml.CT_DocDefaults{RPrDefault: &oxml.CT_RPrDefault{
		RPr: &oxml.CT_RPr{Lang: &oxml.CT_Lang{Val: "nl-NL"}},
	}}}
	doc.FillMergeFieldsWith(map[string]string{"D": "2026-03-01"}, MergeOptions{FormatSwitches: true, Locale: "en-US"})
	if got := doc.Paragraphs()[0].Text(); got != "maart" {
		t.Errorf("paragraph text = %q, want %q", got, "maart")
	}
}

// FuzzFieldFormats feeds arbitrary values and pictures through both
// formatters; neither may panic, and the result must be valid UTF-8.
func FuzzFieldFormats(f *testing.F) {
	f.Add("1234.5", "#,##0.00;(#,##0.00);'zero'", "2026-04-17T09:30:00", "dddd, d. MMMM yyyy h:mm am/pm 'x")
	f.Add("-0.005", "x0.0x#", "09:30", "'unterminated")
	f.Add("", "", "", "")
	f.Add("9999999999999999999999.9999", "0", "2026-02-29", "MMMMMM dddddd yyyyy")
	f.Fuzz(func(t *testing.T, num, numPic, date, datePic string) {
		o := &fieldFormat{Locale: "de-DE"}
		if d, ok := o.parseNumber(num); ok {
			for _, syms := range [][2]string{{".", ","}, {",", "."}, {",", " "}} {
				out, _ := formatNumberPicture(d, numPic, syms[0], syms[1])
				if !utf8.ValidString(out) && utf8.ValidString(numPic) {
					t.Fatalf("invalid UTF-8 from formatNumberPicture(%q, %q): %q", num, numPic, out)
				}
			}
		}
		if at, ok := o.parseDate(date); ok {
			out, _ := formatDatePicture(at, datePic, builtinDateNames["fr"])
			if !utf8.ValidString(out) && utf8.ValidString(datePic) {
				t.Fatalf("invalid UTF-8 from formatDatePicture(%q, %q): %q", date, datePic, out)
			}
		}
	})
}

// TestFormatNumberPictureRefused checks that unmatched quotes are refused,
// as Word refuses them ("Picture string contains unmatched quotes").
func TestFormatNumberPictureRefused(t *testing.T) {
	for _, c := range []struct{ value, picture string }{
		{"5", "0 'abc"}, {"5", "'abc"},
	} {
		d, _ := parseDecimal(c.value)
		if got, reason := formatNumberPicture(d, c.picture, ".", ","); reason == "" {
			t.Errorf("formatNumberPicture(%s, %q) = %q, want a refusal", c.value, c.picture, got)
		}
	}
}

// TestFillMergeFieldsFormatSwitchEdges covers formats written as given and a
// format switch with no argument.
func TestFillMergeFieldsFormatSwitchEdges(t *testing.T) {
	body := `<w:body><w:p>` +
		wordMergeField(` MERGEFIELD A \# "" \b "x" `, `«A»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD A \# "$" `, `«A»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD D \@ \* MERGEFORMAT `, `«D»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD D \@ "yyyy" \* MERGEFORMAT `, `«D»`, "") +
		`</w:p></w:body>`
	doc := openDocFixture(t, body)
	var warned []string
	doc.FillMergeFieldsWith(map[string]string{"A": "5", "D": "2026-04-17"}, MergeOptions{
		FormatSwitches: true,
		Warn:           func(field, value, reason string) { warned = append(warned, field) },
	})
	// An empty number format shows nothing, as in Word, so \b goes too.
	if got, want := doc.Paragraphs()[0].Text(), "|$|2026-04-17|2026"; got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}
	if want := []string{"D"}; !reflect.DeepEqual(warned, want) {
		t.Errorf("warned = %v, want %v", warned, want)
	}
}

func TestParseDateZoneWithoutSeconds(t *testing.T) {
	if _, ok := (&fieldFormat{}).parseDate("2026-04-17T09:30Z"); !ok {
		t.Error(`parseDate("2026-04-17T09:30Z") failed`)
	}
}

// TestFormatDatePictureUnmatchedQuote checks that an unmatched apostrophe is
// refused, as Word refuses it ("Picture string contains unmatched quotes").
func TestFormatDatePictureUnmatchedQuote(t *testing.T) {
	at := time.Date(1999, 11, 6, 11, 15, 0, 0, time.UTC)
	if got, reason := formatDatePicture(at, "HH:mm MMM-d, 'yy", builtinDateNames["en"]); reason == "" {
		t.Errorf("formatDatePicture with an unmatched quote = %q, want a refusal", got)
	}
}

// TestFormatDatePictureAmPmSpelling checks that only am/pm and AM/PM are
// read; Word turns other spellings into garbage, so they are refused.
func TestFormatDatePictureAmPmSpelling(t *testing.T) {
	at := time.Date(2026, 3, 1, 13, 5, 0, 0, time.UTC)
	if got, reason := formatDatePicture(at, "h:mm Am/Pm", builtinDateNames["en"]); reason == "" {
		t.Errorf("formatDatePicture with Am/Pm = %q, want a refusal", got)
	}
}

// TestBuiltinSpanishNames pins the Spanish short names Word for Mac writes.
func TestBuiltinSpanishNames(t *testing.T) {
	es := builtinDateNames["es"]
	if got := strings.Join(es.ShortDays[:], " "); got != "dom lun mar mié jue vie sáb" {
		t.Errorf("es short days = %q", got)
	}
	if got := strings.Join(es.ShortMonths[:], " "); got != "ene feb mar abr may jun jul ago sept oct nov dic" {
		t.Errorf("es short months = %q", got)
	}
}
