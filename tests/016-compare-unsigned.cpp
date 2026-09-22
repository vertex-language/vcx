// Unsigned comparisons: a large value is greater, not negative.
#include <cstdio>

void cmp(unsigned a, unsigned b) {
    std::printf("%d%d%d%d\n", a < b, a <= b, a > b, a >= b);
}

int main() {
    cmp(1u, 0xFFFFFFFFu);
    cmp(0x80000000u, 1u);
    return 0;
}
