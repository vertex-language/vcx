// Unsigned arithmetic wraps modulo 2^32.
#include <cstdio>

int main() {
    unsigned a = 4294967295u;
    unsigned b = a + 2u;
    unsigned c = 0u - 1u;
    std::printf("%u %u\n", b, c);
    return 0;
}
