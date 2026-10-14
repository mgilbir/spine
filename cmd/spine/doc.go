package main

import (
	"errors"

	"io"
	"os"
	"path/filepath"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// maxInputBytes is the input size guard: files larger than 1 GiB are refused.
const maxInputBytes = 1 << 30

// document is an opened Office file of either supported kind.
type document struct {
	kind      string // "xlsx" or "pptx"
	encrypted bool
	password  string
	path      string
	wb        *xlsx.Workbook
	pres      *pptx.Presentation
}

func (d *document) close() {
	if d.wb != nil {
		_ = d.wb.Close()
	}
	if d.pres != nil {
		_ = d.pres.Close()
	}
}

// openOptions carries the flags shared by opening and saving.
type openOptions struct {
	password string
}

// openDocument opens path read-only in memory, detecting the kind from the
// package's content type rather than its file name. Actual encryption is
// detected at open: the plaintext open is tried first, so a plaintext file
// opened with --password stays plaintext (encrypted=false, the password has
// no side effects), while a file that fails it with opc.ErrEncrypted is
// reopened with the password and marked encrypted. A wrong password is
// therefore reported exactly.
func openDocument(path string, opts openOptions) (*document, error) {
	if fi, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, errf("not_found", "file not found: %s", path)
		}
		return nil, errf("io", "cannot open %s: %v", path, err)
	} else if fi.Size() > maxInputBytes {
		return nil, errf("unsupported", "%s is larger than the 1 GiB input limit", path)
	}
	ropts, err := readerOptions(opts)
	if err != nil {
		return nil, err
	}
	// Detect actual encryption: try the plaintext open first.
	kind, err := sniffKind(path, nil)
	encrypted := false
	if err != nil {
		if !errors.Is(err, opc.ErrEncrypted) {
			// Either an already-mapped CLI error (unrecognizable file) or a raw
			// open failure.
			var ce *cliError
			if asCLIError(err, &ce) {
				return nil, ce
			}
			return nil, openError(path, err)
		}
		// The file really is encrypted. Without a password this is the usual
		// unsupported error; with one, reopen with it (which also reports a
		// wrong password exactly).
		if opts.password == "" {
			return nil, openError(path, err)
		}
		kind, err = sniffKind(path, ropts)
		if err != nil {
			return nil, openError(path, err)
		}
		encrypted = true
	}
	d := &document{kind: kind, encrypted: encrypted, password: opts.password, path: path}
	openOpts := ropts
	if !encrypted {
		openOpts = nil
	}
	switch kind {
	case "xlsx":
		wb, err := xlsx.Open(path, openOpts...)
		if err != nil {
			d.close()
			return nil, openError(path, err)
		}
		d.wb = wb
	case "pptx":
		p, err := pptx.Open(path, openOpts...)
		if err != nil {
			d.close()
			return nil, openError(path, err)
		}
		d.pres = p
	}
	return d, nil
}

func readerOptions(opts openOptions) ([]opc.ReaderOption, error) {
	var ropts []opc.ReaderOption
	if opts.password != "" {
		ropts = append(ropts, opc.WithPassword(opts.password))
	}
	return ropts, nil
}

// openError maps a package-open failure to an error code.
func openError(path string, err error) *cliError {
	if ce, ok := err.(*cliError); ok {
		return ce
	}
	switch {
	case errors.Is(err, opc.ErrEncrypted):
		return errf("unsupported", "%s is encrypted; pass --password to open it", path)
	case errors.Is(err, crypto.ErrWrongPassword):
		return errf("unsupported", "%s: wrong password", path)
	case errors.Is(err, opc.ErrStrictOOXML):
		return errf("unsupported", "%s is an ISO-Strict OOXML file, which is not supported", path)
	default:
		return errf("unsupported", "cannot open %s: %v", path, err)
	}
}

// sniffKind reads the package's main-part content type to decide between xlsx
// and pptx. Anything else is unsupported. The opc.OpenReader failure is
// returned raw so the caller can match opc.ErrEncrypted and retry with a
// password; content-type failures are already CLI errors.
func sniffKind(path string, ropts []opc.ReaderOption) (string, error) {
	r, err := opc.OpenReader(path, ropts...)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	rels := r.GetRelationshipsByType(opc.RelTypeOfficeDocument)
	if len(rels) == 0 || r.ContentTypes == nil {
		return "", errf("unsupported", "%s is not a recognizable Office file", path)
	}
	mainPart := opc.ResolvePartName("/", rels[0].Target)
	ct := r.ContentTypes.GetContentType(mainPart)
	switch ct {
	case opc.ContentTypeWorkbook,
		opc.ContentTypeWorkbookTemplateMain,
		opc.ContentTypeWorkbookMacroMain,
		opc.ContentTypeWorkbookTemplateMacroMain:
		return "xlsx", nil
	case opc.ContentTypePresentationMain,
		opc.ContentTypePresentationTemplateMain,
		opc.ContentTypePresentationMacroMain,
		opc.ContentTypePresentationTemplateMacroMain:
		return "pptx", nil
	default:
		return "", errf("unsupported", "%s is not a recognizable Office file", path)
	}
}

// save writes the document to path atomically: the bytes go to a temporary
// file in the destination directory (mode 0600), are fsynced, have the
// destination's existing file mode applied (0644 for a new file), and are
// renamed over the destination. A document opened with a password is
// re-encrypted. On any failure the temporary file is removed and the
// destination is untouched.
func (d *document) save(path string) error {
	destMode := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		if fi.IsDir() {
			return errf("io", "cannot save to %s: it is a directory", path)
		}
		// An existing destination keeps its file mode across the rename.
		destMode = fi.Mode().Perm()
	}
	dir := filepath.Dir(path)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return errf("io", "cannot save to %s: parent directory does not exist", path)
	}
	tmp, err := os.CreateTemp(dir, ".spine-*.tmp")
	if err != nil {
		return errf("io", "cannot create temporary file in %s: %v", dir, err)
	}
	tmpName := tmp.Name()
	if err := d.writeTo(tmp); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return errf("io", "cannot flush %s: %v", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return errf("io", "cannot close %s: %v", tmpName, err)
	}
	if err := os.Chmod(tmpName, destMode); err != nil {
		_ = os.Remove(tmpName)
		return errf("io", "cannot set mode on %s: %v", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return errf("io", "cannot replace %s: %v", path, err)
	}
	return nil
}

// writeTo serializes the document into w, encrypting it when the session
// opened it with a password.
func (d *document) writeTo(w io.Writer) error {
	if d.encrypted {
		switch d.kind {
		case "xlsx":
			return d.wb.SaveEncryptedTo(w, d.password)
		case "pptx":
			return d.pres.SaveEncryptedTo(w, d.password)
		}
	}
	switch d.kind {
	case "xlsx":
		data, err := d.wb.SaveBytes()
		if err != nil {
			return errf("io", "cannot serialize %s: %v", d.path, err)
		}
		if _, err := w.Write(data); err != nil {
			return errf("io", "cannot write %s: %v", d.path, err)
		}
		return nil
	case "pptx":
		data, err := d.pres.SaveBytes()
		if err != nil {
			return errf("io", "cannot serialize %s: %v", d.path, err)
		}
		if _, err := w.Write(data); err != nil {
			return errf("io", "cannot write %s: %v", d.path, err)
		}
		return nil
	}
	return errf("internal", "unknown document kind %q", d.kind)
}
