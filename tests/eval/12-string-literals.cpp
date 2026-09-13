// Does the constant evaluator read a string literal the way the language
// spells it?
//
// §5.13.5 [lex.string] -- the type is an array of const char one longer
// than the text, so sizeof counts the null and the escapes count as one
// each (§5.13.3). Adjacent pieces are one literal (§5.13.5/13); a raw
// literal's backslashes are characters (§5.13.5/3). The subscripts read
// the array the literal is.

static_assert(sizeof("") == 1);
static_assert(sizeof("abc") == 4);
static_assert(sizeof("a\nb") == 4);
static_assert(sizeof("\x41\101\0") == 4);
static_assert(sizeof("ab" "cd") == 5);
static_assert(sizeof(R"(a\nb)") == 5);

static_assert("abc"[0] == 'a');
static_assert("abc"[3] == 0);
static_assert("a\nb"[1] == '\n');
static_assert("\x41"[0] == 'A');
static_assert("\101"[0] == 'A');
static_assert(R"(a\nb)"[1] == '\\');
static_assert("ab" "cd"[2] == 'c');

// The other encodings have their own character types and widths.
static_assert(sizeof(u8"ab") == 3 * sizeof(char8_t));
static_assert(sizeof(u"ab") == 3 * sizeof(char16_t));
static_assert(sizeof(U"ab") == 3 * sizeof(char32_t));
static_assert(sizeof(L"ab") == 3 * sizeof(wchar_t));

// §9.4.3 [dcl.init.string] -- an array's bound from its literal.
constexpr char word[] = "word";
static_assert(sizeof(word) == 5);
static_assert(word[4] == 0);
constexpr char padded[8] = "ab";
static_assert(sizeof(padded) == 8);
