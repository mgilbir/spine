package opc

// ReaderOption configures a Reader, following the same shape as
// pptx.OLEObjectOption: a function that mutates the resolved settings, applied
// left to right so a later option wins.
//
// ReaderOptions itself remains the resolved form and the *WithOptions entry
// points still take it, so a caller who already builds the struct is
// unaffected. These exist because the struct form makes every field a decision
// at the call site — you write a composite literal naming the one limit you
// care about and, more importantly, a reader of that code has to know which
// zero values mean "default" and which mean "off". Naming the intent is
// clearer:
//
//	r, err := opc.OpenReader(path, opc.WithMaxNestingDepth(2000))
//
// Each option carries its field's own convention, which differs between them:
// the size and depth bounds treat a negative value as unbounded and zero as
// "use the package-level default", while AllowMissingDataIntegrity is a plain
// switch. The doc comment on each option states which applies rather than
// leaving it to be inferred.
type ReaderOption func(*ReaderOptions)

// applyReaderOptions resolves a list of options into a ReaderOptions.
func applyReaderOptions(opts []ReaderOption) ReaderOptions {
	var o ReaderOptions
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	return o
}

// WithMaxNestingDepth bounds how deeply elements may nest in any XML part of
// the package, overriding the package-level MaxNestingDepth for this Reader.
//
// Nesting is a resource dimension the byte-oriented limits cannot see: every
// level costs a decoder frame and a model frame however few bytes express it,
// so a part well under a megabyte can exhaust memory. The default (1000) is
// calibrated against real documents — the deepest part in 170,913 parts across
// 3,600 Common Crawl documents nests 95 levels.
//
// Pass a negative depth to disable the bound.
func WithMaxNestingDepth(depth int) ReaderOption {
	return func(o *ReaderOptions) { o.MaxNestingDepth = depth }
}

// WithMaxDecompressedPartSize bounds how many bytes any single part may
// decompress to, overriding the package-level MaxDecompressedPartSize for this
// Reader. It guards against decompression bombs. Pass a negative size to
// disable the bound.
func WithMaxDecompressedPartSize(size int64) ReaderOption {
	return func(o *ReaderOptions) { o.MaxDecompressedPartSize = size }
}

// WithMaxDecompressedPackageSize bounds the total bytes the Reader may
// decompress across all parts, overriding the package-level
// MaxDecompressedPackageSize for this Reader. It catches what the per-part
// bound cannot: a package that honestly declares many parts, each individually
// within the limit. Pass a negative size to disable the bound.
func WithMaxDecompressedPackageSize(size int64) ReaderOption {
	return func(o *ReaderOptions) { o.MaxDecompressedPackageSize = size }
}

// WithMaxPackageEntries bounds how many zip entries the package may contain,
// overriding the package-level MaxPackageEntries for this Reader. Every entry
// costs a header, a name and a *File whether or not it is ever read, which the
// byte-oriented bounds cannot see. Pass a negative count to disable the bound.
func WithMaxPackageEntries(entries int) ReaderOption {
	return func(o *ReaderOptions) { o.MaxPackageEntries = entries }
}

// WithAllowMissingDataIntegrity decrypts an agile-encrypted package whose
// EncryptionInfo descriptor carries no dataIntegrity element WITHOUT verifying
// the package HMAC. It applies only to the encrypted-open paths; the plain-zip
// readers ignore it.
//
// Leave it off unless you know you need it. The descriptor is plaintext and
// unauthenticated, so an attacker who can modify the file can delete that
// element as easily as they can flip bits in the malleable CBC ciphertext;
// honouring its absence turns an authenticated format into an unauthenticated
// one at the attacker's option. That was finding C361.
func WithAllowMissingDataIntegrity(allow bool) ReaderOption {
	return func(o *ReaderOptions) { o.AllowMissingDataIntegrity = allow }
}
