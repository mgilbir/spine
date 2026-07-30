package pptx

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// This file is the standing guard for the "getter cannot see the file" class of
// bug — a public getter that reports one thing on the handle you just wrote to
// and something else (usually nil) on the same object read back from the saved
// deck.
//
// The live instance was AutoShape.Glow/Reflection/SoftEdge/Bevel: a shape's
// SpPr on the domain handle is an overlay that starts empty for a materialized
// shape and is merged onto the parsed node at save, so the effects were always
// written correctly — but the materializer never carried the parsed effects
// back into the overlay, and the getters read the overlay. Every shape read
// from a file therefore answered Glow() == nil while the glow sat in its XML,
// which inverts the most natural use of the getter:
//
//	if s.Glow() == nil { s.SetGlow(...) }   // fired on shapes that already had one
//
// Every existing test missed it for the same reason: they set an effect and
// read it back on the *same in-memory handle*, where the setter has already
// populated the overlay. That assertion passes whether or not parsing works at
// all. So this guard deliberately goes through save → reopen; an assertion on
// the authoring handle would reproduce the exact blind spot it exists to
// remove.
//
// The property asserted per setter/getter pair is:
//
//	write it, save, reopen, navigate back, read it — you get what you wrote.
//
// Stated as "the reopened deck agrees with the authoring handle", so the guard
// does not need to know each getter's normalization: it compares the getter
// against itself across the round trip.
//
// The subject list is derived from the source by reflection over the same
// receiver registry TestEveryMutatorMarksTheDeck uses, so a setter added
// tomorrow is covered by construction. Anything that legitimately does not
// round-trip goes in setterRoundTripExemptions with a reason, and an exemption
// that stops being needed fails the test — the point of the guard is that the
// known asymmetries are written down in one list instead of being discovered
// one at a time.

// isSetterName reports whether name is a "SetX" mutator whose paired getter
// would be "X". The "Set" has to be followed by an upper-case letter so
// Settings-shaped names are not caught.
func isSetterName(name string) bool {
	return strings.HasPrefix(name, "Set") && len(name) > 3 && name[3] >= 'A' && name[3] <= 'Z'
}

// setterRoundTripReceiverExemptions lists receivers from receiverProviders that
// this guard cannot drive, with the reason. Checked for staleness against
// receiverProviders.
var setterRoundTripReceiverExemptions = map[string]string{
	"Video": "no read path yields a *Video: a p:pic carrying media parses back as a *Picture, so there is no reopened handle to navigate to",
	"Audio": "no read path yields an *Audio: see Video",
}

// setterRoundTripProviders overrides receiverProviders for receivers whose
// mutator-test provider *adds* the object (which would add a second one after
// the reopen instead of finding the first). Round-trip navigation has to be a
// pure read on both sides of the save.
var setterRoundTripProviders = map[string]func(t *testing.T, p *Presentation) any{
	"Animation": func(t *testing.T, p *Presentation) any {
		t.Helper()
		anims := p.Slides()[0].Animations()
		if len(anims) == 0 {
			t.Fatal("fixture slide carries no animation")
		}
		return anims[0]
	},
}

// setterRoundTripExemptions lists "Type.SetX" pairs that cannot be driven, or
// that legitimately do not survive a round trip. Every entry says why. An entry
// no longer matching a real setter/getter pair fails the test, so the list
// cannot rot into a mute list.
//
// The value of writing these down is that the asymmetries of the API are
// visible in one place: a caller reading this list learns which setters do not
// answer through their getter, instead of finding out one at a time.
var setterRoundTripExemptions = map[string]string{
	// Deliberately not persisted.
	"Picture.SetImagePath":     "deprecated label setter: it records the string ImagePath returns and nothing on the save path reads it, so the deck is unchanged by design",
	"Picture.SetImageData":     "ImageData reports the *pending* bytes an unsaved edit is carrying; once the deck is saved the image lives in a media part and the field is empty again, so a reopened picture reads nil by construction. Picture.Data is the file-aware reader (it falls back to the media part) and is what a caller should use",
	"Animation.SetByParagraph": "documented no-op on an animation read back from a file: only animations this session added are serialized from the handle, so the flag cannot reach the timing tree",

	// Need an argument the driver cannot invent.
	"Presentation.SetCustomShows": "each entry names slide relationship ids that must already exist in presentation.xml.rels; a synthesized id fails validation with dangling-rel. Covered by TestCustomShowsReadWrite",
}

// setterRoundTripFixture is mutatorFixture plus an animation, so the Animation
// receiver can be reached by reading on both sides of the save rather than by
// adding one. The animation has to name a resolved shape id — one added against
// id 0 is dropped at save — which is why this builds on an already-saved deck.
func setterRoundTripFixture(t *testing.T) []byte {
	t.Helper()
	p := openFixture(t, mutatorFixture(t))
	s := p.Slides()[0]
	shapes := s.Shapes()
	if len(shapes) == 0 {
		t.Fatal("fixture slide has no shapes to animate")
	}
	s.AddAnimation(shapes[0].ID(), EffectFadeIn, TriggerOnClick)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("fixture SaveBytes: %v", err)
	}
	return data
}

// TestEverySetterSurvivesSaveAndReopen is the guard. For every public setter
// with a paired getter, on every registered receiver, it writes a value, saves,
// reopens, navigates back to the same object and requires the getter to report
// what the authoring handle reported.
func TestEverySetterSurvivesSaveAndReopen(t *testing.T) {
	fixture := setterRoundTripFixture(t)

	for name, reason := range setterRoundTripReceiverExemptions {
		if reason == "" {
			t.Errorf("receiver exemption %q has no reason", name)
		}
		if _, ok := receiverProviders[name]; !ok {
			t.Errorf("stale receiver exemption %q: receiverProviders has no such type — delete the entry", name)
		}
	}
	for name := range setterRoundTripProviders {
		if _, ok := receiverProviders[name]; !ok {
			t.Errorf("stale provider override %q: receiverProviders has no such type — delete the entry", name)
		}
	}

	var typeNames []string
	for name := range receiverProviders {
		if _, exempt := setterRoundTripReceiverExemptions[name]; exempt {
			continue
		}
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	usedExemptions := map[string]bool{}
	pairs, unpaired := 0, 0

	for _, typeName := range typeNames {
		provide := receiverProviders[typeName]
		if override, ok := setterRoundTripProviders[typeName]; ok {
			provide = override
		}
		// One deck just to enumerate this receiver's methods.
		probe := openFixture(t, fixture)
		rt := reflect.TypeOf(provide(t, probe))

		var setters []string
		for i := 0; i < rt.NumMethod(); i++ {
			if m := rt.Method(i); isSetterName(m.Name) {
				setters = append(setters, m.Name)
			}
		}
		sort.Strings(setters)

		for _, setterName := range setters {
			getterName := setterName[len("Set"):]
			key := typeName + "." + setterName
			setter, _ := rt.MethodByName(setterName)
			getter, ok := rt.MethodByName(getterName)
			if !ok || !getterPairsWith(getter.Type, setter.Type) {
				// A setter with no readable counterpart cannot report a wrong
				// answer, so it is out of scope rather than exempt.
				unpaired++
				continue
			}
			pairs++
			if reason, exempt := setterRoundTripExemptions[key]; exempt {
				if reason == "" {
					t.Errorf("%s: exemption with no reason", key)
				}
				usedExemptions[key] = true
				continue
			}
			t.Run(key, func(t *testing.T) {
				driveSetterRoundTrip(t, fixture, typeName, setterName, getterName, provide)
			})
		}
	}

	for key := range setterRoundTripExemptions {
		if !usedExemptions[key] {
			t.Errorf("stale exemption %q: no such setter/getter pair (or it is no longer paired) — delete the entry", key)
		}
	}

	t.Logf("drove %d setter/getter pairs (%d exempt); %d setters have no paired getter",
		pairs-len(usedExemptions), len(usedExemptions), unpaired)
}

// getterPairsWith reports whether getter can be called with a prefix of the
// setter's arguments. Index-style setters (SetLevelBold(level, bold)) pair with
// index-style getters (LevelBold(level)); a same-named method taking unrelated
// arguments is not a pair.
func getterPairsWith(getter, setter reflect.Type) bool {
	if getter.NumOut() == 0 || getter.NumIn() > setter.NumIn() || getter.IsVariadic() {
		return false
	}
	for i := 1; i < getter.NumIn(); i++ { // 0 is the receiver on both
		if getter.In(i) != setter.In(i) {
			return false
		}
	}
	return true
}

// driveSetterRoundTrip runs the write → save → reopen → read cycle for one
// pair. It tries two argument seeds, because a setter handed the value already
// in place legitimately leaves the getter unmoved and would make the round-trip
// assertion vacuous.
func driveSetterRoundTrip(t *testing.T, fixture []byte, typeName, setterName, getterName string, provide func(*testing.T, *Presentation) any) {
	t.Helper()
	key := typeName + "." + setterName

	var lastPristine, lastWritten string
	for seed := 0; seed < 2; seed++ {
		p := openFixture(t, fixture)
		recv := reflect.ValueOf(provide(t, p))
		if p.contentChanged() {
			t.Fatalf("navigating to the receiver already marked the deck edited; that is a read path marking, not %s", key)
		}

		setter := recv.MethodByName(setterName)
		gen := &argGen{seed: seed, deck: p, t: t}
		args, err := gen.args(setter.Type())
		if err != nil {
			t.Fatalf("cannot build arguments for %s: %v\n"+
				"add a sample for the type, or an entry to setterRoundTripExemptions saying why this pair cannot be driven", key, err)
		}

		getter := recv.MethodByName(getterName)
		getterArgs := args[:getter.Type().NumIn()]
		pristine := fingerprintCall(t, key, getter, getterArgs)

		callSetter(t, key, setter, args)
		written := fingerprintCall(t, key, getter, getterArgs)
		lastPristine, lastWritten = pristine, written
		if written == pristine {
			// The value was already in place (or the setter does not feed this
			// getter). Try the other seed before concluding anything.
			continue
		}

		saved, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("%s: SaveBytes after the setter: %v\n"+
				"if the synthesized value cannot legally be saved, exempt the pair with that reason", key, err)
		}
		reopened := openFixture(t, saved)
		back := reflect.ValueOf(provide(t, reopened))
		// The getter arguments carry over unchanged: only index-style leading
		// parameters reach a getter, and those are plain scalars.
		got := fingerprintCall(t, key, back.MethodByName(getterName), getterArgs)
		if got != written {
			t.Errorf("%s: the getter cannot see what was written to the file.\n"+
				"  on the authoring handle: %s\n"+
				"  after save and reopen:   %s\n"+
				"(before the setter ran it read %s)\n"+
				"Either the value never reached the part, or the reopened object never carries it back to what the getter reads. "+
				"If the difference is a documented normalization, declare it in setterRoundTripExemptions with that reason.",
				key, written, got, pristine)
		}
		return
	}

	t.Errorf("%s: the setter does not move its getter even on the authoring handle "+
		"(both argument seeds left it reading %s; the last write reported %s).\n"+
		"Either the pair is not really a pair, or the property is not readable back — declare it in setterRoundTripExemptions with the reason.",
		key, lastPristine, lastWritten)
}

// callSetter invokes the setter, turning a panic into a failure that says what
// to do about it rather than taking the whole run down.
func callSetter(t *testing.T, key string, setter reflect.Value, args []reflect.Value) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s panicked on the synthesized value (%v); give its argument type a sample, or exempt the pair", key, r)
		}
	}()
	if setter.Type().IsVariadic() {
		setter.CallSlice(args)
		return
	}
	setter.Call(args)
}

func fingerprintCall(t *testing.T, key string, getter reflect.Value, args []reflect.Value) string {
	t.Helper()
	var out []reflect.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s: the getter panicked (%v)", key, r)
			}
		}()
		out = getter.Call(args)
	}()
	parts := make([]string, 0, len(out))
	for _, v := range out {
		parts = append(parts, fingerprint(v))
	}
	return strings.Join(parts, ", ")
}

// --- value fingerprints ---

// fidelityFields are round-trip capture fields: verbatim source attributes,
// empty-tag styles and the like. They are deliberately different between a
// programmatically built value and the same value parsed back from XML, and
// they carry no semantics a getter's caller can observe, so the fingerprint
// ignores them.
var fidelityFields = map[string]bool{
	"CapturedAttrs":     true,
	"CapturedEmptyTag":  true,
	"OriginalNSDecls":   true,
	"OriginalRootAttrs": true,
	"ChildOrder":        true,
	"UnknownChildren":   true,
	"InlineNSDecls":     true,
	"ExtAttrs":          true,
}

// fingerprintOverrides render values whose exported shape says nothing useful:
// domain handles (all-unexported, and pointing into their own deck, so no
// cross-deck structural comparison is possible) and opaque value types.
var fingerprintOverrides = map[reflect.Type]func(v reflect.Value) string{
	reflect.TypeOf((*Hyperlink)(nil)): func(v reflect.Value) string {
		h := v.Interface().(*Hyperlink)
		return fmt.Sprintf("hyperlink(url=%q anchor=%q tooltip=%q)", h.URL(), h.Anchor(), h.Tooltip())
	},
	reflect.TypeOf(dml.Fill{}): func(v reflect.Value) string {
		return fmt.Sprintf("fill(type=%v)", v.Interface().(dml.Fill).Type())
	},
}

// fingerprint renders a getter result as a string that is comparable across
// two decks. Unexported fields are skipped: a domain handle's internals point
// at its own deck, so they can never compare equal across a save, and anything
// a caller can actually observe is reachable through the exported surface or
// through an override above. A struct with no observable surface renders as
// "<opaque>", which fails the "the setter moved its getter" precondition and so
// forces an override or an exemption rather than passing silently.
func fingerprint(v reflect.Value) string {
	if !v.IsValid() {
		return "<invalid>"
	}
	if fn, ok := fingerprintOverrides[v.Type()]; ok {
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return "nil"
		}
		return fn(v)
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return "nil"
		}
		return fingerprint(v.Elem())
	case reflect.Struct:
		if err, ok := v.Interface().(error); ok && err != nil {
			return "error(" + err.Error() + ")"
		}
		var parts []string
		rt := v.Type()
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			if !f.IsExported() || fidelityFields[f.Name] {
				continue
			}
			parts = append(parts, f.Name+"="+fingerprint(v.Field(i)))
		}
		if len(parts) == 0 {
			return "<opaque " + rt.String() + ">"
		}
		return "{" + strings.Join(parts, " ") + "}"
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() {
			return "nil"
		}
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return fmt.Sprintf("bytes(len=%d %x)", v.Len(), firstBytes(v))
		}
		parts := make([]string, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			parts = append(parts, fingerprint(v.Index(i)))
		}
		return "[" + strings.Join(parts, " ") + "]"
	case reflect.Map:
		if v.IsNil() {
			return "nil"
		}
		parts := make([]string, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			parts = append(parts, fingerprint(iter.Key())+":"+fingerprint(iter.Value()))
		}
		sort.Strings(parts)
		return "map[" + strings.Join(parts, " ") + "]"
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", v.Float())
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return "<" + v.Kind().String() + ">"
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}

func firstBytes(v reflect.Value) []byte {
	n := v.Len()
	if n > 8 {
		n = 8
	}
	b := make([]byte, n)
	reflect.Copy(reflect.ValueOf(b), v.Slice(0, n))
	return b
}

// --- argument synthesis ---

// typeSamples supplies values for types whose valid values cannot be invented:
// enumerations, and structs whose fields are interdependent. Two per type, so
// the second seed writes something different from the first — a setter handed
// the value already in place would otherwise read as "does not move its
// getter".
//
// Keying on the *type* rather than the method keeps this from going stale
// silently: a setter taking a type with no sample fails with "no sample for
// type X" rather than being skipped.
var typeSamples = map[reflect.Type][2]any{
	reflect.TypeOf(dml.Color{}):       {dml.ColorRed, dml.ColorBlue},
	reflect.TypeOf(dml.Fill{}):        {dml.NewSolidFill(dml.ColorRed), dml.NewSolidFill(dml.ColorBlue)},
	reflect.TypeOf(dml.DashStyle("")): {dml.DashDash, dml.DashDot},
	// Bevel presets are a fixed vocabulary; width and height are deliberately
	// unequal so a getter reading the neighbouring slot fails.
	reflect.TypeOf(dml.Bevel3D{}): {
		dml.Bevel3D{Preset: "circle", Width: 4, Height: 6},
		dml.Bevel3D{Preset: "relaxedInset", Width: 7, Height: 9},
	},
	// Reflection's fractions and angles are stored as percentages and 60000ths
	// of a degree; these values are exact in both.
	reflect.TypeOf(dml.Reflection{}): {
		dml.Reflection{BlurRadius: 3, Distance: 5, Direction: 90, FadeDirection: 180, StartOpacity: 0.5, StartPosition: 0.1, EndOpacity: 0.2, EndPosition: 0.9},
		dml.Reflection{BlurRadius: 6, Distance: 8, Direction: 45, FadeDirection: 270, StartOpacity: 0.4, StartPosition: 0.3, EndOpacity: 0.6, EndPosition: 0.8},
	},
	reflect.TypeOf(ConnectorKind(0)):           {ConnectorElbow, ConnectorCurved},
	reflect.TypeOf(BulletType(0)):              {BulletChar, BulletNone},
	reflect.TypeOf(AutoNumberScheme("")):       {AutoNumberArabicPeriod, AutoNumberRomanUcPeriod},
	reflect.TypeOf(AutofitType(0)):             {AutofitShape, AutofitNormal},
	reflect.TypeOf(PlayMode(0)):                {PlayAutomatically, PlayOnClick},
	reflect.TypeOf(PlaceholderOrientation("")): {PlaceholderOrientationVertical, PlaceholderOrientationHorizontal},
	reflect.TypeOf(PlaceholderSize("")):        {PlaceholderSizeHalf, PlaceholderSizeQuarter},
	reflect.TypeOf(BorderStyle("")):            {BorderStyleDashed, BorderStyleDotted},
	reflect.TypeOf(TabAlign("")):               {TabAlignCenter, TabAlignRight},
	reflect.TypeOf(TransitionType(0)):          {TransitionFade, TransitionWipe},
	reflect.TypeOf(TransitionDirection("")):    {TransitionDirLeft, TransitionDirRight},
	reflect.TypeOf(TransitionOrientation("")):  {TransitionHorizontal, TransitionVertical},
	reflect.TypeOf(MorphOption("")):            {MorphByWord, MorphByChar},
	// Duration is coarse in the base schema (it snaps to 0.5/1.0/2.0), so the
	// samples use values that survive it.
	reflect.TypeOf(Transition{}): {
		Transition{Type: TransitionWipe, Duration: 1, AdvanceOnClick: true, Direction: TransitionDirLeft},
		Transition{Type: TransitionBlind, Duration: 2, AdvanceOnClick: true, Orientation: TransitionVertical},
	},
	reflect.TypeOf(enum.TextAlign("")):      {enum.TextAlignCenter, enum.TextAlignRight},
	reflect.TypeOf(enum.VerticalAlign("")):  {enum.VerticalAlignMiddle, enum.VerticalAlignBottom},
	reflect.TypeOf(enum.TextAnchor("")):     {enum.TextAnchorMiddle, enum.TextAnchorBottom},
	reflect.TypeOf(enum.TextWrapping("")):   {enum.TextWrappingNone, enum.TextWrappingSquare},
	reflect.TypeOf(enum.StrikeStyle("")):    {enum.StrikeSingle, enum.StrikeDouble},
	reflect.TypeOf(enum.UnderlineStyle("")): {enum.UnderlineSingle, enum.UnderlineDouble},
}

// argGen builds one distinct value per parameter (and per struct field), from
// an ascending counter. Distinct values matter: a bevel whose width equals its
// height, or a rectangle whose width equals its height, passes even when the
// getter reads the neighbouring slot.
type argGen struct {
	seed int
	n    int
	deck *Presentation
	t    *testing.T
}

// next returns the next value in the ascending sequence. The two seeds start
// the sequence at different points, so seed 1 writes a different value than
// seed 0 everywhere.
func (g *argGen) next() int {
	g.n++
	return g.n + 5*g.seed
}

func (g *argGen) args(ft reflect.Type) ([]reflect.Value, error) {
	n := ft.NumIn()
	args := make([]reflect.Value, 0, n)
	for i := 0; i < n; i++ {
		pt := ft.In(i)
		if ft.IsVariadic() && i == n-1 {
			args = append(args, reflect.MakeSlice(pt, 0, 0))
			continue
		}
		v, err := g.value(pt)
		if err != nil {
			return nil, fmt.Errorf("parameter %d (%s): %w", i, pt, err)
		}
		args = append(args, v)
	}
	return args, nil
}

func (g *argGen) value(pt reflect.Type) (reflect.Value, error) {
	if s, ok := typeSamples[pt]; ok {
		return reflect.ValueOf(s[g.seed]).Convert(pt), nil
	}
	switch pt {
	case reflect.TypeOf(dml.EMU(0)):
		return reflect.ValueOf(dml.EMU(int64(g.next()) * 12700)), nil
	case reflect.TypeOf((*Shape)(nil)).Elem():
		return reflect.ValueOf(Shape(findShape[*AutoShape](g.t, g.deck))), nil
	}
	if pt.Kind() == reflect.Interface && pt.NumMethod() == 0 {
		v := reflect.New(pt).Elem()
		v.Set(reflect.ValueOf(fmt.Sprintf("guard-%d", g.next())))
		return v, nil
	}
	// A named scalar with no sample is an enumeration whose valid values this
	// file does not know; inventing one would write something the format may
	// legally drop.
	if pt.PkgPath() != "" && pt.Kind() != reflect.Struct && pt.Kind() != reflect.Slice && pt.Kind() != reflect.Pointer {
		return reflect.Value{}, fmt.Errorf("no sample for named type %s: add one to typeSamples", pt)
	}

	switch pt.Kind() {
	case reflect.String:
		return reflect.ValueOf(fmt.Sprintf("guard-%d", g.next())).Convert(pt), nil
	case reflect.Bool:
		// Both polarities across the seeds, so a property already true is still
		// moved by one of them.
		return reflect.ValueOf(g.seed == 0).Convert(pt), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(int64(g.next())).Convert(pt), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(uint64(g.next())).Convert(pt), nil
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(float64(g.next())).Convert(pt), nil
	case reflect.Slice:
		if pt.Elem().Kind() == reflect.Uint8 {
			return reflect.ValueOf(minimalTransparentPNG).Convert(pt), nil
		}
		elem, err := g.value(pt.Elem())
		if err != nil {
			return reflect.Value{}, err
		}
		s := reflect.MakeSlice(pt, 1, 1)
		s.Index(0).Set(elem)
		return s, nil
	case reflect.Struct:
		v := reflect.New(pt).Elem()
		for i := 0; i < pt.NumField(); i++ {
			f := pt.Field(i)
			if !f.IsExported() {
				return reflect.Value{}, fmt.Errorf("%s has unexported fields and no sample: add one to typeSamples", pt)
			}
			fv, err := g.value(f.Type)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("field %s: %w", f.Name, err)
			}
			v.Field(i).Set(fv)
		}
		return v, nil
	case reflect.Pointer:
		if pt.Elem().Kind() == reflect.Struct {
			elem, err := g.value(pt.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			p := reflect.New(pt.Elem())
			p.Elem().Set(elem)
			return p, nil
		}
	}
	return reflect.Value{}, fmt.Errorf("no way to build a %s", pt)
}
