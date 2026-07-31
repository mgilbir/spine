// Package options provides the functional-option mechanism the rest of the
// module configures itself with.
//
// It exists because the mechanism was being rewritten per package. Each site
// declared its own option type and its own loop to apply a slice of them, so
// the same three decisions — do options compose left to right, is a nil option
// skipped or an error, is the base a zero value or a set of defaults — were
// answered independently and not always the same way. One of them also grew a
// parallel set of mutable package-level variables that could not be changed
// safely while another goroutine was reading, because per-call options arrived
// after the globals and were added beside them rather than replacing them.
//
// The answers are now given once, here:
//
//   - Options apply left to right, so a later option wins.
//   - A nil option is skipped, so a list built conditionally can carry a hole
//     without the caller filtering it.
//   - Resolve starts from an explicit defaults value, so a package states its
//     defaults as data (a DefaultXxx function) rather than as zero values that
//     every reader has to decode.
//
// A package states its defaults as an exported function and resolves options on
// top of it, matching pptx.DefaultOptions and pptx.DefaultCreateOptions:
//
//	type Config struct{ Limit int }
//
//	func DefaultConfig() Config { return Config{Limit: 100} }
//
//	type ConfigOption = options.Option[Config]
//
//	func WithLimit(n int) ConfigOption {
//		return func(c *Config) { c.Limit = n }
//	}
//
//	func Do(opts ...ConfigOption) { cfg := options.Resolve(DefaultConfig(), opts...); ... }
//
// Deliberately a function rather than a Default method on the configuration
// type. A value-receiver method that returns fresh defaults ignores its
// receiver, so cfg = cfg.Default() silently discards everything already set,
// while reading like it fills in the unset fields. A function takes no receiver
// and so cannot be misread that way.
//
// The alias rather than a defined type is deliberate: it keeps the package's
// own name in signatures and documentation while staying interchangeable with
// options.Option, so a caller writing a helper that returns options need not
// care which spelling it uses.
package options

// Option adjusts a configuration of type T in place.
type Option[T any] func(*T)

// Resolve applies opts to a copy of defaults, left to right, and returns the
// result. Nil options are skipped. defaults is not modified, so a caller may
// pass a shared value.
func Resolve[T any](defaults T, opts ...Option[T]) T {
	out := defaults
	for _, fn := range opts {
		if fn != nil {
			fn(&out)
		}
	}
	return out
}

// Replace returns an Option that sets the whole configuration, so a caller
// holding a prepared value can pass it where options are expected. Options
// after it still apply on top.
func Replace[T any](cfg T) Option[T] {
	return func(dst *T) { *dst = cfg }
}
