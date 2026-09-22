// Integer multiplication and its signs.
#include <cstdio>

int mul(int a, int b) { return a * b; }

int main() {
    std::printf("%d %d %d %d\n", mul(6, 7), mul(-6, 7), mul(-6, -7), mul(12345, 0));
    return 0;
}
