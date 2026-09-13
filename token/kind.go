package token

// Kind identifies the lexical class of a token.
type Kind uint8

const (
	ILLEGAL Kind = iota
	EOF
	COMMENT

	IDENT

	// HEADER_NAME is <vector> or "foo.h" in #include and import directives.
	HEADER_NAME

	// Literals (undecoded in token stream; decoded in sema).
	literal_beg
	INT_LIT
	FLOAT_LIT
	CHAR_LIT
	STRING_LIT
	literal_end

	punct_beg
	LBRACK // [
	RBRACK // ]
	LPAREN // (
	RPAREN // )
	LBRACE // {
	RBRACE // }

	PERIOD      // .
	ARROW       // ->
	PERIOD_STAR // .*
	ARROW_STAR  // ->*

	INC // ++
	DEC // --

	AND   // &
	MUL   // *
	ADD   // +
	SUB   // -
	TILDE // ~
	NOT   // !

	QUO // /
	REM // %
	SHL // <<
	SHR // >>

	LSS       // <
	GTR       // >
	LEQ       // <=
	GEQ       // >=
	EQL       // ==
	NEQ       // !=
	SPACESHIP // <=>

	XOR  // ^
	OR   // |
	LAND // &&
	LOR  // ||

	QUESTION // ?
	COLON    // :
	SCOPE    // ::
	SEMI     // ;
	ELLIPSIS // ...

	ASSIGN     // =
	MUL_ASSIGN // *=
	QUO_ASSIGN // /=
	REM_ASSIGN // %=
	ADD_ASSIGN // +=
	SUB_ASSIGN // -=
	SHL_ASSIGN // <<=
	SHR_ASSIGN // >>=
	AND_ASSIGN // &=
	XOR_ASSIGN // ^=
	OR_ASSIGN  // |=

	COMMA    // ,
	HASH     // #
	HASHHASH // ##

	// CARET_CARET is the reflection operator (^^) under C++26 (P2996).
	CARET_CARET // ^^
	punct_end

	// Keywords ([lex.key]). Contextual keywords (import, module, override, final) are parsed as identifiers.
	keyword_beg
	ALIGNAS
	ALIGNOF
	ASM
	AUTO
	BOOL
	BREAK
	CASE
	CATCH
	CHAR
	CHAR8_T
	CHAR16_T
	CHAR32_T
	CLASS
	CONCEPT
	CONST
	CONSTEVAL
	CONSTEXPR
	CONSTINIT
	CONST_CAST
	CONTINUE
	CO_AWAIT
	CO_RETURN
	CO_YIELD
	DECLTYPE
	DEFAULT
	DELETE
	DO
	DOUBLE
	DYNAMIC_CAST
	ELSE
	ENUM
	EXPLICIT
	EXPORT
	EXTERN
	FALSE
	FLOAT
	FOR
	FRIEND
	GOTO
	IF
	INLINE
	INT
	LONG
	MUTABLE
	NAMESPACE
	NEW
	NOEXCEPT
	NULLPTR
	OPERATOR
	PRIVATE
	PROTECTED
	PUBLIC
	REGISTER
	REINTERPRET_CAST
	REQUIRES
	RETURN
	SHORT
	SIGNED
	SIZEOF
	STATIC
	STATIC_ASSERT
	STATIC_CAST
	STRUCT
	SWITCH
	TEMPLATE
	THIS
	THREAD_LOCAL
	THROW
	TRUE
	TRY
	TYPEDEF
	TYPEID
	TYPENAME
	UNION
	UNSIGNED
	USING
	VIRTUAL
	VOID
	VOLATILE
	WCHAR_T
	WHILE
	std_keyword_end

	// C++26 keywords. Recognized only under Cxx26, because each is a
	// name a conforming C++23 program is allowed to have used.
	CONTRACT_ASSERT // contract_assert, P2900
	cxx26_keyword_end

	// Extension keywords.
	RESTRICT  // __restrict, __restrict__
	TYPEOF    // __typeof__, __typeof, typeof
	ATTRIBUTE // __attribute__
	DECLSPEC  // __declspec
	UUIDOF    // __uuidof — MSVC, and unavoidable in the COM headers
	UNALIGNED // __unaligned — MSVC, spelled UNALIGNED in winnt.h
	INT128    // __int128
	INT64     // __int64 — MSVC's fixed-width specifiers, from <basetsd.h>
	INT32     // __int32
	INT16     // __int16
	INT8      // __int8
	SEH_TRY   // __try
	EXCEPT    // __except
	FINALLY   // __finally
	LEAVE     // __leave
	keyword_end
)

var names = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:       "IDENT",
	HEADER_NAME: "HEADER_NAME",

	INT_LIT:    "INT_LIT",
	FLOAT_LIT:  "FLOAT_LIT",
	CHAR_LIT:   "CHAR_LIT",
	STRING_LIT: "STRING_LIT",

	LBRACK: "[",
	RBRACK: "]",
	LPAREN: "(",
	RPAREN: ")",
	LBRACE: "{",
	RBRACE: "}",

	PERIOD:      ".",
	ARROW:       "->",
	PERIOD_STAR: ".*",
	ARROW_STAR:  "->*",

	INC: "++",
	DEC: "--",

	AND:   "&",
	MUL:   "*",
	ADD:   "+",
	SUB:   "-",
	TILDE: "~",
	NOT:   "!",

	QUO: "/",
	REM: "%",
	SHL: "<<",
	SHR: ">>",

	LSS:       "<",
	GTR:       ">",
	LEQ:       "<=",
	GEQ:       ">=",
	EQL:       "==",
	NEQ:       "!=",
	SPACESHIP: "<=>",

	XOR:  "^",
	OR:   "|",
	LAND: "&&",
	LOR:  "||",

	QUESTION: "?",
	COLON:    ":",
	SCOPE:    "::",
	SEMI:     ";",
	ELLIPSIS: "...",

	ASSIGN:     "=",
	MUL_ASSIGN: "*=",
	QUO_ASSIGN: "/=",
	REM_ASSIGN: "%=",
	ADD_ASSIGN: "+=",
	SUB_ASSIGN: "-=",
	SHL_ASSIGN: "<<=",
	SHR_ASSIGN: ">>=",
	AND_ASSIGN: "&=",
	XOR_ASSIGN: "^=",
	OR_ASSIGN:  "|=",

	COMMA:       ",",
	HASH:        "#",
	HASHHASH:    "##",
	CARET_CARET: "^^",

	ALIGNAS:          "alignas",
	ALIGNOF:          "alignof",
	ASM:              "asm",
	AUTO:             "auto",
	BOOL:             "bool",
	BREAK:            "break",
	CASE:             "case",
	CATCH:            "catch",
	CHAR:             "char",
	CHAR8_T:          "char8_t",
	CHAR16_T:         "char16_t",
	CHAR32_T:         "char32_t",
	CLASS:            "class",
	CONCEPT:          "concept",
	CONST:            "const",
	CONSTEVAL:        "consteval",
	CONSTEXPR:        "constexpr",
	CONSTINIT:        "constinit",
	CONST_CAST:       "const_cast",
	CONTINUE:         "continue",
	CO_AWAIT:         "co_await",
	CO_RETURN:        "co_return",
	CO_YIELD:         "co_yield",
	DECLTYPE:         "decltype",
	DEFAULT:          "default",
	DELETE:           "delete",
	DO:               "do",
	DOUBLE:           "double",
	DYNAMIC_CAST:     "dynamic_cast",
	ELSE:             "else",
	ENUM:             "enum",
	EXPLICIT:         "explicit",
	EXPORT:           "export",
	EXTERN:           "extern",
	FALSE:            "false",
	FLOAT:            "float",
	FOR:              "for",
	FRIEND:           "friend",
	GOTO:             "goto",
	IF:               "if",
	INLINE:           "inline",
	INT:              "int",
	LONG:             "long",
	MUTABLE:          "mutable",
	NAMESPACE:        "namespace",
	NEW:              "new",
	NOEXCEPT:         "noexcept",
	NULLPTR:          "nullptr",
	OPERATOR:         "operator",
	PRIVATE:          "private",
	PROTECTED:        "protected",
	PUBLIC:           "public",
	REGISTER:         "register",
	REINTERPRET_CAST: "reinterpret_cast",
	REQUIRES:         "requires",
	RETURN:           "return",
	SHORT:            "short",
	SIGNED:           "signed",
	SIZEOF:           "sizeof",
	STATIC:           "static",
	STATIC_ASSERT:    "static_assert",
	STATIC_CAST:      "static_cast",
	STRUCT:           "struct",
	SWITCH:           "switch",
	TEMPLATE:         "template",
	THIS:             "this",
	THREAD_LOCAL:     "thread_local",
	THROW:            "throw",
	TRUE:             "true",
	TRY:              "try",
	TYPEDEF:          "typedef",
	TYPEID:           "typeid",
	TYPENAME:         "typename",
	UNION:            "union",
	UNSIGNED:         "unsigned",
	USING:            "using",
	VIRTUAL:          "virtual",
	VOID:             "void",
	VOLATILE:         "volatile",
	WCHAR_T:          "wchar_t",
	WHILE:            "while",

	CONTRACT_ASSERT: "contract_assert",

	RESTRICT:  "__restrict",
	TYPEOF:    "__typeof__",
	ATTRIBUTE: "__attribute__",
	DECLSPEC:  "__declspec",
	UUIDOF:    "__uuidof",
	UNALIGNED: "__unaligned",
	INT128:    "__int128",
	INT64:     "__int64",
	INT32:     "__int32",
	INT16:     "__int16",
	INT8:      "__int8",
	SEH_TRY:   "__try",
	EXCEPT:    "__except",
	FINALLY:   "__finally",
	LEAVE:     "__leave",
}

// String returns the keyword or punctuator spelling, or the class name
// for kinds with no fixed spelling (IDENT, INT_LIT, …).
func (k Kind) String() string {
	if int(k) < len(names) && names[k] != "" {
		return names[k]
	}
	return "Kind(" + itoa(int(k)) + ")"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// table builds a spelling→kind map over a half-open range of kinds.
func table(lo, hi Kind) map[string]Kind {
	m := make(map[string]Kind, hi-lo)
	for k := lo; k < hi; k++ {
		m[names[k]] = k
	}
	return m
}

var (
	stdKeywords   = table(keyword_beg+1, std_keyword_end)
	cxx26Keywords = table(std_keyword_end+1, cxx26_keyword_end)
	extKeywords   = table(cxx26_keyword_end+1, keyword_end)
)

// altTokens maps alternative operator spellings to their canonical operator kind.
var altTokens = map[string]Kind{
	"and":    LAND,
	"and_eq": AND_ASSIGN,
	"bitand": AND,
	"bitor":  OR,
	"compl":  TILDE,
	"not":    NOT,
	"not_eq": NEQ,
	"or":     LOR,
	"or_eq":  OR_ASSIGN,
	"xor":    XOR,
	"xor_eq": XOR_ASSIGN,
}

// aliases maps extension keyword spellings to their standard equivalent kind.
var aliases = map[string]Kind{
	// alignof aliases
	"__alignof":   ALIGNOF,
	"__alignof__": ALIGNOF,

	// thread_local alias
	"__thread": THREAD_LOCAL,

	// Qualifiers
	"__const":      CONST,
	"__volatile":   VOLATILE,
	"__volatile__": VOLATILE,

	// Inlining
	"__inline":      INLINE,
	"__inline__":    INLINE,
	"__forceinline": INLINE,

	// Inline assembly
	"__asm":   ASM,
	"__asm__": ASM,

	// nullptr alias
	"__nullptr": NULLPTR,

	// Extensions
	"__restrict":   RESTRICT,
	"__restrict__": RESTRICT,
	"__typeof":     TYPEOF,
	"typeof":       TYPEOF,
}

// Lookup maps an identifier spelling to its keyword or operator kind, or returns IDENT.
func Lookup(name string, std Std) Kind {
	if k, ok := stdKeywords[name]; ok {
		return k
	}
	if k, ok := altTokens[name]; ok {
		return k
	}
	if k, ok := extKeywords[name]; ok {
		return k
	}
	if k, ok := aliases[name]; ok {
		return k
	}
	if std >= Cxx26 {
		if k, ok := cxx26Keywords[name]; ok {
			return k
		}
	}
	return IDENT
}

// IsAltToken reports whether a spelling is an alternative operator token.
func IsAltToken(name string) bool {
	_, ok := altTokens[name]
	return ok
}

// IsStandardKeyword reports whether name is an ISO standard keyword.
func IsStandardKeyword(name string, std Std) bool {
	if _, ok := stdKeywords[name]; ok {
		return true
	}
	if std >= Cxx26 {
		if _, ok := cxx26Keywords[name]; ok {
			return true
		}
	}
	return false
}

func (k Kind) IsLiteral() bool { return literal_beg < k && k < literal_end }
func (k Kind) IsPunct() bool   { return punct_beg < k && k < punct_end }
func (k Kind) IsKeyword() bool { return keyword_beg < k && k < keyword_end }

// IsExtension reports whether a kind is a non-standard extension keyword.
func (k Kind) IsExtension() bool { return cxx26_keyword_end < k && k < keyword_end }

// Operator precedence for binary operators.
const (
	LowestPrec  = 0 // non-binary operators
	HighestPrec = 13
)

func (k Kind) Precedence() int {
	switch k {
	case COMMA:
		return 1
	case LOR:
		return 2
	case LAND:
		return 3
	case OR:
		return 4
	case XOR:
		return 5
	case AND:
		return 6
	case EQL, NEQ:
		return 7
	case LSS, GTR, LEQ, GEQ:
		return 8
	case SPACESHIP:
		return 9
	case SHL, SHR:
		return 10
	case ADD, SUB:
		return 11
	case MUL, QUO, REM:
		return 12
	case PERIOD_STAR, ARROW_STAR:
		return 13
	}
	return LowestPrec
}
