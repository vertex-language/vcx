// Bit-fields: packing, truncation, signedness.
#include <cstdio>

struct Flags {
    unsigned a : 3;
    unsigned b : 5;
    int c : 4;
    unsigned d : 1;
};

int main() {
    Flags f{};
    f.a = 9;   // keeps 1
    f.b = 31;
    f.c = -3;
    f.d = 1;
    std::printf("%u %u %d %u %zu\n", f.a, f.b, f.c, f.d, sizeof(Flags));
    return 0;
}
