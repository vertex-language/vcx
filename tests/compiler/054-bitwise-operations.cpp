// 054-bitwise-operations.cpp
// Tests bitwise operators and compound bitwise assignments:
// - &, |, ^, ~
// - <<, >> on signed and unsigned integers
// - &=, |=, ^=, <<=, >>=
// - 32-bit and 64-bit integer bitwise operations

int test_basic_bitwise() {
    int a = 0b1100; // 12
    int b = 0b1010; // 10

    int and_val = a & b; // 0b1000 = 8
    int or_val  = a | b; // 0b1110 = 14
    int xor_val = a ^ b; // 0b0110 = 6
    int not_val = ~a;    // -13

    return and_val + or_val + xor_val + (not_val & 0xFF); // 8 + 14 + 6 + 243 = 271
}

int test_shifts() {
    unsigned int u = 1;
    u = u << 4; // 16
    u = u >> 2; // 4

    int s = -16;
    int s_shr = s >> 2; // -4 (arithmetic shift)

    return (int)u + s_shr; // 4 + (-4) = 0
}

int test_compound_bitwise() {
    int v = 0x0F;
    v |= 0xF0;  // 0xFF = 255
    v &= 0xAA;  // 0xAA = 170
    v ^= 0x55;  // 0xFF = 255
    v <<= 2;    // 1020
    v >>= 1;    // 510
    return v;   // 510
}

int test_64bit_bitwise() {
    unsigned long long mask = 0x0000000100000000ULL;
    unsigned long long val = 0x00000000FFFFFFFFULL;
    unsigned long long combined = mask | val; // 0x1FFFFFFFF
    combined ^= 0x0000000100000000ULL;        // 0xFFFFFFFF
    return (int)(combined & 0xFF);             // 255
}

int main() {
    int r1 = test_basic_bitwise();    // 271
    int r2 = test_shifts();           // 0
    int r3 = test_compound_bitwise(); // 510
    int r4 = test_64bit_bitwise();    // 255

    // Total: 271 + 0 + 510 + 255 = 1036
    // 1036 % 256 = 12
    return (r1 + r2 + r3 + r4) % 256;
}
