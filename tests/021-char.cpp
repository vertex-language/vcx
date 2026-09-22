// char: arithmetic on characters, and signed char's range.
#include <cstdio>

int main() {
    char c = 'a';
    c += 2;
    signed char s = -128;
    unsigned char u = 255;
    std::printf("%c %d %d %d\n", c, c, s, u);
    return 0;
}
