package docx

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// fieldFormat holds the MergeOptions that format MERGEFIELD values with
// their \@ and \# switches; see MergeOptions.FormatSwitches.
type fieldFormat struct {
	Locale                           string
	DecimalSeparator, GroupSeparator string
	Names                            map[string]DateNames
	ParseDate                        func(value string) (time.Time, bool)
	ParseNumber                      func(value string) (string, bool)
	Warn                             func(field, value, reason string)
}

// fieldFormatOf returns the formatting options of opts, or nil when
// FormatSwitches is off.
func fieldFormatOf(opts MergeOptions) *fieldFormat {
	if !opts.FormatSwitches {
		return nil
	}
	return &fieldFormat{
		Locale:           opts.Locale,
		DecimalSeparator: opts.DecimalSeparator,
		GroupSeparator:   opts.GroupSeparator,
		Names:            opts.DateNames,
		ParseDate:        opts.ParseDate,
		ParseNumber:      opts.ParseNumber,
		Warn:             opts.Warn,
	}
}

// DateNames are the month and day names of one language for \@ date
// formats: MMMM, MMM, dddd and ddd. Days start on Sunday.
type DateNames struct {
	Months, ShortMonths [12]string
	Days, ShortDays     [7]string
}

// builtinDateNames holds the month and day names of the built-in languages.
// Word takes them from the operating system's locale data, so they can differ
// between Windows and macOS; these follow Word's output where it was checked
// (Word for Mac: da, and en and de) and the common spelling otherwise.
var builtinDateNames = map[string]DateNames{
	"en": {
		Months:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		ShortMonths: [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		Days:        [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		ShortDays:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
	},
	"de": {
		Months:      [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"},
		ShortMonths: [12]string{"Jan", "Feb", "Mär", "Apr", "Mai", "Jun", "Jul", "Aug", "Sep", "Okt", "Nov", "Dez"},
		Days:        [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"},
		ShortDays:   [7]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"},
	},
	"nl": {
		Months:      [12]string{"januari", "februari", "maart", "april", "mei", "juni", "juli", "augustus", "september", "oktober", "november", "december"},
		ShortMonths: [12]string{"jan", "feb", "mrt", "apr", "mei", "jun", "jul", "aug", "sep", "okt", "nov", "dec"},
		Days:        [7]string{"zondag", "maandag", "dinsdag", "woensdag", "donderdag", "vrijdag", "zaterdag"},
		ShortDays:   [7]string{"zo", "ma", "di", "wo", "do", "vr", "za"},
	},
	"fr": {
		Months:      [12]string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"},
		ShortMonths: [12]string{"janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."},
		Days:        [7]string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"},
		ShortDays:   [7]string{"dim.", "lun.", "mar.", "mer.", "jeu.", "ven.", "sam."},
	},
	"es": {
		Months:      [12]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"},
		ShortMonths: [12]string{"ene.", "feb.", "mar.", "abr.", "may.", "jun.", "jul.", "ago.", "sep.", "oct.", "nov.", "dic."},
		Days:        [7]string{"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
		ShortDays:   [7]string{"do.", "lu.", "ma.", "mi.", "ju.", "vi.", "sá."},
	},
	"it": {
		Months:      [12]string{"gennaio", "febbraio", "marzo", "aprile", "maggio", "giugno", "luglio", "agosto", "settembre", "ottobre", "novembre", "dicembre"},
		ShortMonths: [12]string{"gen", "feb", "mar", "apr", "mag", "giu", "lug", "ago", "set", "ott", "nov", "dic"},
		Days:        [7]string{"domenica", "lunedì", "martedì", "mercoledì", "giovedì", "venerdì", "sabato"},
		ShortDays:   [7]string{"dom", "lun", "mar", "mer", "gio", "ven", "sab"},
	},
	"da": {
		Months:      [12]string{"januar", "februar", "marts", "april", "maj", "juni", "juli", "august", "september", "oktober", "november", "december"},
		ShortMonths: [12]string{"jan", "feb", "mar", "apr", "maj", "jun", "jul", "aug", "sep", "okt", "nov", "dec"},
		Days:        [7]string{"søndag", "mandag", "tirsdag", "onsdag", "torsdag", "fredag", "lørdag"},
		ShortDays:   [7]string{"søn.", "man.", "tirs.", "ons.", "tors.", "fre.", "lør."},
	},
}

// numberSymbols holds the decimal and digit-grouping symbols of the built-in
// languages; other languages use the English ones.
var numberSymbols = map[string][2]string{
	"en": {".", ","},
	"de": {",", "."},
	"nl": {",", "."},
	"fr": {",", " "},
	"es": {",", "."},
	"it": {",", "."},
	"da": {",", "."},
}

// primaryLanguage returns the lower-cased primary subtag of a language tag.
func primaryLanguage(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	return tag
}

// separators returns the decimal and grouping symbols the options use.
func (o *fieldFormat) separators() (dec, grp string) {
	locale := o.Locale
	if locale == "" {
		locale = "en-US"
	}
	syms, ok := numberSymbols[primaryLanguage(locale)]
	if !ok {
		syms = numberSymbols["en"]
	}
	dec, grp = syms[0], syms[1]
	if o.DecimalSeparator != "" {
		dec = o.DecimalSeparator
	}
	if o.GroupSeparator != "" {
		grp = o.GroupSeparator
	}
	return dec, grp
}

// names returns the month and day names for the first of the languages that
// has them, then English.
func (o *fieldFormat) names(langs ...string) DateNames {
	for _, l := range langs {
		p := primaryLanguage(l)
		if p == "" {
			continue
		}
		if n, ok := o.Names[p]; ok {
			return n
		}
		if n, ok := builtinDateNames[p]; ok {
			return n
		}
	}
	return builtinDateNames["en"]
}

// isoLayouts are the default date and time forms, tried in order.
var isoLayouts = []string{
	"2006-01-02",
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
	"15:04",
	"15:04:05",
}

func (o *fieldFormat) parseDate(value string) (time.Time, bool) {
	if o.ParseDate != nil {
		return o.ParseDate(value)
	}
	v := strings.TrimSpace(value)
	for _, layout := range isoLayouts {
		if t, err := time.Parse(layout, v); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// decimalRe is the canonical decimal form: an optional sign, digits, and an
// optional point with digits on either side.
var decimalRe = regexp.MustCompile(`^([+-]?)([0-9]*)(?:\.([0-9]*))?$`)

// decimal is an exact decimal number.
type decimal struct {
	neg  bool
	int  string // integer digits without leading zeros; "" for 0
	frac string // fraction digits
}

func parseDecimal(s string) (decimal, bool) {
	m := decimalRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil || (m[2] == "" && m[3] == "") {
		return decimal{}, false
	}
	d := decimal{neg: m[1] == "-", int: strings.TrimLeft(m[2], "0"), frac: m[3]}
	if d.isZero() {
		d.neg = false
	}
	return d, true
}

func (d decimal) isZero() bool {
	return d.int == "" && strings.Trim(d.frac, "0") == ""
}

// round rounds to places fraction digits, half away from zero, as Word
// rounds field results.
func (d decimal) round(places int) decimal {
	if len(d.frac) <= places {
		d.frac += strings.Repeat("0", places-len(d.frac))
		return d
	}
	up := d.frac[places] >= '5'
	digits := []byte(d.int + d.frac[:places])
	if up {
		i := len(digits) - 1
		for ; i >= 0; i-- {
			if digits[i] == '9' {
				digits[i] = '0'
				continue
			}
			digits[i]++
			break
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		}
	}
	n := len(digits) - places
	out := decimal{neg: d.neg, int: strings.TrimLeft(string(digits[:n]), "0"), frac: string(digits[n:])}
	if out.isZero() {
		out.neg = false
	}
	return out
}

func (o *fieldFormat) parseNumber(value string) (decimal, bool) {
	if o.ParseNumber != nil {
		s, ok := o.ParseNumber(value)
		if !ok {
			return decimal{}, false
		}
		return parseDecimal(s)
	}
	return parseDecimal(value)
}

// formatDatePicture writes t with a Word date-time picture. It returns false
// with a reason for an item it does not support (a `numbered item`).
func formatDatePicture(t time.Time, picture string, names DateNames) (string, string) {
	var b strings.Builder
	rs := []rune(picture)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '\'':
			// Quoted text. Word's documentation shows "MMM-d, 'yy" as
			// "Nov-6, '99", but Word itself refuses the unmatched quote.
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				j++
			}
			if j == len(rs) {
				// Word refuses the field: "Picture string contains
				// unmatched quotes."
				return "", "the format has unmatched quotes"
			}
			b.WriteString(string(rs[i+1 : j]))
			i = j + 1
			continue
		case r == '`':
			return "", "numbered items (`…`) in date pictures are not supported"
		case strings.EqualFold(string(rs[i:min(i+5, len(rs))]), "am/pm"):
			// The case follows the format: Word writes "am" for am/pm and
			// "AM" for AM/PM.
			mark := "AM"
			if t.Hour() >= 12 {
				mark = "PM"
			}
			if r == 'a' {
				mark = strings.ToLower(mark)
			}
			b.WriteString(mark)
			i += 5
			continue
		}
		n := 1
		for i+n < len(rs) && rs[i+n] == r {
			n++
		}
		switch r {
		case 'M':
			switch n {
			case 1:
				fmt.Fprintf(&b, "%d", int(t.Month()))
			case 2:
				fmt.Fprintf(&b, "%02d", int(t.Month()))
			case 3:
				b.WriteString(names.ShortMonths[t.Month()-1])
			default:
				b.WriteString(names.Months[t.Month()-1])
			}
		case 'd', 'D':
			switch n {
			case 1:
				fmt.Fprintf(&b, "%d", t.Day())
			case 2:
				fmt.Fprintf(&b, "%02d", t.Day())
			case 3:
				b.WriteString(names.ShortDays[t.Weekday()])
			default:
				b.WriteString(names.Days[t.Weekday()])
			}
		case 'y', 'Y':
			if n <= 2 {
				fmt.Fprintf(&b, "%02d", t.Year()%100)
			} else {
				fmt.Fprintf(&b, "%04d", t.Year())
			}
		case 'h':
			h := t.Hour() % 12
			if h == 0 {
				h = 12
			}
			writeClock(&b, h, n)
		case 'H':
			writeClock(&b, t.Hour(), n)
		case 'm':
			writeClock(&b, t.Minute(), n)
		case 's', 'S':
			writeClock(&b, t.Second(), n)
		default:
			b.WriteString(string(rs[i : i+n]))
		}
		i += n
	}
	return b.String(), ""
}

func writeClock(b *strings.Builder, v, n int) {
	if n >= 2 {
		fmt.Fprintf(b, "%02d", v)
	} else {
		fmt.Fprintf(b, "%d", v)
	}
}

// numItem is one item of a numeric picture section.
type numItem struct {
	kind byte   // '0', '#', 'x' placeholders; '.' decimal; '-', '+' signs; 'l' literal
	text string // literal text
}

// parseNumberSection splits one numeric picture section into items, reading
// the locale's decimal symbol as the decimal point and its grouping symbol,
// before the decimal point, as grouping. It reports whether the section
// groups digits, or a reason for an unsupported item.
func parseNumberSection(sec, dec, grp string) ([]numItem, bool, string) {
	var items []numItem
	grouping := false
	seenDecimal := false
	for i := 0; i < len(sec); {
		r, size := utf8.DecodeRuneInString(sec[i:])
		switch {
		case r == '\'':
			j := strings.IndexByte(sec[i+1:], '\'')
			if j < 0 {
				// Word refuses the field: "Picture string contains
				// unmatched quotes."
				return nil, false, "the format has unmatched quotes"
			}
			end := i + 1 + j
			items = append(items, numItem{kind: 'l', text: sec[i+1 : end]})
			i = end + 1
			continue
		case r == '`':
			return nil, false, "numbered items (`…`) in numeric pictures are not supported"
		case r == '0' || r == '#' || r == 'x':
			items = append(items, numItem{kind: byte(r)})
		case !seenDecimal && strings.HasPrefix(sec[i:], dec):
			seenDecimal = true
			items = append(items, numItem{kind: '.'})
			i += len(dec)
			continue
		case !seenDecimal && grp != "" && strings.HasPrefix(sec[i:], grp):
			// Anywhere before the decimal point, even between quoted texts,
			// the grouping symbol turns on grouping and is not written:
			// Word shows 12345 with "0 'items', 'x'" as "12,345 items x".
			grouping = true
			i += len(grp)
			continue
		case r == '-' || r == '+':
			items = append(items, numItem{kind: byte(r)})
		default:
			items = append(items, numItem{kind: 'l', text: string(r)})
		}
		i += size
	}
	return items, grouping, ""
}

// formatNumberPicture writes d with a Word numeric picture: placeholders 0
// (a digit or 0), # (a digit or a space) and x (drops the digits to its left,
// or rounds at its place after the decimal point; a space where no digit is
// left), the decimal and grouping
// symbols of the locale, - and + signs, 'text' and other literal characters,
// and positive;negative;zero sections. A negative number formatted with a
// section that has no sign item gets a minus sign right before its number,
// after any text in front. A # after the decimal point shows nothing for a
// trailing zero. A format with no digit placeholder shows only its text, as in
// Word. An empty section for the value, or unmatched quotes, is refused with
// a reason.
func formatNumberPicture(d decimal, picture, dec, grp string) (string, string) {
	sections := splitSections(picture)
	// The section is chosen by the exact number, before rounding: Word shows
	// -0.001 with "0.00;(0.00)" as "(0.00)" and 0.001 with "0;-0;'zero'" as
	// "0".
	sec := sections[0]
	autoMinus := d.neg
	neg := d.neg
	negSection := false
	switch {
	case d.isZero() && len(sections) >= 3:
		sec = sections[2]
	case d.neg && len(sections) >= 2:
		sec, autoMinus, negSection = sections[1], false, true
		d.neg = false
	}
	if strings.TrimSpace(sec) == "" {
		return "", "the number format section for this value is empty"
	}
	items, grouping, reason := parseNumberSection(sec, dec, grp)
	if reason != "" {
		return "", reason
	}

	var intPH, fracPH []int // indexes into items
	afterDecimal := false
	for i, it := range items {
		switch it.kind {
		case '.':
			afterDecimal = true
		case '0', '#', 'x':
			if afterDecimal {
				fracPH = append(fracPH, i)
			} else {
				intPH = append(intPH, i)
			}
		case '-', '+':
			autoMinus = false
		}
	}
	// An x after the decimal point is the last place shown: it rounds there
	// and later placeholders show nothing.
	places := len(fracPH)
	for k, idx := range fracPH {
		if items[idx].kind == 'x' {
			places = k + 1
			break
		}
	}
	r := d.round(places)
	digits := r.int
	// A # after the decimal point shows its digit only when it is not a
	// trailing zero: Word shows 5.5 with "0.0#" as "5.5".
	shown := places
	for shown > 0 && items[fracPH[shown-1]].kind == '#' && r.frac[shown-1] == '0' {
		shown--
	}
	// In the first section the sign is the rounded result's: Word shows
	// -0.001 with "0.00" as "0.00". The negative section keeps it.
	if neg && r.isZero() && !negSection {
		neg = false
	}

	// An x before the decimal point drops the digits to its left.
	for k, idx := range intPH {
		if items[idx].kind == 'x' {
			keep := len(intPH) - k
			if len(digits) > keep {
				digits = digits[len(digits)-keep:]
			}
			break
		}
	}

	// Characters for each integer placeholder, right-aligned; digits that do
	// not fit go before the first placeholder.
	intOut := make(map[int]string, len(intPH))
	k := len(intPH)
	overflow := ""
	if len(digits) > k {
		overflow = digits[:len(digits)-k]
		digits = digits[len(digits)-k:]
	}
	pad := k - len(digits)
	var cells []string // integer characters left to right, overflow first
	for _, ch := range overflow {
		cells = append(cells, string(ch))
	}
	for j, idx := range intPH {
		c := ""
		if j >= pad {
			c = string(digits[j-pad])
		} else if items[idx].kind == '0' {
			c = "0"
		} else {
			c = " "
		}
		cells = append(cells, c)
	}
	if grouping {
		cells = groupCells(cells, grp)
	}
	if k > 0 {
		head := strings.Join(cells[:len(cells)-k+1], "")
		intOut[intPH[0]] = head
		for j := 1; j < k; j++ {
			intOut[intPH[j]] = cells[len(cells)-k+j]
		}
	}

	var b strings.Builder
	// The automatic minus goes right before the number, after any text in
	// front of it: Word shows -5 with "$#,##0" as "$-   5".
	minus := autoMinus && neg
	writeMinus := func() {
		if minus {
			b.WriteString("-")
			minus = false
		}
	}
	// A picture with no placeholder before the decimal point still shows the
	// integer digits, there.
	looseInt := ""
	if k == 0 {
		looseInt = overflow
	}
	fracIdx := 0
	for i, it := range items {
		switch it.kind {
		case 'l':
			b.WriteString(it.text)
		case '.':
			if places > 0 || looseInt != "" {
				writeMinus()
			}
			b.WriteString(looseInt)
			looseInt = ""
			if places > 0 {
				b.WriteString(dec)
			}
		case '-':
			if neg {
				b.WriteString("-")
			} else {
				b.WriteString(" ")
			}
		case '+':
			switch {
			case neg:
				b.WriteString("-")
			case r.isZero():
				b.WriteString(" ")
			default:
				b.WriteString("+")
			}
		case '0', '#', 'x':
			writeMinus()
			if s, ok := intOut[i]; ok {
				b.WriteString(s)
				continue
			}
			if fracIdx < shown {
				b.WriteByte(r.frac[fracIdx])
			}
			fracIdx++
		}
	}
	return b.String(), ""
}

// splitSections splits a numeric picture on the ; outside quoted text.
func splitSections(picture string) []string {
	var out []string
	inQuote := false
	start := 0
	for i := 0; i < len(picture); i++ {
		switch picture[i] {
		case '\'':
			inQuote = !inQuote
		case ';':
			if !inQuote {
				out = append(out, picture[start:i])
				start = i + 1
			}
		}
	}
	return append(out, picture[start:])
}

// groupCells inserts the grouping symbol between every three integer digits,
// counted from the right, where both neighbours are digits. The symbol joins
// the cell on its left, so each placeholder keeps one cell.
func groupCells(cells []string, grp string) []string {
	n := len(cells)
	out := make([]string, n)
	copy(out, cells)
	for i := n - 1; i > 0; i-- {
		fromRight := n - i // digits at and right of i
		if fromRight%3 != 0 {
			continue
		}
		if isDigitCell(cells[i-1]) && isDigitCell(cells[i]) {
			out[i-1] += grp
		}
	}
	return out
}

func isDigitCell(c string) bool {
	return len(c) == 1 && c[0] >= '0' && c[0] <= '9'
}

// applyPicture formats value with the \@ date or \# number format (Word's
// "picture") of a MERGEFIELD instruction's switches when opts is set. It
// returns the value unchanged, after reporting to opts.Warn, when it cannot.
func applyPicture(field, value string, sw fieldSwitches, opts *fieldFormat, langs ...string) string {
	if opts == nil || value == "" || (!sw.hasDate && !sw.hasNumber) {
		return value
	}
	warn := func(reason string) string {
		if opts.Warn != nil {
			opts.Warn(field, value, reason)
		}
		return value
	}
	if sw.hasDate {
		if strings.TrimSpace(sw.date) == "" {
			return warn("the date format is empty")
		}
		t, ok := opts.parseDate(value)
		if !ok {
			return warn("not a date the options read")
		}
		out, reason := formatDatePicture(t, sw.date, opts.names(append(langs, opts.Locale)...))
		if reason != "" {
			return warn(reason)
		}
		return out
	}
	if strings.TrimSpace(sw.number) == "" {
		return warn("the number format is empty")
	}
	num, ok := opts.parseNumber(value)
	if !ok {
		return warn("not a number the options read")
	}
	dec, grp := opts.separators()
	out, reason := formatNumberPicture(num, sw.number, dec, grp)
	if reason != "" {
		return warn(reason)
	}
	return out
}
