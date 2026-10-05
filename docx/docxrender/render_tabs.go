package docxrender

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// Tab stops.
//
// forme has one tab size: stops at its multiples, from the paragraph's left
// edge. Word's stops are explicit positions from the page margin, with an
// alignment and a leader, and the default stops start at the margin whatever
// the indent. A paragraph that needs Word's stops (it has custom stops, or a
// left indent that is not a multiple of the default stop) therefore sets its
// tabs itself:
//
//   - while the paragraph is translated each tab is a token in the markup, in
//     its own span boundary, and each manual line break likewise; the block
//     keeps the pieces (wordTabBlock);
//   - before the section is laid out, the text between tokens is measured by
//     laying it out once, unwrapped, in forme itself, so widths include the
//     same shaping the real layout does;
//   - the tab widths are computed from the stops, and each tab becomes an
//     inline block of that width (holding its leader characters, if any).
//
// The widths assume the paragraph's text sits on one line between manual
// breaks. After the real layout a paragraph whose lines are not the manual
// lines is reported as approximated.

// wordTabItem is one token of a tab paragraph.
type wordTabItem struct {
	// kind is 'T' for a tab and 'B' for a manual line break.
	kind byte
	// open and closing are the markup of the run's span at the tab, which a
	// leader is set in; nodes is how many elements it is.
	open, closing string
	nodes         int
	// html is the break's markup.
	html string
	// marker is the tab after a list marker (render_numbering.go): it only
	// sets where the first line's text starts, so a paragraph that wraps does
	// not make it approximate.
	marker bool
}

// wordTabBlock is a paragraph block whose tabs are not yet resolved.
type wordTabBlock struct {
	// open is the paragraph element's opening tag; content its markup, with
	// the tokens in place.
	open, content string
	items         []wordTabItem
	// left is the paragraph's left indent and first the first line's offset
	// from it, in pixels; the stops are measured from the text column's left
	// edge, which is the wrapper's.
	left, first float64
	// firstLine is set on the block that holds the paragraph's first line.
	firstLine  bool
	stops      []wordTab
	defaultTab float64
	// markerHTML is the markup of a list marker that begins the content and is
	// aligned on the first line's start by its width (markerAlign right or
	// center); openAt returns the opening tag for a given first-line offset.
	markerHTML, markerAlign string
	openAt                  func(first float64) string
}

// wordEffectiveTabs resolves a cascaded list of tab stops: later entries
// replace earlier ones at the same position, and a clear removes the stop.
func wordEffectiveTabs(list []wordTab) []wordTab {
	var out []wordTab
	for _, t := range list {
		kept := out[:0:0]
		for _, o := range out {
			if math.Abs(o.pos-t.pos) > 0.01 {
				kept = append(kept, o)
			}
		}
		out = kept
		if t.val != "clear" {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].pos < out[j].pos })
	return out
}

// wordNeedsTabLayout reports whether a paragraph with these stops and left
// indent cannot use forme's own tabs.
func wordNeedsTabLayout(stops []wordTab, left, defaultTab float64) bool {
	for _, s := range stops {
		if s.val != "bar" {
			return true
		}
	}
	rem := math.Abs(math.Mod(left, defaultTab))
	return rem > 0.01 && defaultTab-rem > 0.01
}

const wordTabSentinel = "\x01"

// tabToken replaces a tab with a token, closing the run's span around it.
func (rn *wordRun) tabToken(kind byte) error {
	slot := rn.slotForPiece()
	open, closing, err := rn.span(slot)
	if err != nil {
		return err
	}
	nodes := rn.spans[slot].nodes
	if err = rn.closeSpan(); err != nil {
		return err
	}
	p := rn.p
	item := wordTabItem{kind: kind, open: open, closing: closing, nodes: nodes}
	if kind == 'B' {
		if err = p.r.charge(nodes); err != nil {
			return err
		}
		item.html = open + "<br>" + closing
	}
	p.tabItems = append(p.tabItems, item)
	p.sb.WriteString(wordTabSentinel + string(kind) + strconv.Itoa(len(p.tabItems)-1) + wordTabSentinel)
	p.hasContent = true
	return nil
}

// wordTabPart is a piece of a tab block's content: markup, or an item.
type wordTabPart struct {
	html string
	item int // -1 for markup
}

func wordTabParts(tb *wordTabBlock) ([]wordTabPart, error) {
	var parts []wordTabPart
	rest := tb.content
	for {
		i := strings.Index(rest, wordTabSentinel)
		if i < 0 {
			if rest != "" {
				parts = append(parts, wordTabPart{html: rest, item: -1})
			}
			return parts, nil
		}
		if i > 0 {
			parts = append(parts, wordTabPart{html: rest[:i], item: -1})
		}
		rest = rest[i+1:]
		j := strings.Index(rest, wordTabSentinel)
		if j < 2 {
			return nil, fmt.Errorf("%w: tab token", render.ErrInvalid)
		}
		n, err := strconv.Atoi(rest[1:j])
		if err != nil || n < 0 || n >= len(tb.items) || tb.items[n].kind != rest[0] {
			return nil, fmt.Errorf("%w: tab token", render.ErrInvalid)
		}
		parts = append(parts, wordTabPart{item: n})
		rest = rest[j+1:]
	}
}

// measure lays out each markup fragment on its own unwrapped line and returns
// its width in pixels.
func (r *wordRenderer) measure(fragments []string) ([]float64, error) {
	if len(fragments) == 0 {
		return nil, nil
	}
	strut := r.fonts.first
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="margin:0;font-family:`)
	sb.WriteString(strut.css)
	sb.WriteString(`;font-size:1px;line-height:0;font-kerning:none;font-variant-ligatures:none">`)
	for i, f := range fragments {
		sb.WriteString(`<div id="m` + strconv.Itoa(i) + `" style="margin:0;white-space:pre;font-size:1px;line-height:0">`)
		sb.WriteString(f)
		sb.WriteString("</div>")
	}
	sb.WriteString("</body></html>")
	built := layout.Build(layout.Input{HTML: sb.String(), Fonts: r.fonts})
	if err := r.findings(built.Findings, built.Failed); err != nil {
		return nil, err
	}
	if built.Root == nil {
		return nil, fmt.Errorf("%w: layout produced no boxes", render.ErrUnsupported)
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	// Wide enough that nothing wraps, well inside layout's coordinate range.
	width, _ := style.FromPx(1 << 20)
	rec := layout.NewRecorder(nil)
	frag := layout.Layout(built.Root, layout.Size{W: width}, built.Fonts, rec)
	if frag == nil {
		return nil, fmt.Errorf("%w: layout produced no fragments", render.ErrUnsupported)
	}
	if err := r.findings(rec.Findings(), rec.Failed()); err != nil {
		return nil, err
	}
	frags := make([]*layout.Fragment, len(fragments))
	wordFindBlocks(frag, "m", frags)
	out := make([]float64, len(fragments))
	for i, f := range frags {
		if f == nil {
			return nil, fmt.Errorf("%w: layout dropped a measurement", render.ErrUnsupported)
		}
		out[i] = wordMeasured(f)
	}
	return out, nil
}

// wordMeasured is the width of the text on the first line of a fragment.
func wordMeasured(f *layout.Fragment) float64 {
	stack := []*layout.Fragment{f}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(cur.Lines) > 0 {
			end := 0.0
			for _, run := range cur.Lines[0].Runs {
				end = math.Max(end, run.X.Add(run.Width).Px())
			}
			return end
		}
		for i := len(cur.Children) - 1; i >= 0; i-- {
			stack = append(stack, cur.Children[i])
		}
	}
	return 0
}

// wordLeaderChar is the character a leader is drawn with, and whether Word's
// leader is drawn exactly by it.
func wordLeaderChar(leader string) (c rune, exact bool) {
	switch leader {
	case "dot":
		return '.', true
	case "hyphen":
		return '-', true
	case "underscore":
		return '_', true
	case "heavy":
		return '_', false
	case "middleDot":
		return '.', false
	}
	return 0, true
}

// nextStop picks the first stop after pen: the paragraph's own stops, the left
// indent of a hanging first line, and past them the default stops from the
// margin.
func (tb *wordTabBlock) nextStop(pen float64, firstLine bool) wordTab {
	const eps = 0.01
	best := wordTab{pos: math.Inf(1)}
	for _, s := range tb.stops {
		if s.val != "bar" && s.pos > pen+eps && s.pos < best.pos {
			best = s
		}
	}
	if firstLine && tb.first < 0 && tb.left > pen+eps && tb.left < best.pos {
		best = wordTab{pos: tb.left, val: "left"}
	}
	if math.IsInf(best.pos, 1) {
		best = wordTab{pos: (math.Floor(pen/tb.defaultTab+eps) + 1) * tb.defaultTab, val: "left"}
	}
	return best
}

// resolveTabs measures and resolves the tabs of every tab block of a section,
// leaving finished markup in block.inner.
func (r *wordRenderer) resolveTabs(sec *wordSection) error {
	type resolved struct {
		b     *wordBlock
		parts []wordTabPart
		// index of the measurement of each markup part, or -1.
		meas []int
		// markerMeas is the measurement of the aligned list marker, or -1.
		markerMeas int
	}
	var all []*resolved
	var fragments []string
	for _, b := range sec.blocks {
		if b.tab == nil {
			continue
		}
		parts, err := wordTabParts(b.tab)
		if err != nil {
			return err
		}
		rs := &resolved{b: b, parts: parts, meas: make([]int, len(parts)), markerMeas: -1}
		if b.tab.markerHTML != "" {
			rs.markerMeas = len(fragments)
			fragments = append(fragments, b.tab.markerHTML)
		}
		for i, p := range parts {
			rs.meas[i] = -1
			if p.item < 0 {
				rs.meas[i] = len(fragments)
				fragments = append(fragments, p.html)
			}
		}
		all = append(all, rs)
	}
	if len(all) == 0 {
		return nil
	}
	widths, err := r.measure(fragments)
	if err != nil {
		return err
	}
	type spacer struct {
		width  float64
		leader string
		item   int
	}
	type leaderProbe struct {
		k     int
		rs    *resolved
		part  int
		char  rune
		width float64
	}
	var probes []*leaderProbe
	spacers := make([]map[int]spacer, len(all))
	// shifts is how far an aligned list marker starts before the position the
	// first line starts at.
	shifts := make([]float64, len(all))
	for k, rs := range all {
		tb := rs.b.tab
		spacers[k] = map[int]spacer{}
		pen := tb.left + tb.first
		if rs.markerMeas >= 0 {
			shifts[k] = widths[rs.markerMeas]
			if tb.markerAlign == "center" {
				shifts[k] /= 2
			}
			pen -= shifts[k]
		}
		firstLine := tb.firstLine
		for i, p := range rs.parts {
			switch {
			case p.item < 0:
				pen += widths[rs.meas[i]]
			case tb.items[p.item].kind == 'B':
				pen, firstLine = tb.left, false
			default:
				next := 0.0
				if i+1 < len(rs.parts) && rs.parts[i+1].item < 0 {
					next = widths[rs.meas[i+1]]
				}
				stop := tb.nextStop(pen, firstLine)
				w := stop.pos - pen
				switch stop.val {
				case "right", "decimal":
					w -= next
				case "center":
					w -= next / 2
				}
				if stop.val == "decimal" {
					if err = r.approximate("decimal tab stops"); err != nil {
						return err
					}
				}
				w = math.Max(0, w)
				pen += w
				spacers[k][i] = spacer{width: w, leader: stop.leader, item: p.item}
				if c, exact := wordLeaderChar(stop.leader); c != 0 && w > 0 {
					if !exact {
						if err = r.approximate("tab leader " + stop.leader); err != nil {
							return err
						}
					}
					probes = append(probes, &leaderProbe{k: k, rs: rs, part: i, char: c})
				}
			}
		}
	}
	if len(probes) > 0 {
		html := make([]string, len(probes))
		for i, p := range probes {
			item := p.rs.b.tab.items[p.rs.parts[p.part].item]
			html[i] = item.open + string(p.char) + item.closing
		}
		cw, err := r.measure(html)
		if err != nil {
			return err
		}
		for i, p := range probes {
			p.width = cw[i]
		}
	}
	leaders := map[[2]int]*leaderProbe{}
	for _, p := range probes {
		leaders[[2]int{p.k, p.part}] = p
	}
	for k, rs := range all {
		tb := rs.b.tab
		var sb strings.Builder
		if shifts[k] != 0 && tb.openAt != nil {
			sb.WriteString(tb.openAt(tb.first - shifts[k]))
		} else {
			sb.WriteString(tb.open)
		}
		lines := 1
		checked := false
		for i, p := range rs.parts {
			switch {
			case p.item < 0:
				sb.WriteString(p.html)
			case tb.items[p.item].kind == 'B':
				sb.WriteString(tb.items[p.item].html)
				lines++
				checked = true
			default:
				checked = checked || !tb.items[p.item].marker
				sp := spacers[k][i]
				var c wordCSS
				c.add("display", "inline-block")
				c.px("width", sp.width)
				nodes := 1
				inner := ""
				if lp := leaders[[2]int{k, i}]; lp != nil && lp.width > 0 {
					n := int(sp.width / lp.width)
					if n > 8192 {
						n = 8192
					}
					if n > 0 {
						c.add("text-align", "right")
						c.add("white-space", "nowrap")
						item := tb.items[sp.item]
						inner = item.open + strings.Repeat(string(lp.char), n) + item.closing
						nodes += item.nodes
					}
				}
				if err = r.charge(nodes); err != nil {
					return err
				}
				sb.WriteString(`<span style="` + c.String() + `">` + inner + "</span>")
			}
		}
		sb.WriteString("</div>")
		rs.b.inner = sb.String()
		if checked {
			rs.b.expectLines = lines
		}
	}
	return nil
}
