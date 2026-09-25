package sysroot

// builtinEntry marks where the compiler's own headers go in the search
// order. vcx fills the slot itself -- libc++ on macOS, then its own
// compiler headers (include/gnu) -- so the entry carries no filesystem:
// it is a position, not a directory.
func builtinEntry() Entry {
	return Entry{Name: "<builtin>", System: true}
}
