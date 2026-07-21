package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Scenario is a what-if scenario: a named set of substitute values for a group
// of changing (input) cells on a sheet, managed through Excel's Scenario
// Manager (Data > What-If Analysis > Scenario Manager).
type Scenario struct {
	// Name is the scenario's display name (required, unique within the sheet).
	Name string
	// Comment is the optional free-text comment shown in the Scenario Manager.
	Comment string
	// User is the optional author recorded for the scenario.
	User string
	// Hidden hides the scenario from the Scenario Manager list.
	Hidden bool
	// Locked marks the scenario locked (only meaningful when the sheet is
	// protected with the scenarios option).
	Locked bool
	// Inputs are the changing cells and the values this scenario substitutes.
	Inputs []ScenarioInput
}

// ScenarioInput is one changing cell within a scenario and the value the
// scenario substitutes for it.
type ScenarioInput struct {
	// Cell is the changing cell reference (e.g. "B2").
	Cell string
	// Value is the substitute value, stored verbatim as the cell's value.
	Value string
}

// Scenarios returns the what-if scenarios defined on the sheet, in document
// order. It is read-only; the returned slice is a copy.
func (s *Sheet) Scenarios() []Scenario {
	if s.ws() == nil || s.ws().Scenarios == nil {
		return nil
	}
	out := make([]Scenario, 0, len(s.ws().Scenarios.Scenario))
	for _, sc := range s.ws().Scenarios.Scenario {
		pub := Scenario{
			Name:    sc.Name,
			Comment: sc.Comment,
			User:    sc.User,
			Hidden:  sc.Hidden != nil && *sc.Hidden,
			Locked:  sc.Locked != nil && *sc.Locked,
		}
		for _, ic := range sc.InputCells {
			pub.Inputs = append(pub.Inputs, ScenarioInput{Cell: ic.R, Value: ic.Val})
		}
		out = append(out, pub)
	}
	return out
}

// AddScenario adds a what-if scenario to the sheet. The name must be non-empty
// and unique (case-insensitively) among the sheet's scenarios, and the scenario
// must reference at least one changing cell; each input cell reference is
// validated. Adding a scenario marks the sheet dirty so its worksheet part is
// regenerated on save (the sheet's other scenarios, if any, are re-emitted from
// the typed model rather than their preserved bytes).
func (s *Sheet) AddScenario(sc Scenario) error {
	if strings.TrimSpace(sc.Name) == "" {
		return fmt.Errorf("xlsx: scenario name must not be empty")
	}
	if len(sc.Inputs) == 0 {
		return fmt.Errorf("xlsx: scenario %q must reference at least one changing cell", sc.Name)
	}

	s.ensureWorksheet()
	if s.ws().Scenarios != nil {
		for _, existing := range s.ws().Scenarios.Scenario {
			if strings.EqualFold(existing.Name, sc.Name) {
				return fmt.Errorf("xlsx: scenario %q already exists", sc.Name)
			}
		}
	}

	entry := oxml.CT_Scenario{Name: sc.Name, User: sc.User, Comment: sc.Comment}
	if sc.Hidden {
		v := true
		entry.Hidden = &v
	}
	if sc.Locked {
		v := true
		entry.Locked = &v
	}
	for _, in := range sc.Inputs {
		row, col, err := ParseCellRef(in.Cell)
		if err != nil {
			return fmt.Errorf("xlsx: scenario %q: invalid changing cell %q: %w", sc.Name, in.Cell, err)
		}
		entry.InputCells = append(entry.InputCells, oxml.CT_InputCells{
			R:   FormatCellRef(row, col),
			Val: in.Value,
		})
	}

	if s.ws().Scenarios == nil {
		s.ws().Scenarios = &oxml.CT_Scenarios{}
	}
	scs := s.ws().Scenarios
	scs.Scenario = append(scs.Scenario, entry)
	scs.Dirty = true

	// Point "current"/"show" at the first scenario when the element is being
	// created, matching how Excel initializes a fresh scenarios block.
	if scs.Current == nil {
		zero := uint32(0)
		scs.Current = &zero
	}
	if scs.Show == nil {
		zero := uint32(0)
		scs.Show = &zero
	}
	scs.SqRef = scenarioChangingCells(scs)

	// A parsed sheet marshals by its captured child order; a scenarios element
	// the source lacked must be inserted at its schema position or it would be
	// dropped on save.
	s.ws().EnsureChildOrder("scenarios")
	s.markDirty()
	return nil
}

// scenarioChangingCells returns the space-separated union of every scenario's
// changing-cell references, in first-seen order — the value Excel records in the
// scenarios element's sqref attribute.
func scenarioChangingCells(scs *oxml.CT_Scenarios) string {
	seen := make(map[string]struct{})
	var refs []string
	for i := range scs.Scenario {
		for _, ic := range scs.Scenario[i].InputCells {
			if _, ok := seen[ic.R]; ok {
				continue
			}
			seen[ic.R] = struct{}{}
			refs = append(refs, ic.R)
		}
	}
	return strings.Join(refs, " ")
}
