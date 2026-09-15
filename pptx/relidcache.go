package pptx

import "github.com/mgilbir/spine/opc"

// relIDMaxEntry caches the highest relationship id number in one part's
// relationship scope, along with how many relationships it was computed from.
//
// nextRelIDNum scanned the whole scope on every call. AddLayout calls it and
// then adds a relationship to that same scope, so adding layouts was quadratic
// even after the fmt.Sscanf parsing cost was removed: 1600 layouts 32ms, 6400
// layouts 464ms, a growth exponent of 1.94.
//
// This is the id space C363 came out of, so the cache is built to fail in the
// safe direction only. A maximum that is too HIGH skips an id, which costs
// nothing — ids need only be unique. A maximum that is too LOW hands out an id
// another relationship already holds, which is the C363 defect: the save keeps
// one relationship and drops the other as a duplicate, leaving a reference that
// resolves to the wrong part. So:
//
//   - The recorded count is compared against the live slice on every read; any
//     relationship added or removed without maintaining the cache rescans.
//   - Maintenance only ever raises the maximum, never lowers it.
//   - The rescan delegates to nextRelationshipID, so the cached value is by
//     construction the number that function would have returned.
//
// Relationship ids are never rewritten in place anywhere in the package, so the
// count is a sufficient signal for a change the cache must notice.
//
// This maximum and SlideMaster.maxLayoutRelID cover the same allocation from
// two directions and are mutually redundant: a layout's id lands both in
// sm.layouts and in the master's relationship scope, so either maximum alone
// still prevents a reused id. Removing both is what breaks it. The redundancy
// is deliberate given what a reused id costs here, but it does mean neither can
// be exercised on its own.
type relIDMaxEntry struct {
	max   int
	count int
}

// maxRelIDFor returns the highest relationship id number in partName's scope.
func (p *Presentation) maxRelIDFor(partName string) int {
	rels := p.relationships[partName]
	if e, ok := p.relIDMax[partName]; ok && e.count == len(rels) {
		return e.max
	}
	p.relIDRescans++
	max := nextRelationshipID(rels) - 1
	if p.relIDMax == nil {
		p.relIDMax = make(map[string]relIDMaxEntry)
	}
	p.relIDMax[partName] = relIDMaxEntry{max: max, count: len(rels)}
	return max
}

// appendRelationship adds rel to partName's scope and keeps the cached maximum
// in step, so a loop that allocates an id and then registers the relationship
// does not rescan the scope on every pass. Prefer it over appending to
// p.relationships directly; appending directly is still correct, it just costs
// a rescan on the next read.
func (p *Presentation) appendRelationship(partName string, rel *opc.Relationship) {
	p.relationships[partName] = append(p.relationships[partName], rel)
	e, ok := p.relIDMax[partName]
	if !ok {
		// Nothing cached for this scope yet; the next read builds it from the
		// live slice, which now includes rel.
		return
	}
	if id, parsed := relIDNum(rel.ID); parsed && id > e.max {
		e.max = id
	}
	e.count = len(p.relationships[partName])
	p.relIDMax[partName] = e
}
