// Raw string literals and the UTF string literals' element counts.
#include <cstdio>

int main() {
    const char* raw = R"(a\nb"c)";
    const char* delim = R"xy(paren)"inside)xy";
    std::printf("%s %s\n", raw, delim);
    std::printf("%zu %zu %zu %zu\n", sizeof(u8"é"), sizeof(u"é"), sizeof(U"é"), sizeof("é"));
    return 0;
}
