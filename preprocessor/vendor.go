package preprocessor

import (
	"slices"
	"strings"

	"github.com/vertex-language/vcx/token"
)

// Vendor configures vendor-specific preprocessor operator responses
// (e.g. __has_builtin, __has_feature, __is_target_*).
type Vendor struct {
	// Features is __has_feature: `cxx_rvalue_references`, `cxx_rtti`.
	Features map[string]bool

	// Extensions is what __has_extension adds to Features: a feature of a
	// later standard accepted as an extension in an earlier one.
	Extensions map[string]bool

	// Builtins is __has_builtin: `__builtin_expect`, `__is_same`.
	Builtins map[string]bool

	// Attributes is __has_attribute, the GNU spelling's attributes, each
	// with the value the operator yields -- 1 for a GNU attribute.
	Attributes map[string]int

	// Arch, VendorName, OS and Environment are the target triple's four
	// parts, which __is_target_arch and its neighbours compare against, each
	// with every spelling that names it: `arm64` and `aarch64`, `macos`,
	// `macosx` and `darwin`.
	Arch, VendorName, OS, Environment []string
}

// The vendor operators, by name.
const (
	opHasFeature     = "__has_feature"
	opHasExtension   = "__has_extension"
	opHasBuiltin     = "__has_builtin"
	opHasAttribute   = "__has_attribute"
	opHasCAttribute  = "__has_c_attribute"
	opHasWarning     = "__has_warning"
	opHasIncludeNext = "__has_include_next"
	opIsTargetArch   = "__is_target_arch"
	opIsTargetVendor = "__is_target_vendor"
	opIsTargetOS     = "__is_target_os"
	opIsTargetEnv    = "__is_target_environment"
)

// isOperatorName reports whether name is a supported preprocessor operator.
func (p *Preprocessor) isOperatorName(name string) bool {
	switch name {
	case "__has_include", "__has_cpp_attribute":
		return true
	}
	return p.cfg.Vendor != nil && vendorOperator(name)
}

func vendorOperator(name string) bool {
	switch name {
	case opHasFeature, opHasExtension, opHasBuiltin, opHasAttribute, opHasCAttribute,
		opHasWarning, opHasIncludeNext, opIsTargetArch, opIsTargetVendor, opIsTargetOS, opIsTargetEnv:
		return true
	}
	return false
}

// evalVendor answers one vendor operator at line[i], returning its value
// and the index of its closing parenthesis.
func (p *Preprocessor) evalVendor(line []Token, i int) (n, end int, ok bool) {
	t := line[i]
	name := t.Text()
	if name == opHasIncludeNext {
		return p.evalHasIncludeNext(line, i)
	}
	j := i + 1
	if j >= len(line) || line[j].Kind != token.LPAREN {
		p.errorf(t.Site(), "%s requires a parenthesized operand", name)
		return 0, 0, false
	}
	// The operand as written, parentheses nested inside it included:
	// `__has_attribute(__always_inline__)`, `__has_warning("-Wx")`.
	var b strings.Builder
	depth := 1
	for j++; j < len(line); j++ {
		switch line[j].Kind {
		case token.LPAREN:
			depth++
		case token.RPAREN:
			if depth--; depth == 0 {
				return p.vendorAnswer(name, b.String()), j, true
			}
		}
		b.WriteString(line[j].Text())
	}
	p.errorf(t.Site(), "missing ')' after %s", name)
	return 0, 0, false
}

func (p *Preprocessor) vendorAnswer(op, operand string) int {
	v := p.cfg.Vendor
	b := func(x bool) int {
		if x {
			return 1
		}
		return 0
	}
	switch op {
	case opHasFeature:
		return b(v.Features[operand])
	case opHasExtension:
		return b(v.Features[operand] || v.Extensions[operand])
	case opHasBuiltin:
		return b(v.Builtins[operand])
	case opHasAttribute:
		return v.Attributes[normalizeAttr(operand)]
	case opHasCAttribute, opHasWarning:
		// No C attributes in a C++ translation unit, and no warning
		// switches a header could turn off.
		return 0
	case opIsTargetArch:
		return b(slices.Contains(v.Arch, operand))
	case opIsTargetVendor:
		return b(slices.Contains(v.VendorName, operand))
	case opIsTargetOS:
		return b(slices.Contains(v.OS, operand))
	case opIsTargetEnv:
		return b(slices.Contains(v.Environment, operand))
	}
	return 0
}

// normalizeAttr is an attribute name without the underscores it may be
// spelled with, and without a `gnu::` scope: __always_inline__ is
// always_inline.
func normalizeAttr(s string) string {
	s = strings.TrimPrefix(s, "gnu::")
	if len(s) > 4 && strings.HasPrefix(s, "__") && strings.HasSuffix(s, "__") {
		s = s[2 : len(s)-2]
	}
	return s
}

// evalHasIncludeNext is __has_include with #include_next's search: whether
// a header of the name exists after the directory the current file came
// from.
func (p *Preprocessor) evalHasIncludeNext(line []Token, i int) (n, end int, ok bool) {
	t := line[i]
	j := i + 1
	if j >= len(line) || line[j].Kind != token.LPAREN {
		p.errorf(t.Site(), "__has_include_next requires a parenthesized header name")
		return 0, 0, false
	}
	name, angled, consumed, ok := p.headerName(line[j+1:], t.Site())
	if !ok {
		return 0, 0, false
	}
	j += 1 + consumed
	if j >= len(line) || line[j].Kind != token.RPAREN {
		p.errorf(t.Site(), "missing ')' after __has_include_next")
		return 0, 0, false
	}
	if p.probeFrom(name, angled, true) {
		return 1, j, true
	}
	return 0, j, true
}
