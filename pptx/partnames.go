package pptx

import "fmt"

// partNameAlloc caches the set of part names already taken for one kind of
// part, so allocating the next name does not rebuild that set on every call.
//
// Three allocators shared the same shape and the same cost. Each built a map of
// every name of its kind plus every other part name, then probed "...1.xml",
// "...2.xml", ... from the start until one was free, formatting a string per
// probe step. Both halves are O(n) and each runs once per add, so every one of
// them was quadratic:
//
//	AddSlide          500 -> 2000 slides   17.1ms -> 280ms    exp 2.02
//	Slide.SetNotes    800 -> 3200 slides    132ms -> 2.14s    exp 2.00
//	AddLayout         400 -> 1600 layouts    66ms -> 1.04s    exp 1.95
//
// A CPU profile of AddSlide put 76% of it in its allocator, a third of the
// total inside fmt.Sprintf.
//
// Cache the taken names and resume the probe where the last allocation
// finished. The name handed out is still the lowest-numbered free one: the
// probe only advances within a cache generation, and nothing frees a name
// without changing one of the two counts below, which rebuilds the cache and
// resets the probe.
//
// Staleness is the number of parts of this kind alone. Handing out a name
// optimistically counts the part the caller is about to create, so a run of
// adds never rebuilds. A caller that does not create it leaves the count one
// too high, which costs a rebuild on the next call — never a wrong answer.
//
// The number of OTHER parts is deliberately not part of that signal, even
// though a rebuild seeds the taken set from them. Doing so made the three
// allocators invalidate each other: SetNotes stores a notes part, which grew
// otherParts, which staled the slide cache, so the next AddSlide rebuilt — and
// a loop adding a slide and its notes stayed quadratic with all three caches
// "working". Instead the candidate is checked against otherParts directly,
// which is a single map lookup, so a part stored since the rebuild is still
// seen. That is the direction that matters: two parts sharing a name makes the
// package unopenable.
type partNameAlloc struct {
	pattern string
	owned   int
	used    map[string]bool
	probe   int
}

// take returns the lowest-numbered free name and advances the probe. A name is
// free when neither the set captured at the last rebuild nor the live
// otherParts map holds it.
//
// It deliberately does not record the name in used: the probe never goes
// backwards within a cache generation, so no later allocation can reach that
// index, and a rebuild repopulates used from the live parts.
func (a *partNameAlloc) take(p *Presentation) string {
	for i := a.probe; ; i++ {
		name := fmt.Sprintf(a.pattern, i)
		if _, taken := p.otherParts[name]; !taken && !a.used[name] {
			a.probe = i + 1
			return name
		}
	}
}

// allocFor returns the cache at *slot, rebuilding it when it no longer
// describes the presentation. ownedNames enumerates the names of parts of this
// kind and is called only on a rebuild.
func (p *Presentation) allocFor(slot **partNameAlloc, pattern string, ownedLen int, ownedNames func(yield func(string))) *partNameAlloc {
	if a := *slot; a != nil && a.owned == ownedLen {
		return a
	}
	p.partNameRebuilds++
	a := &partNameAlloc{
		pattern: pattern,
		owned:   ownedLen,
		used:    make(map[string]bool, ownedLen+len(p.otherParts)),
		probe:   1,
	}
	ownedNames(func(name string) {
		if name != "" {
			a.used[name] = true
		}
	})
	*slot = a
	return a
}

// invalidateSlideNames drops the slide-name cache. Call it wherever a slide's
// part name changes without either count changing, which staleness cannot see.
func (p *Presentation) invalidateSlideNames() {
	p.slideNameCache = nil
}

// nextAvailableSlidePartName returns a slide part name not already in use.
func (p *Presentation) nextAvailableSlidePartName() string {
	a := p.allocFor(&p.slideNameCache, "/ppt/slides/slide%d.xml", len(p.slides), func(yield func(string)) {
		for _, slide := range p.slides {
			if slide != nil {
				yield(slide.partName)
			}
		}
	})
	// Count the slide the caller is about to append.
	a.owned++
	return a.take(p)
}

// nextAvailableLayoutPartName returns a slideLayout part name not already used
// by an existing layout or other part.
func (p *Presentation) nextAvailableLayoutPartName() string {
	a := p.allocFor(&p.layoutNameCache, "/ppt/slideLayouts/slideLayout%d.xml", len(p.slideLayouts), func(yield func(string)) {
		for _, l := range p.slideLayouts {
			if l != nil {
				yield(l.partName)
			}
		}
	})
	a.owned++
	return a.take(p)
}

// nextAvailableNotesName returns a notesSlide part name not already in use.
// Notes parts live in otherParts and have no owning slice of their own, so the
// part the caller is about to store is counted there.
func (p *Presentation) nextAvailableNotesName() string {
	a := p.allocFor(&p.notesNameCache, "/ppt/notesSlides/notesSlide%d.xml", 0, func(func(string)) {})
	return a.take(p)
}
