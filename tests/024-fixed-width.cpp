// The <cstdint> types, their sizes and limits.
#include <cstdint>
#include <cstdio>

int main() {
    std::int8_t a = INT8_MIN;
    std::uint16_t b = UINT16_MAX;
    std::int32_t c = INT32_MIN;
    std::uint64_t d = UINT64_MAX;
    std::printf("%d %u %d %llu\n", a, b, c, (unsigned long long)d);
    std::printf("%zu %zu %zu %zu\n", sizeof a, sizeof b, sizeof c, sizeof d);
    return 0;
}
