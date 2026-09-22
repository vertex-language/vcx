// 64-bit integers: arithmetic past 32 bits.
#include <cstdio>

int main() {
    long long a = 3000000000LL;
    long long b = a * 3;
    unsigned long long c = 0xFFFFFFFFFFFFFFFFull;
    std::printf("%lld %llu %lld\n", b, c, b / -7);
    return 0;
}
