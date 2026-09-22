// Right shifts: arithmetic for a signed value, logical for an unsigned one.
#include <cstdio>

int sar(int v, int n) { return v >> n; }
unsigned shr(unsigned v, int n) { return v >> n; }

int main() {
    std::printf("%d %d %u\n", sar(-256, 4), sar(256, 4), shr(0x80000000u, 31));
    return 0;
}
