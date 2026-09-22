// Widening: sign extension for signed types, zero extension for unsigned.
#include <cstdio>

int main() {
    signed char sc = -5;
    unsigned char uc = 251;
    int a = sc, b = uc;
    long long c = -5;
    unsigned long long d = 4294967291u;
    std::printf("%d %d %lld %llu\n", a, b, c, d);
    return 0;
}
