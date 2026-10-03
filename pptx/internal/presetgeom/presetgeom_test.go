package presetgeom

import "testing"

func TestPresets(t *testing.T) {
	if err := Err(); err != nil {
		t.Fatal(err)
	}
	if len(shapes) != 186 {
		t.Fatalf("%d presets", len(shapes))
	}
	g, ok := Lookup("rightArrow")
	if !ok || g.AvLst == nil || len(g.AvLst.Gd) != 2 || g.GdLst == nil || g.PathLst == nil || len(g.PathLst.Path) != 1 || len(g.PathLst.Path[0].Commands()) == 0 {
		t.Fatalf("rightArrow: %+v", g)
	}
	if _, ok := Lookup("nonsense"); ok {
		t.Fatal("unknown preset found")
	}
}
