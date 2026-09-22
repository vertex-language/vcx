// std::bitset: set, test, count, shifts, and conversion.
#include <bitset>
#include <cstdio>

int main() {
    std::bitset<16> b(0b1011);
    b.set(8);
    b.flip(0);
    std::bitset<16> c = (b << 2) ^ std::bitset<16>(0xFF);
    std::printf("%s %zu %d %lu\n", b.to_string().c_str(), b.count(), (int)b.test(3), c.to_ulong());
    return 0;
}
