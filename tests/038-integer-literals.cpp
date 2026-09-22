// Integer literals: bases, digit separators, suffixes.
#include <cstdio>

int main() {
    std::printf("%d %d %d %d\n", 0x1F, 017, 0b1011, 1'000'000);
    std::printf("%llu %zu\n", 18446744073709551615ull, sizeof(10l));
    return 0;
}
