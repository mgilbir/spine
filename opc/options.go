package opc

import "github.com/mgilbir/spine/common/options"

// Reader configuration lives here, in one form.
//
// It used to live in three: five mutable package-level variables, a
// ReaderOptions struct passed to a parallel set of *WithOptions entry points,
// and functional options over the top. The variables were the oldest layer and
// the worst — their own doc comments conceded they could not be changed while
// another goroutine was opening a package — and each later layer was added
// beside the previous one rather than replacing it, so a caller had to know
// which of three mechanisms a given knob answered to, and which zero values
// meant "default" as against "off".
//
// Now: DefaultReaderOptions is the starting point, With* options adjust it, and
// the open functions take them variadically. There is no global state to
// coordinate, so concurrent opens with different limits need no coordination,
// and one rule covers every bound — a value of zero or less disables it.

// Default bounds applied to a Reader when no option overrides them. They are
// constants rather than variables so a process cannot mutate them out from
// under a Reader that is already open; to use a different value, pass the
// matching option.
const (
	// DefaultMaxDecompressedPartSize bounds how many bytes any single part may
	// decompress to, guarding against decompression bombs — a small compressed
	// entry that expands enormously.
	DefaultMaxDecompressedPartSize int64 = 1 << 30 // 1 GiB

	// DefaultMaxDecompressedPackageSize bounds the total across all parts. It
	// catches what the per-part bound cannot: a package that honestly declares
	// many parts, each individually within the limit.
	DefaultMaxDecompressedPackageSize int64 = 4 << 30 // 4 GiB

	// DefaultMaxPackageEntries bounds how many zip entries a package may
	// contain. Entry count is a dimension the byte-oriented bounds cannot see:
	// every entry costs a header, a name and a *File whether or not it is ever
	// read (C459).
	DefaultMaxPackageEntries = 1 << 16 // 65536

	// DefaultMaxNestingDepth bounds how deeply elements may nest in any XML
	// part. Nesting is another dimension the byte bounds cannot see — each
	// level costs a decoder frame and a model frame however few bytes express
	// it, so a 244 KB part nesting 80,000 deep cost 627 MB resident, and the
	// per-level cost grows with depth.
	//
	// Calibrated rather than chosen: the deepest part among 170,913 parts in
	// 3,600 Common Crawl documents nests 95 levels, so this leaves an order of
	// magnitude of headroom over anything a producer writes.
	DefaultMaxNestingDepth = 1000

	// DefaultMaxEncryptedInputSize bounds how many bytes OpenEncrypted reads
	// from its source before parsing the CFB container, so a hostile size
	// argument cannot drive an unbounded allocation.
	DefaultMaxEncryptedInputSize int64 = 2 << 30 // 2 GiB
)

// ReaderOptions is the resolved configuration for one Reader.
//
// Build it with DefaultReaderOptions and the With* options rather than by hand:
// a zero-valued ReaderOptions disables every bound, which is almost never what
// a caller means.
type ReaderOptions struct {
	// MaxDecompressedPartSize bounds how many bytes any single part may
	// decompress to. Zero or less disables the bound.
	MaxDecompressedPartSize int64

	// MaxDecompressedPackageSize bounds the total bytes decompressed across all
	// parts. Zero or less disables the bound.
	MaxDecompressedPackageSize int64

	// MaxPackageEntries bounds how many zip entries the package may contain.
	// Zero or less disables the bound.
	MaxPackageEntries int

	// MaxNestingDepth bounds how deeply elements may nest in any XML part.
	// Zero or less disables the bound.
	MaxNestingDepth int

	// MaxEncryptedInputSize bounds how many bytes an open reads before parsing
	// the CFB container of an encrypted input. Zero or less disables the bound.
	// It is consulted only for an input that turns out to be encrypted; a plain
	// zip is bounded by the decompression limits instead.
	MaxEncryptedInputSize int64

	// AllowMissingDataIntegrity decrypts an agile-encrypted package whose
	// EncryptionInfo descriptor carries no dataIntegrity element WITHOUT
	// verifying the package HMAC.
	//
	// Leave it false unless you know you need it: the descriptor is plaintext
	// and unauthenticated, so an attacker who can modify the file can delete
	// that element as easily as they can flip bits in the malleable CBC
	// ciphertext. Honouring its absence turns an authenticated format into an
	// unauthenticated one at the attacker's option (C361).
	AllowMissingDataIntegrity bool

	// password, when non-nil, is the password an open uses if the input turns
	// out to be a password-encrypted (CFB-wrapped) package. Nil means no
	// password was supplied, and such an input reports ErrEncrypted. Set it
	// with WithPassword.
	//
	// It is a *string, and unexported, so that a password cannot reach a log.
	// fmt walks unexported fields as readily as exported ones, so a plain
	// string here — exported or not, at any nesting depth — would appear in
	// any %v, %+v or %#v of a value containing a ReaderOptions. A pointer
	// prints as its address instead, and being unexported it also stays out of
	// encoding/json. TestPasswordDoesNotLeakThroughFormatting holds this.
	password *string
}

// DefaultReaderOptions returns the configuration a Reader uses when no option
// overrides it: every bound set to its Default* constant, integrity
// verification required, and no password. It is the base every open resolves
// options on top of.
//
// A caller can take it to see or adjust the whole set at once:
//
//	o := opc.DefaultReaderOptions()
//	o.MaxNestingDepth = 2000
//	r, err := opc.OpenReader(path, opc.WithReaderOptions(o))
func DefaultReaderOptions() ReaderOptions {
	return ReaderOptions{
		MaxDecompressedPartSize:    DefaultMaxDecompressedPartSize,
		MaxDecompressedPackageSize: DefaultMaxDecompressedPackageSize,
		MaxPackageEntries:          DefaultMaxPackageEntries,
		MaxNestingDepth:            DefaultMaxNestingDepth,
		MaxEncryptedInputSize:      DefaultMaxEncryptedInputSize,
		AllowMissingDataIntegrity:  false,
	}
}

// ReaderOption adjusts a Reader's configuration. See package
// common/options for the rules every option set in this module follows:
// applied left to right, nil skipped, resolved on top of an explicit default.
type ReaderOption = options.Option[ReaderOptions]

// WithReaderOptions replaces the whole configuration, so a caller holding a
// prepared ReaderOptions can pass it where options are expected. Later options
// still apply on top of it.
func WithReaderOptions(o ReaderOptions) ReaderOption { return options.Replace(o) }

// WithMaxDecompressedPartSize bounds how many bytes any single part may
// decompress to. Zero or less disables the bound.
func WithMaxDecompressedPartSize(size int64) ReaderOption {
	return func(o *ReaderOptions) { o.MaxDecompressedPartSize = size }
}

// WithMaxDecompressedPackageSize bounds the total bytes decompressed across all
// parts. Zero or less disables the bound.
func WithMaxDecompressedPackageSize(size int64) ReaderOption {
	return func(o *ReaderOptions) { o.MaxDecompressedPackageSize = size }
}

// WithMaxPackageEntries bounds how many zip entries the package may contain.
// Zero or less disables the bound.
func WithMaxPackageEntries(entries int) ReaderOption {
	return func(o *ReaderOptions) { o.MaxPackageEntries = entries }
}

// WithMaxNestingDepth bounds how deeply elements may nest in any XML part.
// Zero or less disables the bound.
func WithMaxNestingDepth(depth int) ReaderOption {
	return func(o *ReaderOptions) { o.MaxNestingDepth = depth }
}

// WithMaxEncryptedInputSize bounds how many bytes an open reads before parsing
// the CFB container of an encrypted input. Zero or less disables the bound.
func WithMaxEncryptedInputSize(size int64) ReaderOption {
	return func(o *ReaderOptions) { o.MaxEncryptedInputSize = size }
}

// WithAllowMissingDataIntegrity decrypts an agile-encrypted package whose
// descriptor carries no dataIntegrity element without verifying the package
// HMAC. See ReaderOptions.AllowMissingDataIntegrity for why to leave it off.
func WithAllowMissingDataIntegrity(allow bool) ReaderOption {
	return func(o *ReaderOptions) { o.AllowMissingDataIntegrity = allow }
}

// WithPassword supplies the password for a password-encrypted (CFB-wrapped)
// package, so an encrypted document opens through the ordinary open:
//
//	r, err := opc.OpenReader(path, opc.WithPassword("secret"))
//
// The open detects the container from the input's leading bytes, so the same
// call opens a plain package and an encrypted one; the password is simply
// unused when the input is a plain zip. Without this option an encrypted input
// reports ErrEncrypted.
//
// A wrong password returns crypto.ErrWrongPassword — including an empty one,
// which is recorded as given rather than treated as no password at all, since
// only the caller knows whether they meant to supply one.
//
// The password is held out of reach of fmt and encoding/json; see
// ReaderOptions.password.
func WithPassword(password string) ReaderOption {
	return func(o *ReaderOptions) { o.password = &password }
}
