// Narrowing integer conversions keep the low bits.
#include <cstdio>

int main() {
    int big = 0x12345678;
    unsigned char c = (unsigned char)big;
    short s = (short)big;
    unsigned u = (unsigned)0x1234567890ull;
    std::printf("%x %x %x\n", c, (unsigned short)s, u);
    return 0;
}
