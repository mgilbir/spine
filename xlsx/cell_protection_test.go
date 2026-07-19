package xlsx

import (
	"bytes"
	"testing"
)

func TestCellStyle_Protection_GetSet(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	// An unlocked, formula-hidden cell format.
	idx, err := sm.NewCellStyle(CellStyle{
		Protection: &ProtectionStyle{Locked: false, Hidden: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if idx == 0 {
		t.Fatal("expected a non-default style index")
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Protection == nil {
		t.Fatal("Protection = nil after round-trip through the style manager")
	}
	if cs.Protection.Locked {
		t.Error("Locked = true, want false")
	}
	if !cs.Protection.Hidden {
		t.Error("Hidden = false, want true")
	}
}

func TestCellStyle_Protection_SaveReopen(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	c.SetString("editable")
	if err := c.SetStyle(CellStyle{Protection: &ProtectionStyle{Locked: false}}); err != nil {
		t.Fatal(err)
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rw, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	rs := firstSheet(t, rw)
	rc, err := rs.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	si := rc.StyleIndex()
	if si == nil {
		t.Fatal("cell has no style index after reopen")
	}
	cs, err := rw.Styles().GetCellStyle(*si)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Protection == nil {
		t.Fatal("Protection = nil after save+reopen")
	}
	if cs.Protection.Locked {
		t.Error("Locked = true after reopen, want false")
	}
}

func TestCellStyle_Protection_DefaultLocked(t *testing.T) {
	wb := Create()
	sm := wb.Styles()
	// Locked defaults to Excel's true when a protection block is present but the
	// locked attribute is absent; here we assert the read-back of an explicit
	// locked cell.
	idx, err := sm.NewCellStyle(CellStyle{Protection: &ProtectionStyle{Locked: true}})
	if err != nil {
		t.Fatal(err)
	}
	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Protection == nil || !cs.Protection.Locked {
		t.Errorf("Locked = %v, want true", cs.Protection)
	}
}
