# Exploratory sweeps

How to look for a new class of defect with syntactic matchers before deciding
whether it deserves a guard, and what those tools cannot see. This is
maintainer methodology; the guards themselves, and the rules for writing one,
are in [CONTRIBUTING § Mechanical guards](../../CONTRIBUTING.md#mechanical-guards).

`ast-grep` is a good way to *find* a new class before you know whether it is
real. Rules are quick to write and run without any project setup:

```
ast-grep scan --inline-rules '...' xlsx/          # ad-hoc
ast-grep scan -r some-rule.yml xlsx/              # from a file
```

There is deliberately no `sgconfig.yml` and no committed rule set, and ast-grep
is not wired into CI. A rule that lives outside the test suite is a guard nobody
watches, and every invariant currently worth enforcing is already a Go test that
runs on every push — `TestSheetDirtyIsOnlySetByMarkDirty` covers the same ground
as the obvious ast-grep rule and got the Go `method_declaration` /
`function_declaration` distinction right, which the first draft of that rule did
not: it silently excluded nothing and reported a false positive. The same slip in
the other direction reports "clean" forever.

`semgrep` is the heavier option and earns its weight on one specific kind of
question. Both tools are effectively intra-procedural, so neither can answer
"this getter reads a field the materializer never assigns" — that spans two
functions. But semgrep can express **negation and helper-call awareness within
one function**, which ast-grep's relational rules cannot, and that is often
enough once the question is restated.

That restatement is the transferable part. The getter-versus-materializer
property was undecidable as posed; rewritten as *a materializer that reads the
parsed node's `SpPr` but never mirrors it into the overlay* it fits inside a
single function, and a rule with one `pattern` plus three `pattern-not`s — the
third covering the helper form `$HELPER(&$DST.spPr, ...)` — retro-found both
buggy materializers behind C599. Two false positives came with them, both
dismissed in seconds because the types had no overlay field at all.

**When a cross-function property blocks you, look for an equivalent
single-function restatement.** That is what turned "cannot be automated" into two
true positives.

Know the limits before reaching for either:

- ast-grep matched C599's eight buggy methods only by their *formatting* —
  one-line delegating getters. `Connector.line()` is correct and has a body, so
  it did not match; reformat either and the answer flips in both directions. It
  found the right set for the wrong reason.
- Some questions are not in the Go source at all. Whether a
  `RawAttrListOverride` call site is safe depends on whether the attribute is
  *schema-required*, which no rule over the code can see. There the guard does
  not analyse — it forces the decision to be written down per call site and
  fails when a new site appears undeclared.

So: sweep with ast-grep, reach for semgrep when the question needs negation,
then promote anything real to a Go test. Prefer
`go/ast` and `go/types` for the promoted version — they are in the standard
library (plus `golang.org/x/tools/go/packages` to load them, as
`internal/maporder` does), and the questions that
matter here ("does this method transitively reach `markDirty`?", "are these map
keys sorted before use?") need types and a call graph, which a syntactic matcher
cannot answer.
