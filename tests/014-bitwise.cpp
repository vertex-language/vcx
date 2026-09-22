// And, or, xor and complement.
#include <cstdio>

int main() {
    unsigned a = 0xF0F0u, b = 0x3C3Cu;
    std::printf("%x %x %x %x\n", a & b, a | b, a ^ b, ~a);
    return 0;
}
