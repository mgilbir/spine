package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// The picture accessors below are deliberately exercised with a NON-SQUARE
// image (10x8 px at 96 DPI -> 95250 x 76200 EMU -> 7.5 x 6 pt). A square
// fixture would pass under exactly the transposition bug these tests exist to
// catch, so every assertion pins width and height to different numbers and
// cross-checks the EMU and point forms against each other.

const (
	tinyPNGWidthEMU  = 10 * emuPerPixel // 95250
	tinyPNGHeightEMU = 8 * emuPerPixel  // 76200
	tinyPNGWidthPt   = 7.5
	tinyPNGHeightPt  = 6.0
)

// TestEMUToPoints pins the EMU->point conversion. A wrong constant (e.g. 96
// DPI instead of 72 pt/inch) or an inverted ratio changes every reported
// dimension in the picture API.
func TestEMUToPoints(t *testing.T) {
	cases := []struct {
		emu  int64
		want float64
	}{
		{0, 0},
		{914400, 72},      // one inch
		{12700, 1},        // one point
		{tinyPNGWidthEMU, tinyPNGWidthPt},
		{tinyPNGHeightEMU, tinyPNGHeightPt},
		{-12700, -1},
	}
	for _, tc := range cases {
		if got := emuToPoints(tc.emu); got != tc.want {
			t.Errorf("emuToPoints(%d) = %v, want %v", tc.emu, got, tc.want)
		}
	}
}

// TestPictureDimensionAccessors checks Width/Height/WidthEMU/HeightEMU against
// each other and against the image's intrinsic size. The bug class is a
// transposed accessor (HeightEMU returning the width) or a missing unit
// conversion; both are invisible when width == height or when each accessor is
// only asserted in isolation.
func TestPictureDimensionAccessors(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPictureFromBytes(tinyPNG10x8(), "image/png")
	if err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}

	check := func(t *testing.T, label string, pic *Picture) {
		t.Helper()
		if tinyPNGWidthEMU == tinyPNGHeightEMU {
			t.Fatal("fixture must not be square, or a transposition bug passes")
		}
		if got := pic.WidthEMU(); got != tinyPNGWidthEMU {
			t.Errorf("%s: WidthEMU() = %d, want %d", label, got, tinyPNGWidthEMU)
		}
		if got := pic.HeightEMU(); got != tinyPNGHeightEMU {
			t.Errorf("%s: HeightEMU() = %d, want %d", label, got, tinyPNGHeightEMU)
		}
		if got := pic.Width(); got != tinyPNGWidthPt {
			t.Errorf("%s: Width() = %v, want %v", label, got, tinyPNGWidthPt)
		}
		if got := pic.Height(); got != tinyPNGHeightPt {
			t.Errorf("%s: Height() = %v, want %v", label, got, tinyPNGHeightPt)
		}
		// The point and EMU forms must describe the same frame: a getter wired
		// to the wrong field shows up here even if both constants above were
		// updated to match the bug.
		w, h := pic.Size()
		if int64(w) != pic.WidthEMU() {
			t.Errorf("%s: WidthEMU() = %d, but Size() width = %d", label, pic.WidthEMU(), w)
		}
		if int64(h) != pic.HeightEMU() {
			t.Errorf("%s: HeightEMU() = %d, but Size() height = %d", label, pic.HeightEMU(), h)
		}
		if pic.Width() != emuToPoints(pic.WidthEMU()) {
			t.Errorf("%s: Width() = %v disagrees with emuToPoints(WidthEMU()) = %v", label, pic.Width(), emuToPoints(pic.WidthEMU()))
		}
		if pic.Height() != emuToPoints(pic.HeightEMU()) {
			t.Errorf("%s: Height() = %v disagrees with emuToPoints(HeightEMU()) = %v", label, pic.Height(), emuToPoints(pic.HeightEMU()))
		}
	}

	check(t, "created", pic)

	// An explicit SetSize must be reflected by all four accessors, still with
	// width != height.
	pic.SetSize(dml.Inches(3), dml.Inches(1))
	if got, want := pic.WidthEMU(), int64(dml.Inches(3)); got != want {
		t.Errorf("after SetSize: WidthEMU() = %d, want %d", got, want)
	}
	if got, want := pic.HeightEMU(), int64(dml.Inches(1)); got != want {
		t.Errorf("after SetSize: HeightEMU() = %d, want %d", got, want)
	}
	if got := pic.Width(); got != 216 {
		t.Errorf("after SetSize: Width() = %v, want 216", got)
	}
	if got := pic.Height(); got != 72 {
		t.Errorf("after SetSize: Height() = %v, want 72", got)
	}

	// Reset to the intrinsic size and confirm the accessors survive a save and
	// reopen, i.e. that they read the frame written to the part.
	pic.SetSize(tinyPNGWidthEMU, tinyPNGHeightEMU)
	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	pics := rs.Pictures()
	if len(pics) != 1 {
		t.Fatalf("Pictures() = %d, want 1", len(pics))
	}
	check(t, "reopened", pics[0])
}

// TestPicturePartName covers the three states of PartName: unresolvable before
// the picture has a slide, unresolvable before the deck is saved (no
// relationship exists yet), and the media part name after a save/reopen. A
// PartName that returned a non-empty placeholder in the unresolved cases, or
// the relationship id rather than the target part, fails here.
func TestPicturePartName(t *testing.T) {
	if got := NewPicture().PartName(); got != "" {
		t.Errorf("detached Picture.PartName() = %q, want \"\"", got)
	}

	p := Create()
	s := p.AddSlide()
	pic, err := s.AddPictureFromBytes(tinyPNG10x8(), "image/png")
	if err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	if got := pic.PartName(); got != "" {
		t.Errorf("unsaved Picture.PartName() = %q, want \"\" (no relationship yet)", got)
	}

	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	pics := rs.Pictures()
	if len(pics) != 1 {
		t.Fatalf("Pictures() = %d, want 1", len(pics))
	}
	got := pics[0].PartName()
	if got != "/ppt/media/image1.png" {
		t.Errorf("Picture.PartName() = %q, want /ppt/media/image1.png", got)
	}
	// The name must actually address a part in the package.
	if _, ok := rp.otherParts[got]; !ok {
		t.Errorf("PartName() = %q, which is not a part of the package", got)
	}

	// A picture whose relationship id does not resolve must report "" rather
	// than a fabricated name.
	pics[0].relID = "rIdNoSuchThing"
	if got := pics[0].PartName(); got != "" {
		t.Errorf("PartName() with a dangling relID = %q, want \"\"", got)
	}
}

// TestMediaShapeAccessors covers the mediaShape read surface on both Video and
// Audio. Every pair of fields that could be crossed (media bytes vs poster
// bytes, media content type vs poster content type) is given distinguishable
// values, so a getter reading its neighbour's field fails.
func TestMediaShapeAccessors(t *testing.T) {
	// A minimal MP4 (ftyp box) and a distinct audio payload, so the media data
	// of one shape can never be mistaken for the other's or for the poster.
	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42")...)
	mp4 = append(mp4, make([]byte, 16)...)
	mp3 := append([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"), make([]byte, 16)...)
	posterBytes := []byte("POSTER-BYTES-NOT-MEDIA")

	cases := []struct {
		name      string
		mediaData []byte
		mediaCT   string
		wantType  ShapeType
		add       func(*Slide, []byte, string) *mediaShape
		shape     func(*Slide, []byte, string) Shape
	}{
		{
			name: "video", mediaData: mp4, mediaCT: "video/mp4", wantType: ShapeTypeVideo,
			add:   func(s *Slide, d []byte, ct string) *mediaShape { return &s.AddVideo(d, ct).mediaShape },
			shape: func(s *Slide, d []byte, ct string) Shape { return s.AddVideo(d, ct) },
		},
		{
			name: "audio", mediaData: mp3, mediaCT: "audio/mpeg", wantType: ShapeTypeAudio,
			add:   func(s *Slide, d []byte, ct string) *mediaShape { return &s.AddAudio(d, ct).mediaShape },
			shape: func(s *Slide, d []byte, ct string) Shape { return s.AddAudio(d, ct) },
		},
	}

	seenTypes := map[ShapeType]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Create()
			s := p.AddSlide()

			if got := tc.shape(s, tc.mediaData, tc.mediaCT).ShapeType(); got != tc.wantType {
				t.Errorf("ShapeType() = %v, want %v", got, tc.wantType)
			}
			if prev, dup := seenTypes[tc.wantType]; dup {
				t.Errorf("ShapeType %v is also reported by %s: the kinds must be distinct", tc.wantType, prev)
			}
			seenTypes[tc.wantType] = tc.name

			m := tc.add(s, tc.mediaData, tc.mediaCT)

			if !bytes.Equal(m.MediaData(), tc.mediaData) {
				t.Errorf("MediaData() = %q, want the media payload %q", m.MediaData(), tc.mediaData)
			}
			if got := m.ContentType(); got != tc.mediaCT {
				t.Errorf("ContentType() = %q, want %q", got, tc.mediaCT)
			}

			// Poster is unset: both results must be empty, not the media's.
			if data, ct := m.Poster(); len(data) != 0 || ct != "" {
				t.Errorf("Poster() on unset media = (%q, %q), want empty", data, ct)
			}

			m.SetPoster(posterBytes, "image/png")
			data, ct := m.Poster()
			if !bytes.Equal(data, posterBytes) {
				t.Errorf("Poster() data = %q, want %q", data, posterBytes)
			}
			if ct != "image/png" {
				t.Errorf("Poster() contentType = %q, want image/png", ct)
			}
			// Setting a poster must not disturb the media payload or its type.
			if !bytes.Equal(m.MediaData(), tc.mediaData) {
				t.Errorf("SetPoster clobbered MediaData(): %q", m.MediaData())
			}
			if got := m.ContentType(); got != tc.mediaCT {
				t.Errorf("SetPoster clobbered ContentType(): %q, want %q", got, tc.mediaCT)
			}

			// PlayMode: default, then each mode reported back as itself.
			if got := m.PlayMode(); got != PlayOnClick {
				t.Errorf("default PlayMode() = %v, want PlayOnClick", got)
			}
			for _, mode := range []PlayMode{PlayAutomatically, PlayOnClick} {
				m.SetPlayMode(mode)
				if got := m.PlayMode(); got != mode {
					t.Errorf("after SetPlayMode(%v), PlayMode() = %v", mode, got)
				}
			}
		})
	}
}
