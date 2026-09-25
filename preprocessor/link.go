package preprocessor

import (
	"strings"

	"github.com/vertex-language/vcx/token"
)

// A LinkKind is what a link pragma names.
type LinkKind uint8

const (
	// LinkLibrary is a library: -lname.
	LinkLibrary LinkKind = iota
	// LinkFramework is an Apple framework: -framework Name.
	LinkFramework
)

// A LinkDirective is a library or framework a unit asks the link of any
// program using it for, the way a manifest's linker settings would:
//
//	#pragma vertex library("m")
//	#pragma vertex framework("AppKit")
//	#pragma comment(lib, "ws2_32")   // clang's and MSVC's spelling
//
// A unit's module carries it to the program: vsc links what the pragmas
// of every package it builds name.
type LinkDirective struct {
	Kind LinkKind
	Name string
	Site Site
}

// Links returns the link directives the unit's pragmas made, in source
// order.
func (p *Preprocessor) Links() []LinkDirective { return p.links }

// linkPragma reads `vertex library("m")`, `vertex framework("X")` or
// `comment(lib, "m")`, the tokens after #pragma.
func linkPragma(line []Token, at Site) (LinkDirective, bool) {
	var kind LinkKind
	var args []Token
	switch {
	case len(line) >= 2 && line[0].Is("vertex") && (line[1].Is("library") || line[1].Is("framework")):
		if line[1].Is("framework") {
			kind = LinkFramework
		}
		args = line[2:]
	case len(line) >= 1 && line[0].Is("comment"):
		args = line[1:]
		// comment(lib, "m"): drop the kind, keep the string.
		if len(args) < 4 || args[0].Kind != token.LPAREN || !args[1].Is("lib") || args[2].Kind != token.COMMA {
			return LinkDirective{}, false
		}
		args = append([]Token{args[0]}, args[3:]...)
	default:
		return LinkDirective{}, false
	}
	if len(args) != 3 || args[0].Kind != token.LPAREN || args[2].Kind != token.RPAREN || !plainString(args[1]) {
		return LinkDirective{}, false
	}
	name := strings.Trim(args[1].Text(), `"`)
	if name == "" {
		return LinkDirective{}, false
	}
	return LinkDirective{Kind: kind, Name: name, Site: at}, true
}
