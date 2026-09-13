// §5.13 [lex.literal] -- every literal grammar, spelled out.

// §5.13.2 [lex.icon]: the four bases, the separators, and the suffixes.
auto dec  = 1234567890;
auto oct  = 0755;
auto hex  = 0xDEADbeef;
auto bin  = 0b1011'0110;
auto sep  = 1'000'000;
auto u    = 42u;
auto l    = 42L;
auto ull  = 42ULL;
auto z    = 42z;    // C++23: the signed size type
auto uz   = 42uz;   // C++23: size_t

// §5.13.4 [lex.fcon]: the decimal and hexadecimal forms.
auto d1 = 1.5;
auto d2 = .5;
auto d3 = 5.;
auto d4 = 1e10;
auto d5 = 1.5e-3;
auto f1 = 1.5f;
auto ld = 1.5L;
auto hf = 0x1.8p3;   // hexadecimal-floating-point-literal: 12.0

// §5.13.3 [lex.ccon]: the encoding prefixes, and the escapes.
auto c1 = 'a';
auto c2 = L'a';
auto c3 = u8'a';
auto c4 = u'é';
auto c5 = U'\U0001F600';
auto c6 = '\n';
auto c7 = '\0';
auto c8 = '\x41';
auto c9 = '\101';
auto ca = '\'';

// §5.13.5 [lex.string]: prefixes, concatenation, and the raw form, whose
// delimiter is whatever sits between the quote and the paren.
auto s1 = "plain";
auto s2 = L"wide";
auto s3 = u8"utf-8";
auto s4 = u"utf-16";
auto s5 = U"utf-32";
auto s6 = "adjacent" " literals" " concatenate";
auto s7 = R"(a raw \n string, unescaped)";
auto s8 = R"delim(one holding a )" inside)delim";

// §5.13.6 [lex.bool] and §5.13.7 [lex.nullptr].
auto b = true;
auto n = nullptr;

// §5.13.8 [lex.ext]: a user-defined literal is a literal and a suffix, and
// the suffix is an identifier or a string of them.
auto ud1 = 42_km;
auto ud2 = 1.5_deg;
auto ud3 = "text"_id;
auto ud4 = 'c'_ch;
