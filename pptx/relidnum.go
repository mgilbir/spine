package pptx

// relIDNum parses the numeric part of a relationship id ("rId12" -> 12).
//
// It replaces fmt.Sscanf(id, "rId%d", &n) in the allocators, which reach it
// once per existing relationship. Sscanf is reflection- and interface-driven:
// a CPU profile of AddLayout put 69% of the whole operation inside it, and 76%
// in the allocator that calls it. Parsing the digits directly is the same
// answer for a fraction of the cost.
//
// The semantics match Sscanf's deliberately, including the parts that look
// wrong: a leading sign is accepted (%d does), and trailing junk is ignored
// ("rId12abc" yields 12, because Sscanf does not require the whole input to be
// consumed). relidnum_test.go asserts the equivalence against fmt.Sscanf itself
// over a table of awkward inputs rather than trusting this comment.
func relIDNum(id string) (int, bool) {
	const prefix = "rId"
	if len(id) < len(prefix) || id[:len(prefix)] != prefix {
		return 0, false
	}
	rest := id[len(prefix):]
	// %d skips leading whitespace, so "rId 1" parses as 1. Matching that is not
	// pedantry: a wild file carrying such an id would otherwise be invisible to
	// the allocator, which would then hand the same number out again.
	for rest != "" && (rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n' || rest[0] == '\r' || rest[0] == '\v' || rest[0] == '\f') {
		rest = rest[1:]
	}
	if rest == "" {
		return 0, false
	}
	neg := false
	if rest[0] == '+' || rest[0] == '-' {
		neg = rest[0] == '-'
		rest = rest[1:]
	}
	if rest == "" || rest[0] < '0' || rest[0] > '9' {
		return 0, false
	}
	n := 0
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c < '0' || c > '9' {
			break // trailing junk, as Sscanf ignores it
		}
		n = n*10 + int(c-'0')
		if n > 1<<30 {
			// Far past any real relationship count. Sscanf would keep going and
			// overflow; refusing is the safe difference, and the test pins that
			// no realistic id reaches here.
			return 0, false
		}
	}
	if neg {
		n = -n
	}
	return n, true
}
