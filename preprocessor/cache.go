package preprocessor

import (
	"io/fs"
	"reflect"
	"sync"

	"github.com/vertex-language/vcx/scanner"
	"github.com/vertex-language/vcx/token"
)

// Cache shares header reads between translation units: a header's bytes,
// its phases 1-3 tokens and diagnostics, and a failed lookup, each found
// once however many units include it. Every unit of a build reads the same
// libc++ and SDK headers, and walks the same search list past the same
// directories that do not have them.
//
// Only what a file is holds here. What a unit learned by reading it — its
// guard, #pragma once, #import — stays in the unit's own open-once cache,
// because another unit's macros decide those.
//
// A Cache assumes the files it has seen do not change while it lives; a
// caller makes one per build. It is safe for concurrent use.
type Cache struct {
	mu sync.Mutex
	m  map[cacheKey]*cacheEntry
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{m: map[cacheKey]*cacheEntry{}}
}

// cacheKey names one read: the file, the display name its token.File
// carries, and the scanner settings its tokens depend on.
type cacheKey struct {
	fsys    fs.FS
	display string
	rel     string
	mode    scanner.Mode
	std     token.Std
}

type cacheEntry struct {
	once  sync.Once
	file  *token.File
	toks  []Token
	diags []token.Diagnostic
	err   error
}

// entry returns key's entry, reading and scanning it with read the first
// time any unit asks.
func (c *Cache) entry(key cacheKey, read func(e *cacheEntry)) *cacheEntry {
	c.mu.Lock()
	e, ok := c.m[key]
	if !ok {
		e = &cacheEntry{}
		c.m[key] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { read(e) })
	return e
}

// shareable reports whether fsys can key a map: os.DirFS and embed.FS can.
// A filesystem that cannot — a map, or a struct holding one — is read
// directly every time.
func shareable(fsys fs.FS) bool {
	return fsys != nil && reflect.ValueOf(fsys).Comparable()
}
