// Do the GNU compilers' own spellings mean what they mean there?
//
// This file is GNU dialect, and lives in tests/compiler/gnu: cl has none of
// it. The standard library headers written for gcc and clang use every one
// of these without declaring them.
//
// A library builtin is the library function under a reserved name, callable
// with no declaration: __builtin_fabs is fabs. An expression builtin is
// computed where it stands -- the floating classifications for any
// floating type, the bit counts, the overflow checks. An asm label names
// the symbol a declaration is known by in the object file, which is how the
// macOS SDK chooses between two versions of one C function.

// The label is the object file's symbol exactly, so it carries the
// container's prefix: "_abs" on Mach-O, "abs" on ELF.
#define STR_(x) #x
#define STR(x) STR_(x)
extern "C" int magnitude(int) __asm(STR(__USER_LABEL_PREFIX__) "abs");

int main() {
    int r = 0;

    // Library builtins.
    if (__builtin_fabs(-2.5) == 2.5 && __builtin_fabsf(-1.5f) == 1.5f) r += 1;
    if (__builtin_abs(-7) == 7 && __builtin_llabs(-8LL) == 8) r += 1;
    if (__builtin_strlen("vertex") == 6) r += 1;

    // Floating classification, in both widths.
    double zero = 0.0;
    double nan = zero / zero, inf = 1.0 / zero;
    float fsmall = 1e-40f;                                    // subnormal as a float
    if (__builtin_isnan(nan) && !__builtin_isnan(inf) && __builtin_isinf(-inf)) r += 1;
    if (__builtin_isfinite(1.0) && !__builtin_isfinite(inf) && !__builtin_isnormal(fsmall)) r += 1;
    if (__builtin_signbit(-0.0) && !__builtin_signbit(0.0f)) r += 1;
    if (__builtin_fpclassify(1, 2, 3, 4, 5, fsmall) == 4 && __builtin_fpclassify(1, 2, 3, 4, 5, nan) == 1) r += 1;
    if (__builtin_isgreater(2.0, 1.0) && !__builtin_isless(nan, 1.0) && __builtin_isunordered(nan, 1.0)) r += 1;
    if (__builtin_isinf(__builtin_huge_val()) && __builtin_isnan(__builtin_nan(""))) r += 1;

    // Bits.
    if (__builtin_clz(1u) == 31 && __builtin_ctzll(8ULL) == 3 && __builtin_popcount(0xF0u) == 4) r += 1;
    if (__builtin_parity(7u) == 1 && __builtin_bswap32(0x11223344u) == 0x44332211u) r += 1;
    if (__builtin_bswap16(0x1122) == 0x2211 && __builtin_bswap64(1ULL) == (1ULL << 56)) r += 1;

    // Overflow.
    int sum;
    unsigned small;
    if (__builtin_add_overflow(0x7fffffff, 1, &sum) && sum == (int)0x80000000) r += 1;
    if (__builtin_sub_overflow(1u, 2u, &small) && !__builtin_mul_overflow(6, 7, &sum) && sum == 42) r += 1;

    // Hints are nothing.
    if (__builtin_expect(r > 0, 1) && !__builtin_constant_p(r)) r += 1;

    // The asm label: magnitude is abs.
    if (magnitude(-9) == 9) r += 1;

    return r;   // sixteen checks
}
