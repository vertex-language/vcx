// Unsigned division and remainder, with values above INT_MAX.
#include <cstdio>

unsigned quo(unsigned a, unsigned b) { return a / b; }
unsigned rem(unsigned a, unsigned b) { return a % b; }

int main() {
    std::printf("%u %u\n", quo(4000000000u, 7u), rem(4000000000u, 7u));
    return 0;
}
