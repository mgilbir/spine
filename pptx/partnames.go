package pptx

import "fmt"

// slideNameAlloc caches the set of part names already taken, so allocating a
// slide part name does not rebuild it on every call.
//
// nextAvailableSlidePartName built a map of every slide part name and every
// other part name, then probed "/ppt/slides/slide1.xml", "slide2.xml", ...
// from the start until one was free — formatting a string per probe step. Both
// halves are O(slides), and it runs once per AddSlide, so building a deck cost
// O(slides^2): 500 slides took 17.1ms and 2000 took 280ms, a growth exponent
// of 2.02. A CPU profile puts 76% of AddSlide in this one function, with a
// third of the total in fmt.Sprintf; nextPresentationRelID, the other
// per-add scan, does not appear.
//
// Caching the taken set and resuming the probe where the last allocation
// finished makes an add O(1). The probe hint never skips a free name: it only
// advances across allocations, and nothing frees a name without changing the
// number of slides or other parts, which rebuilds the cache and resets the
// probe to 1. So the name handed out is still the lowest-numbered free one,
// exactly as before.
type slideNameAlloc struct {
	slides int
	others int
	used   map[string]bool
	probe  int
}

// slideNames returns the cache, rebuilding it when it no longer describes the
// presentation's slides and other parts.
func (p *Presentation) slideNames() *slideNameAlloc {
	if a := p.slideNameCache; a != nil && a.slides == len(p.slides) && a.others == len(p.otherParts) {
		return a
	}
	p.slideNameRebuilds++
	a := &slideNameAlloc{
		slides: len(p.slides),
		others: len(p.otherParts),
		used:   make(map[string]bool, len(p.slides)+len(p.otherParts)),
		probe:  1,
	}
	for _, slide := range p.slides {
		if slide != nil && slide.partName != "" {
			a.used[slide.partName] = true
		}
	}
	for name := range p.otherParts {
		a.used[name] = true
	}
	p.slideNameCache = a
	return a
}

// invalidateSlideNames drops the cache. Call it wherever a slide's part name
// changes without the number of slides or other parts changing, which the
// staleness check cannot see.
func (p *Presentation) invalidateSlideNames() {
	p.slideNameCache = nil
}

// noteSlideAppended records that the slide holding the most recently allocated
// name now exists. Without it the next allocation sees a slide count that has
// moved and rebuilds the whole set, which is the cost this cache removes.
func (p *Presentation) noteSlideAppended() {
	if a := p.slideNameCache; a != nil {
		a.slides = len(p.slides)
	}
}

// nextAvailableSlidePartName returns the lowest-numbered slide part name not
// already in use, and records it as taken.
func (p *Presentation) nextAvailableSlidePartName() string {
	a := p.slideNames()
	for i := a.probe; ; i++ {
		name := fmt.Sprintf("/ppt/slides/slide%d.xml", i)
		if !a.used[name] {
			// Only the probe is advanced. Marking the name in used as well
			// would be dead: the probe never goes backwards within a cache
			// generation, so no later allocation can reach this index again,
			// and a rebuild repopulates used from the live slides and parts
			// anyway. used exists to skip names taken by something other than
			// this allocator — an other part, or a slide loaded from a file.
			a.probe = i + 1
			return name
		}
	}
}
