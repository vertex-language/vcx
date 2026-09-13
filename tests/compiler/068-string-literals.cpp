// Do string literals have their characters, in every encoding?
//
// §5.13.5 [lex.string] -- an ordinary literal is an array of const char
// one longer than its text; L, u, U and u8 give wchar_t, char16_t,
// char32_t and char8_t; adjacent pieces concatenate (§5.13.5/13); a raw
// literal keeps its backslashes and newlines (§5.13.5/3). §5.13.3 -- an
// escape sequence is one character: \n, \x41, \101, é. §9.4.3
// [dcl.init.string] -- a character array is initialized from a literal,
// its bound taken from the text when it gives none, the rest zeroed
// when it gives more.

int len(const char* s) { int n = 0; while (s[n]) ++n; return n; }
int wlen(const wchar_t* s) { int n = 0; while (s[n]) ++n; return n; }

int main() {
    int r = 0;

    // Concatenation and the null: "hello world" is 11 long in 12 bytes.
    const char* a = "hello" " " "world";
    if (len(a) == 11 && sizeof("hello" " " "world") == 12) r += 1;

    // Escapes are single characters.
    const char* e = "a\nb\tc\\d\"e\x41\101";
    if (len(e) == 11 && e[1] == 10 && e[3] == 9 && e[5] == 92 && e[7] == 34 && e[9] == 'A' && e[10] == 'A') r += 1;

    // A raw literal keeps what an escape would have eaten.
    const char* raw = R"(a\nb)";
    if (len(raw) == 4 && raw[1] == '\\' && raw[2] == 'n') r += 1;
    const char* delim = R"xy(paren ) here)xy";
    if (len(delim) == 12 && delim[6] == ')') r += 1;

    // Wide: wchar_t is the platform's width, and the units are the text.
    const wchar_t* w = L"wide";
    if (wlen(w) == 4 && w[0] == L'w' && sizeof(L"wide") == 5 * sizeof(wchar_t)) r += 1;

    // u and U: the code unit widths are fixed.
    const char16_t* u = u"ab";
    const char32_t* U = U"ab";
    if (sizeof(u"ab") == 6 && sizeof(U"ab") == 12 && u[1] == 98 && U[0] == 97) r += 1;

    // A character above ASCII: UTF-8 bytes in a narrow literal, one code
    // point in a wide one.
    if (sizeof("é") == 3 && sizeof(L"é") == 2 * sizeof(wchar_t) && (unsigned char)"é"[0] == 0xC3) r += 1;

    // §9.4.3: the array takes the text, zeroes the rest, or sizes itself.
    char buf[8] = "abc";
    char exact[] = "abcd";
    wchar_t wbuf[4] = L"xy";
    if (buf[2] == 'c' && buf[3] == 0 && buf[7] == 0 && sizeof(exact) == 5 && exact[4] == 0 && wbuf[2] == 0 && wbuf[3] == 0) r += 1;

    return r; // eight checks
}
