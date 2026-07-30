package pptx

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// This file is the standing guard for the "mutation-not-flagged" class of bug —
// a public mutator that changes state the save path then ignores, or that
// changes the document without anything recording that it did. Two live
// instances of it (a dozen docx feature mutators that never called touch(), and
// xlsx mutators that reported success on opaque sheets) were found by hand; the
// point of doing it by reflection instead is that a mutator added tomorrow is
// covered by construction rather than by someone remembering to add a case.
//
// The property asserted is: calling a public mutator makes
// Presentation.contentChanged() true. That is the single signal the save reads
// to decide whether the deck was edited, so a mutator that leaves it false
// either loses the edit or, at best, saves the change with a stale
// dcterms:modified.
//
// The registry below is keyed by *type*, not by method: adding a method to a
// type already listed is covered automatically, and adding a whole new public
// type fails receiverProviders' completeness check. Anything genuinely exempt
// goes in mutatorExemptions with a reason, in the spirit of captureExemptAttrs
// in pptx/internal/oxml — a list you have to justify beats silence.

// mutatorNamePrefixes are the verbs this package names its mutators with. A
// method whose name starts with one is treated as a mutator and must either
// flip contentChanged or carry an exemption.
var mutatorNamePrefixes = []string{
	"Set", "Add", "Remove", "Clear", "Delete", "Insert", "Move", "Replace",
	"Embed", "Connect", "Reply", "Resolve", "Show", "Duplicate", "Clone",
	"Sync", "Append", "Extract",
}

// isMutatorName reports whether name is one of those verbs applied to
// something. The prefix has to be followed by an upper-case letter (or be the
// whole name), so the verb-shaped getters — Resolved, Connectors, EmbeddedFonts
// — are not mistaken for mutators.
func isMutatorName(name string) bool {
	for _, p := range mutatorNamePrefixes {
		if !strings.HasPrefix(name, p) {
			continue
		}
		rest := name[len(p):]
		if rest == "" || (rest[0] >= 'A' && rest[0] <= 'Z') {
			return true
		}
	}
	return false
}

// mutatorExemptions lists "Type.Method" entries the reflection driver cannot
// meaningfully exercise, or that are legitimately not document mutations. Every
// entry states why. An entry that stops being needed makes the test fail, so
// the list cannot rot into a mute list.
var mutatorExemptions = map[string]string{
	// Not document mutations.
	"Presentation.EmbedTrueTypeFonts": "getter for the p:presentation flag; the mutator is SetEmbedTrueTypeFonts",
	"Presentation.ExtractSlides":      "builds and returns a NEW deck; the receiver is only read",
	"Table.SyncXML":              "explicit early flush of edits already made; the edits themselves are what flag",
	"Picture.SetImagePath":       "records the label ImagePath returns; documented as not changing the picture (use SetImage)",
	"Animation.SetByParagraph":   "documented no-op on an animation read back from a file, where the handle is not editable",

	// Need a real argument the synthesizer cannot invent. Each is covered by a
	// hand-written case in TestMutatorsNeedingRealArguments below.
	"Slide.AddPicture":              "takes a filesystem path; covered by TestMutatorsNeedingRealArguments",
	"Picture.SetImage":              "takes a filesystem path; covered by TestMutatorsNeedingRealArguments",
	"PlaceholderShape.SetImage":     "takes a filesystem path; covered by TestMutatorsNeedingRealArguments",
	"Slide.AddChart":                "takes a built chart.Chart; covered by TestMutatorsNeedingRealArguments",
	"Slide.RemoveShape":             "must be handed a shape that is on the slide; covered by TestMutatorsNeedingRealArguments",
	"GroupShape.RemoveChild":        "must be handed a child that is in the group; covered by TestMutatorsNeedingRealArguments",
	"Presentation.AppendSlidesFrom": "takes another deck; covered by TestMutatorsNeedingRealArguments",
	"Presentation.ReplaceText":      "marks only when a key matches; covered by TestMutatorsNeedingRealArguments",
	"Slide.ReplaceText":             "marks only when a key matches; covered by TestMutatorsNeedingRealArguments",
	"Slide.ReplaceTextInShape":      "marks only when a key matches; covered by TestMutatorsNeedingRealArguments",

	// Correctly do nothing under the fixture's preconditions.
	"Presentation.RemoveVBAProject":     "no-op on a deck that carries no macros; the removal path is flagged (markPartRemoved)",
	"Presentation.RemoveCustomProperty": "no-op for a name the deck does not define; the hit path is flagged",
}

// receiverProviders yields live instances of every public type that carries
// mutators, from a deck freshly reopened from mutatorFixture. Reaching an
// instance must be a pure read: the driver asserts contentChanged is false
// after navigation, so a getter that marks the deck edited fails here.
var receiverProviders = map[string]func(t *testing.T, p *Presentation) any{
	"Presentation":     func(t *testing.T, p *Presentation) any { return p },
	"Slide":            func(t *testing.T, p *Presentation) any { return p.Slides()[0] },
	"SlideMaster":      func(t *testing.T, p *Presentation) any { return p.SlideMasters()[0] },
	"SlideLayout":      func(t *testing.T, p *Presentation) any { return p.SlideLayouts()[0] },
	"MasterTextStyle":  func(t *testing.T, p *Presentation) any { return p.SlideMasters()[0].TitleStyle() },
	"Section":          func(t *testing.T, p *Presentation) any { return p.Sections()[0] },
	"Comment":          func(t *testing.T, p *Presentation) any { return p.Slides()[0].Comments()[0] },
	"EditablePlaceholder": func(t *testing.T, p *Presentation) any {
		return p.SlideLayouts()[0].EditablePlaceholders()[0]
	},
	"TextBox":          func(t *testing.T, p *Presentation) any { return findShape[*TextBox](t, p) },
	"AutoShape":        func(t *testing.T, p *Presentation) any { return findShape[*AutoShape](t, p) },
	"Picture":          func(t *testing.T, p *Presentation) any { return findShape[*Picture](t, p) },
	"Video":            func(t *testing.T, p *Presentation) any { return p.Slides()[0].AddVideo([]byte("v"), "video/mp4") },
	"Audio":            func(t *testing.T, p *Presentation) any { return p.Slides()[0].AddAudio([]byte("a"), "audio/mpeg") },
	"Table":            func(t *testing.T, p *Presentation) any { return findShape[*Table](t, p) },
	"GroupShape":       func(t *testing.T, p *Presentation) any { return findShape[*GroupShape](t, p) },
	"Connector":        func(t *testing.T, p *Presentation) any { return findShape[*Connector](t, p) },
	"PlaceholderShape": func(t *testing.T, p *Presentation) any { return findShape[*PlaceholderShape](t, p) },
	"TableRow":         func(t *testing.T, p *Presentation) any { return findShape[*Table](t, p).Row(0) },
	"TableCell":        func(t *testing.T, p *Presentation) any { return findShape[*Table](t, p).Cell(0, 0) },
	"TextFrame":        func(t *testing.T, p *Presentation) any { return findShape[*TextBox](t, p).TextFrame() },
	"Paragraph": func(t *testing.T, p *Presentation) any {
		return findShape[*TextBox](t, p).TextFrame().Paragraphs()[0]
	},
	"Run": func(t *testing.T, p *Presentation) any {
		return findShape[*TextBox](t, p).TextFrame().Paragraphs()[0].Runs()[0]
	},
	"Hyperlink": func(t *testing.T, p *Presentation) any {
		return findShape[*TextBox](t, p).TextFrame().Paragraphs()[0].Runs()[0].Hyperlink()
	},
	"Animation": func(t *testing.T, p *Presentation) any {
		return p.Slides()[0].AddAnimation(0, EffectFadeIn, TriggerOnClick)
	},
}

// settleAfterNavigation names receivers whose instance cannot be reached by
// reading: a p:pic carrying media parses back as a *Picture, so a *Video or
// *Audio handle only exists on a shape this session added. For those the driver
// saves once after navigating, which flushes and clears the add, and takes the
// post-save state as the baseline. Every other receiver keeps the strict rule
// that navigating to it must not mark the deck.
var settleAfterNavigation = map[string]string{
	"Video": "no read path yields a *Video: media pictures materialize as *Picture, so the handle must be added",
	"Audio": "no read path yields an *Audio: see Video",
}

func findShape[T Shape](t *testing.T, p *Presentation) T {
	t.Helper()
	for _, s := range p.Slides() {
		for _, sh := range s.Shapes() {
			if got, ok := sh.(T); ok {
				return got
			}
		}
	}
	var zero T
	t.Fatalf("fixture has no %T", zero)
	return zero
}

// mutatorFixture builds a deck carrying one of every shape kind the registry
// navigates to, saves it, and returns the bytes. Reopening those bytes gives
// each subtest a deck whose content is fully parsed and completely unedited.
func mutatorFixture(t *testing.T) []byte {
	t.Helper()
	p := Create()
	s := p.AddSlide()

	tb := s.AddTextBox()
	run := tb.TextFrame().AddParagraph().AddRun()
	run.SetText("text")
	run.SetHyperlink("https://example.com")

	as := NewAutoShape("rect")
	if err := s.AddShape(as); err != nil {
		t.Fatalf("AddShape autoshape: %v", err)
	}
	// A picture placeholder, so the image setters have a legal receiver.
	if err := s.AddShape(NewPlaceholderShape(PlaceholderPicture)); err != nil {
		t.Fatalf("AddShape placeholder: %v", err)
	}
	if _, err := s.AddPictureFromBytes(minimalTransparentPNG, "image/png"); err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	s.AddTable(2, 2).Cell(0, 0).SetText("cell")
	s.AddConnector(ConnectorStraight)
	s.AddVideo([]byte("video-bytes"), "video/mp4")
	s.AddAudio([]byte("audio-bytes"), "audio/mpeg")

	grp := NewGroupShape()
	if err := grp.AddChild(NewTextBox()); err != nil {
		t.Fatalf("group AddChild: %v", err)
	}
	if err := s.AddShape(grp); err != nil {
		t.Fatalf("AddShape group: %v", err)
	}

	s.AddComment("Ada Lovelace", "note")
	p.AddSection("Intro").AddSlide(s)
	// Furniture, so the Clear* furniture mutators have something to clear.
	p.SetSlideFooter("Confidential")
	p.SetSlideDate("2026-07-30")
	p.ShowSlideNumbers(true)
	// A second slide, so slide-list mutators (MoveSlide, RemoveSlide) have
	// somewhere to move to.
	p.AddSlide().AddTextBox().TextFrame().SetText("second")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("fixture SaveBytes: %v", err)
	}
	return data
}

func openFixture(t *testing.T, data []byte) *Presentation {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// TestEveryMutatorMarksTheDeck is the guard. For every public mutator on every
// registered type it opens a fresh fixture, calls the mutator with synthesized
// arguments, and requires contentChanged to go from false to true.
func TestEveryMutatorMarksTheDeck(t *testing.T) {
	fixture := mutatorFixture(t)

	usedExemptions := map[string]bool{}
	var typeNames []string
	for name := range receiverProviders {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		provide := receiverProviders[typeName]
		// One deck just to enumerate the methods of this receiver type.
		probe := openFixture(t, fixture)
		rt := reflect.TypeOf(provide(t, probe))

		var methods []string
		for i := 0; i < rt.NumMethod(); i++ {
			if m := rt.Method(i); isMutatorName(m.Name) {
				methods = append(methods, m.Name)
			}
		}
		sort.Strings(methods)

		for _, methodName := range methods {
			key := typeName + "." + methodName
			if reason, ok := mutatorExemptions[key]; ok {
				if reason == "" {
					t.Errorf("%s: exemption with no reason", key)
				}
				usedExemptions[key] = true
				continue
			}
			t.Run(key, func(t *testing.T) {
				// Two argument seeds, each on its own deck. Scalars are drawn
				// from an ascending sequence, so seed 0 gives a (from, to) pair
				// that is a real move while seed 1 gives an enum value that is
				// not the current one — setters that legitimately no-op when
				// handed the value already in place (SetPlayMode) would
				// otherwise read as unflagged.
				for _, seed := range []int{0, 1} {
					p := openFixture(t, fixture)
					recv := reflect.ValueOf(provide(t, p))
					if _, settle := settleAfterNavigation[typeName]; settle {
						if _, err := p.SaveBytes(); err != nil {
							t.Fatalf("settling save: %v", err)
						}
					}
					if p.contentChanged() {
						t.Fatalf("navigating to the receiver already marked the deck edited; that is a read path marking, not %s", key)
					}
					method := recv.MethodByName(methodName)
					args, err := synthesizeArgs(method.Type(), p, seed)
					if err != nil {
						t.Fatalf("cannot build arguments for %s: %v\n"+
							"add a provider to synthesizeArg, or an entry to mutatorExemptions saying why this one cannot be driven", key, err)
					}
					callMutator(t, key, method, args)
					if p.contentChanged() {
						return
					}
				}
				t.Errorf("%s changed state without marking the deck edited: the save cannot tell the deck was modified, "+
					"so the change is at best saved with a stale dcterms:modified and at worst dropped", key)
			})
		}
	}

	for key := range mutatorExemptions {
		if !usedExemptions[key] {
			t.Errorf("stale exemption %q: no such mutator (or it is no longer named like one) — delete the entry", key)
		}
	}
}

// callMutator invokes the method, turning a panic into a test failure that says
// what to do about it rather than taking the whole run down.
func callMutator(t *testing.T, key string, method reflect.Value, args []reflect.Value) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked on synthesized arguments (%v); give it a better argument provider or an exemption", key, r)
		}
	}()
	if method.Type().IsVariadic() {
		method.CallSlice(args)
		return
	}
	method.Call(args)
}

// synthesizeArgs builds one argument per parameter. Integers are given
// ascending values so a (from, to) pair is never a no-op self-move.
func synthesizeArgs(ft reflect.Type, p *Presentation, seed int) ([]reflect.Value, error) {
	n := ft.NumIn()
	args := make([]reflect.Value, 0, n)
	intSeq := seed
	for i := 0; i < n; i++ {
		pt := ft.In(i)
		if ft.IsVariadic() && i == n-1 {
			// CallSlice wants the variadic parameter as a slice; an empty one is
			// the "no extra arguments" call.
			args = append(args, reflect.MakeSlice(pt, 0, 0))
			continue
		}
		v, err := synthesizeArg(pt, p, &intSeq)
		if err != nil {
			return nil, fmt.Errorf("parameter %d (%s): %w", i, pt, err)
		}
		args = append(args, v)
	}
	return args, nil
}

func synthesizeArg(pt reflect.Type, p *Presentation, intSeq *int) (reflect.Value, error) {
	// Types that need a live object out of the deck, or a concrete
	// implementation of an interface.
	switch pt {
	case reflect.TypeOf((*Slide)(nil)):
		return reflect.ValueOf(p.Slides()[0]), nil
	case reflect.TypeOf((*SlideLayout)(nil)):
		return reflect.ValueOf(p.SlideLayouts()[0]), nil
	case reflect.TypeOf((*Section)(nil)):
		return reflect.ValueOf(p.Sections()[0]), nil
	case reflect.TypeOf((*Presentation)(nil)):
		return reflect.ValueOf(Create()), nil
	case reflect.TypeOf((*Shape)(nil)).Elem():
		return reflect.ValueOf(Shape(NewTextBox())), nil
	case reflect.TypeOf(dml.Fill{}):
		return reflect.ValueOf(dml.NewSolidFill(dml.ColorRed)), nil
	}
	if pt.Kind() == reflect.Interface && pt.NumMethod() == 0 {
		// A bare `any` parameter (SetCustomProperty's value).
		v := reflect.New(pt).Elem()
		v.Set(reflect.ValueOf("guard"))
		return v, nil
	}

	switch pt.Kind() {
	case reflect.String:
		return reflect.ValueOf("guard").Convert(pt), nil
	case reflect.Bool:
		return reflect.ValueOf(true).Convert(pt), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := *intSeq
		*intSeq++
		return reflect.ValueOf(int64(v)).Convert(pt), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := *intSeq
		*intSeq++
		return reflect.ValueOf(uint64(v)).Convert(pt), nil
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(12.0).Convert(pt), nil
	case reflect.Slice:
		if pt.Elem().Kind() == reflect.Uint8 {
			return reflect.ValueOf(minimalTransparentPNG).Convert(pt), nil
		}
		elem, err := synthesizeArg(pt.Elem(), p, intSeq)
		if err != nil {
			return reflect.Value{}, err
		}
		s := reflect.MakeSlice(pt, 1, 1)
		s.Index(0).Set(elem)
		return s, nil
	case reflect.Map:
		key, err := synthesizeArg(pt.Key(), p, intSeq)
		if err != nil {
			return reflect.Value{}, err
		}
		val, err := synthesizeArg(pt.Elem(), p, intSeq)
		if err != nil {
			return reflect.Value{}, err
		}
		m := reflect.MakeMap(pt)
		m.SetMapIndex(key, val)
		return m, nil
	case reflect.Struct:
		return reflect.New(pt).Elem(), nil
	case reflect.Pointer:
		if pt.Elem().Kind() == reflect.Struct {
			return reflect.New(pt.Elem()), nil
		}
	}
	return reflect.Value{}, fmt.Errorf("no way to build a %s", pt)
}

// TestMutatorsNeedingRealArguments covers the mutators the reflection driver
// exempts because it cannot invent their arguments. Same property, by hand.
func TestMutatorsNeedingRealArguments(t *testing.T) {
	fixture := mutatorFixture(t)

	cases := []struct {
		name  string
		apply func(t *testing.T, p *Presentation)
	}{
		{"Slide.RemoveShape", func(t *testing.T, p *Presentation) {
			s := p.Slides()[0]
			s.RemoveShape(s.Shapes()[0])
		}},
		{"GroupShape.RemoveChild", func(t *testing.T, p *Presentation) {
			g := findShape[*GroupShape](t, p)
			g.RemoveChild(g.Children()[0])
		}},
		{"Presentation.AppendSlidesFrom", func(t *testing.T, p *Presentation) {
			other := Create()
			other.AddSlide().AddTextBox().TextFrame().SetText("imported")
			if err := p.AppendSlidesFrom(other); err != nil {
				t.Fatalf("AppendSlidesFrom: %v", err)
			}
		}},
		{"Presentation.ReplaceText", func(t *testing.T, p *Presentation) {
			p.ReplaceText(map[string]string{"text": "replaced"})
		}},
		{"Slide.ReplaceText", func(t *testing.T, p *Presentation) {
			p.Slides()[0].ReplaceText(map[string]string{"text": "replaced"})
		}},
		{"Slide.ReplaceTextInShape", func(t *testing.T, p *Presentation) {
			tb := findShape[*TextBox](t, p)
			p.Slides()[0].ReplaceTextInShape(tb.Name(), map[string]string{"text": "replaced"})
		}},
		{"Slide.AddPicture", func(t *testing.T, p *Presentation) {
			// AddPicture is SetImage plus addShape; the filesystem read is the
			// only part the driver cannot supply, and AddPictureFromBytes
			// exercises the same flagging path.
			if _, err := p.Slides()[0].AddPictureFromBytes(minimalTransparentPNG, "image/png"); err != nil {
				t.Fatalf("AddPictureFromBytes: %v", err)
			}
		}},
		{"Picture.SetImage", func(t *testing.T, p *Presentation) {
			// SetImage is SetImageData plus a file read.
			findShape[*Picture](t, p).SetImageData(minimalTransparentPNG, "image/png")
		}},
		{"PlaceholderShape.SetImage", func(t *testing.T, p *Presentation) {
			ph := findShape[*PlaceholderShape](t, p)
			// SetImageData is SetImage without the file read.
			if err := ph.SetImageData(minimalTransparentPNG, "image/png"); err != nil {
				t.Fatalf("SetImageData: %v", err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := openFixture(t, fixture)
			// Materialize before the baseline check so the read itself is not
			// what the assertion measures.
			for _, s := range p.Slides() {
				_ = s.Shapes()
			}
			if p.contentChanged() {
				t.Fatal("the fixture reports itself edited before the mutator ran")
			}
			tc.apply(t, p)
			if !p.contentChanged() {
				t.Errorf("%s changed state without marking the deck edited", tc.name)
			}
		})
	}
}

// TestPlaceholderTypeIsNotAMutator documents the one shape of "mutator" this
// package deliberately hands out with no effect on the document: the detached
// placeholder copies SlideMaster.Placeholders and SlideLayout.Placeholders
// return. Their setters mark an orphan, so nothing reaches the part. The
// supported path is EditablePlaceholder, which the reflection guard drives.
//
// This is not an exemption in the list above (Placeholders is a getter, so the
// driver never sees it); it is here so the trap is pinned rather than only
// described in a doc comment.
func TestPlaceholderTypeIsNotAMutator(t *testing.T) {
	fixture := mutatorFixture(t)
	p := openFixture(t, fixture)

	phs := p.SlideLayouts()[0].Placeholders()
	if len(phs) == 0 {
		t.Skip("fixture layout has no placeholders")
	}
	phs[0].SetName("detached")
	if p.contentChanged() {
		t.Error("SlideLayout.Placeholders now returns live handles; make it an EditablePlaceholder alias and drop this test")
	}
}
