package docx

import (
	"encoding/base64"
	"encoding/xml"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// DocumentEditMode is the kind of editing a reader may still perform while
// document protection is enforced. It is the value of the w:edit attribute of
// w:documentProtection (ECMA-376 §17.15.1.29).
type DocumentEditMode string

const (
	// EditReadOnly allows no editing at all.
	EditReadOnly DocumentEditMode = "readOnly"
	// EditComments allows only adding comments.
	EditComments DocumentEditMode = "comments"
	// EditTrackedChanges allows editing but forces tracked changes on.
	EditTrackedChanges DocumentEditMode = "trackedChanges"
	// EditForms allows editing only form fields.
	EditForms DocumentEditMode = "forms"
)

// DocumentProtection is a read-only view of a document's edit-enforcement
// settings (w:documentProtection, plus w:writeProtection when present). It is
// the read counterpart of Document.Protect/Unprotect.
//
// Like Excel's sheet and workbook protection, Word's document protection is a
// UI guard, not encryption: the document is not protected cryptographically and
// any tool can clear it. HasPassword only reports that a (weak legacy or
// hashed) password guard is present; the password itself is never exposed.
type DocumentProtection struct {
	edit                DocumentEditMode
	enforced            bool
	restrictFormatting  bool
	passwordProtected   bool
	readOnlyRecommended bool
}

// Edit reports which editing mode the protection permits (w:edit).
func (p *DocumentProtection) Edit() DocumentEditMode { return p.edit }

// Enforced reports whether the restriction is actually enforced
// (w:enforcement="1"). A document may declare a restriction while leaving it
// unenforced.
func (p *DocumentProtection) Enforced() bool { return p.enforced }

// RestrictFormatting reports whether formatting is restricted to a selection of
// styles (w:formatting="1").
func (p *DocumentProtection) RestrictFormatting() bool { return p.restrictFormatting }

// HasPassword reports whether a password guard is present on the document
// protection (either the legacy hash or a modern hashValue). The password is
// never exposed.
func (p *DocumentProtection) HasPassword() bool { return p.passwordProtected }

// ReadOnlyRecommended reports whether the document advertises a read-only
// recommendation (w:writeProtection w:recommended="1").
func (p *DocumentProtection) ReadOnlyRecommended() bool { return p.readOnlyRecommended }

// Protection returns the document's edit-enforcement state, or nil when the
// settings declare neither w:documentProtection nor w:writeProtection.
func (d *Document) Protection() *DocumentProtection {
	if d.settings == nil {
		return nil
	}
	dp := d.settings.Child("documentProtection")
	wp := d.settings.Child("writeProtection")
	if dp == nil && wp == nil {
		return nil
	}
	out := &DocumentProtection{}
	if dp != nil {
		out.edit = DocumentEditMode(dp.Attr("edit"))
		out.enforced = isXMLTrue(dp.Attr("enforcement"))
		out.restrictFormatting = isXMLTrue(dp.Attr("formatting"))
		out.passwordProtected = dp.Attr("hash") != "" || dp.Attr("hashValue") != ""
	}
	if wp != nil {
		out.readOnlyRecommended = isXMLTrue(wp.Attr("recommended"))
		if wp.Attr("hash") != "" || wp.Attr("hashValue") != "" {
			out.passwordProtected = true
		}
	}
	return out
}

// DocumentProtectionOptions configures Document.Protect. The zero value enforces
// a read-only restriction on the whole document, mirroring Excel's
// Sheet.Protect default of locking everything.
type DocumentProtectionOptions struct {
	// Edit selects which editing the reader may still perform. The zero value
	// ("") is treated as EditReadOnly.
	Edit DocumentEditMode

	// Password, when non-empty, is guarded with Word's legacy 16-bit password
	// hash (see the note below). This is obfuscation, not security — it is
	// trivially removed and must not be relied on to protect confidential data.
	//
	// Note on the hash: OOXML's w:documentProtection has no dedicated 16-bit
	// password attribute, so Word 2007+ stores a SHA-based verifier in
	// w:hash/w:salt with w:cryptAlgorithmSid and friends. This library instead
	// writes the simple legacy 16-bit obfuscation hash shared with xlsx
	// (crypto.LegacyPasswordHash), base64-encoded into w:hash with no crypt
	// provider attributes. The choice keeps a single, well-understood algorithm
	// across formats; it is deliberately weak and Word may not treat it as one
	// of its own passwords, but the enforcement flag still guards the UI and the
	// file stays schema-valid.
	Password string

	// RestrictFormatting sets w:formatting="1" so that only a selection of
	// styles may be applied while protection is enforced.
	RestrictFormatting bool

	// ReadOnlyRecommended additionally writes w:writeProtection with
	// w:recommended="1", advising editors to open the document read-only.
	ReadOnlyRecommended bool
}

// Protect turns on document protection with the given options, replacing any
// existing w:documentProtection (and, when ReadOnlyRecommended is set,
// w:writeProtection) in settings.xml. It works on both created and opened
// documents; a save regenerates the settings part with the new protection.
//
// Word document protection is a UI guard, not encryption. Even with a password
// it is trivially removed; do not use it to protect confidential data.
func (d *Document) Protect(opts DocumentProtectionOptions) {
	if d.settings == nil {
		d.settings = &oxml.CT_Settings{}
	}

	edit := opts.Edit
	if edit == "" {
		edit = EditReadOnly
	}

	attrs := []xml.Attr{
		wAttr("edit", string(edit)),
		wAttr("enforcement", "1"),
	}
	if opts.RestrictFormatting {
		attrs = append(attrs, wAttr("formatting", "1"))
	}
	if opts.Password != "" {
		attrs = append(attrs, wAttr("hash", legacyDocxHash(opts.Password)))
	}
	d.settings.SetChild("documentProtection", attrs)

	if opts.ReadOnlyRecommended {
		wattrs := []xml.Attr{wAttr("recommended", "1")}
		if opts.Password != "" {
			wattrs = append(wattrs, wAttr("hash", legacyDocxHash(opts.Password)))
		}
		d.settings.SetChild("writeProtection", wattrs)
	} else {
		d.settings.RemoveChild("writeProtection")
	}

	d.settingsModified = true
}

// Unprotect removes document protection (both w:documentProtection and
// w:writeProtection), if any.
func (d *Document) Unprotect() {
	if d.settings == nil {
		return
	}
	removed := d.settings.RemoveChild("documentProtection")
	removed = d.settings.RemoveChild("writeProtection") || removed
	if removed {
		d.settingsModified = true
	}
}

// wAttr builds a WordprocessingML-namespaced attribute so it serializes with
// the w: prefix.
func wAttr(local, value string) xml.Attr {
	return xml.Attr{Name: xml.Name{Space: oxml.NsWml, Local: local}, Value: value}
}

// legacyDocxHash renders the legacy 16-bit obfuscation hash as the base64 of
// its two big-endian bytes, suitable for the w:hash attribute (ST_Base64Binary).
// See DocumentProtectionOptions.Password for why this simple form is used.
func legacyDocxHash(password string) string {
	h := crypto.LegacyPasswordHash(password)
	return base64.StdEncoding.EncodeToString([]byte{byte(h >> 8), byte(h)})
}

// isXMLTrue reports whether an OOXML boolean attribute value denotes true.
func isXMLTrue(v string) bool {
	switch v {
	case "1", "true", "on":
		return true
	default:
		return false
	}
}
