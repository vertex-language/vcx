// A union's members share storage.
#include <cstdio>
#include <cstring>

union Bits {
    unsigned u;
    unsigned char bytes[4];
};

int main() {
    Bits b;
    b.u = 0x11223344u;
    std::printf("%zu %x\n", sizeof(Bits), b.bytes[0]);
    float f = 1.0f;
    unsigned raw;
    std::memcpy(&raw, &f, sizeof raw);
    std::printf("%x\n", raw);
    return 0;
}
