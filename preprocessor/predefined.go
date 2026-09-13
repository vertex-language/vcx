package preprocessor

import (
	"strconv"

	"github.com/vertex-language/vcx/token"
)

// Builtin marks the macros whose replacement list is computed rather than
// stored. They are not macros in the table's ordinary sense — expansion
// consults this file for them — but they occupy names, can be #undef'd where
// the standard allows it, and can be shadowed by a #define, so they live in
// the same table.
type Builtin uint8

const (
	NotBuiltin Builtin = iota
	BuiltinFile
	BuiltinLine
	BuiltinDate
	BuiltinTime
	BuiltinCounter
)

// installPredefines enters the macros [cpp.predefined] requires, then applies
// the caller's -D and -U in order, so a command line can shadow anything the
// language does not forbid it from shadowing.
//
// The feature-test macros of [cpp.predefined]/2 — __cpp_concepts,
// __cpp_lib_*, and the rest of the table — are not here yet. They are a claim
// about what the front end implements, so each one lands with the feature it
// names rather than as a block of optimistic constants: a header that reads
// __cpp_consteval and gets a number vcx cannot honour is worse off than one
// that reads nothing.
func (p *Preprocessor) installPredefines() {
	p.builtinMacro("__FILE__", BuiltinFile)
	p.builtinMacro("__LINE__", BuiltinLine)
	p.builtinMacro("__DATE__", BuiltinDate)
	p.builtinMacro("__TIME__", BuiltinTime)
	p.builtinMacro("__COUNTER__", BuiltinCounter)

	p.valueMacro("__cplusplus", cplusplus(p.cfg.Std))
	if p.cfg.Hosted {
		p.valueMacro("__STDC_HOSTED__", "1")
	} else {
		p.valueMacro("__STDC_HOSTED__", "0")
	}

	for _, d := range p.cfg.Predefines {
		switch d.Kind {
		case PredefineDefine:
			p.DefineText(d.Text)
		case PredefineUndef:
			p.UndefText(d.Text)
		}
	}
}

// cplusplus is __cplusplus. C++23 is 202302L; the C++26 working draft has no
// final value yet, and 202400L is what g++ and clang++ report for it.
func cplusplus(std token.Std) string {
	if std >= token.Cxx26 {
		return "202400L"
	}
	return "202302L"
}

// builtinMacro enters a name whose replacement list is computed at each
// expansion. The body is empty and never read.
func (p *Preprocessor) builtinMacro(name string, b Builtin) {
	p.macros.Define(&Macro{Name: name, ObjLike: true, Builtin: b})
}

// valueMacro enters an ordinary object-like macro with a fixed spelling. It
// goes through the same arena generated tokens use, so its tokens have a
// position space like every other token in the tree.
func (p *Preprocessor) valueMacro(name, spelling string) {
	t := p.gen.Mint(token.INT_LIT, spelling)
	p.macros.Define(&Macro{Name: name, ObjLike: true, Body: []Token{t}})
}

// builtin computes one builtin macro's replacement list at the point of use.
//
// __LINE__ and __FILE__ answer for the invocation, not for the definition,
// which is why they cannot be ordinary macros: LOG(x) that expands to
// __LINE__ must report the line the program wrote LOG on.
func (p *Preprocessor) builtin(m *Macro, at Token) []Token {
	var t Token
	switch m.Builtin {
	case BuiltinFile:
		t = p.gen.Mint(token.STRING_LIT, strconv.Quote(p.currentFile(at)))
	case BuiltinLine:
		t = p.gen.Mint(token.INT_LIT, itoa(p.physicalLine(at.Site())+p.lineDelta))
	case BuiltinDate:
		t = p.gen.Mint(token.STRING_LIT, `"`+p.cfg.Now().Format("Jan _2 2006")+`"`)
	case BuiltinTime:
		t = p.gen.Mint(token.STRING_LIT, `"`+p.cfg.Now().Format("15:04:05")+`"`)
	case BuiltinCounter:
		t = p.gen.Mint(token.INT_LIT, itoa(p.counter))
		p.counter++
	default:
		return nil
	}
	t.Flags = at.Flags & (token.FlagAdjacent | token.FlagNLBefore)
	t.Exp = &Expansion{Macro: m.Name, Use: at.Site(), Outer: at.Exp}
	return []Token{t}
}

// currentFile is what __FILE__ reports: the name #line last set, or the file
// the token was written in. Never absolute, so a build does not leak the
// machine's directory layout into the binary.
func (p *Preprocessor) currentFile(at Token) string {
	if p.fileName != "" {
		return p.fileName
	}
	if s := at.Site(); s.Origin != nil {
		return s.Origin.Name()
	}
	return "<unknown>"
}

// physicalLine is the line a site is on, as typed. #line's offset is applied
// by the caller, so that this stays the answer to a question about the file.
func (p *Preprocessor) physicalLine(s Site) int {
	if s.Origin == nil || s.Origin.File == nil || !s.Pos.IsValid() {
		return 0
	}
	return s.Origin.File.Position(s.Pos).Line
}
