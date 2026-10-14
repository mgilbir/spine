package main

import (
	_ "embed" // for ui.html
	"fmt"
	"strings"
)

// The embedded single-page UI: Structure, Preview and Console panes plus a
// schema drawer. The page names every operation in the vocabulary so the
// console's skeleton insert can rely on the page even if /schema fails.

//go:embed ui.html
var uiHTMLTemplate string

// uiHTML returns the placeholder UI page with the operation names injected.
func uiHTML() []byte {
	names := make([]string, 0, len(opRegistry))
	for _, op := range opRegistry {
		names = append(names, fmt.Sprintf("%q", op.Name))
	}
	list := "[" + strings.Join(names, ",") + "]"
	return []byte(strings.Replace(uiHTMLTemplate, "__SPINE_OPS__", list, 1))
}
