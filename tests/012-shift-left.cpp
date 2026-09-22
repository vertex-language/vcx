// Left shifts.
#include <cstdio>

unsigned shl(unsigned v, int n) { return v << n; }

int main() {
    std::printf("%u %u %u\n", shl(1, 0), shl(1, 31), shl(0x12345678u, 4));
    return 0;
}
