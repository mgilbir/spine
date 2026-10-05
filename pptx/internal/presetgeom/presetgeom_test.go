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

func TestTextWarps(t *testing.T) {
	for name, paths := range map[string]int{"textArchUp": 1, "textWave1": 2, "textInflate": 2, "textWaveNotAWarp": 0, "textButton": 3, "textDeflateInflateDeflate": 6} {
		g, ok := LookupTextWarp(name)
		if paths == 0 {
			if ok {
				t.Errorf("%s: unexpected warp", name)
			}
			continue
		}
		if !ok || g.PathLst == nil || len(g.PathLst.Path) != paths {
			t.Errorf("%s: %v %+v", name, ok, g)
		}
	}
	if _, ok := Lookup("textInflate"); ok {
		t.Error("a warp is not a shape")
	}
}
