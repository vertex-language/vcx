// __int128, long double, and <bit>: popcount, countl_zero, rotl, bit_cast, has_single_bit.
#include <bit>
#include <cstdint>
#include <cstdio>

int main() {
    unsigned __int128 big = (unsigned __int128)1 << 100;
    big += 12345;
    unsigned long long hi = (unsigned long long)(big >> 64), lo = (unsigned long long)big;
    __int128 neg = -(__int128)big / 3;
    long double ld = 1.0L / 4;
    std::printf("%llx %llu %d %Lg\n", hi, lo, neg < 0, ld);
    std::uint32_t v = 0x00F0F000u;
    std::printf("%d %d %d %x %d\n", std::popcount(v), std::countl_zero(v), std::countr_zero(v), std::rotl(v, 8),
                std::has_single_bit(64u));
    std::printf("%x %u\n", std::bit_cast<std::uint32_t>(1.0f), std::bit_ceil(100u));
    return 0;
}
