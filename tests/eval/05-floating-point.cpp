// §7.7 -- floating-point in a constant expression. The evaluator's answers
// have to be the target's arithmetic, not an approximation of it, because
// they end up in the object file as the program's constants.

// Values with an exact binary representation, which are the ones an
// assertion may compare for equality without being a claim about rounding.
static_assert(1.0 + 1.0 == 2.0);
static_assert(3.5 + 0.5 == 4.0);
static_assert(1.0 / 4.0 == 0.25);
static_assert(0.5 * 0.5 == 0.25);
static_assert(10.0 - 2.5 == 7.5);
static_assert(-2.5 + 2.5 == 0.0);

// §7.4 [expr.arith.conv] -- the usual arithmetic conversions bring an int
// operand up to the floating type rather than the other way round.
static_assert(1 + 1.5 == 2.5);
static_assert(3 / 2.0 == 1.5);
static_assert(2.0 * 3 == 6.0);
static_assert(7 / 2 == 3);          // both int: integer division
static_assert(7 / 2.0 == 3.5);      // one double: floating division

// Comparison, which is the same operator on a different type.
static_assert(1.5 < 2.5);
static_assert(2.5 > 1.5);
static_assert(1.5 <= 1.5);
static_assert(1.5 >= 1.5);
static_assert(1.5 != 2.5);
static_assert(0.25 == 0.25);

// A constexpr function over doubles, run at compile time.
constexpr double power(double base, int exp) {
    double r = 1.0;
    for (int i = 0; i < exp; ++i) r *= base;
    return r;
}
static_assert(power(2.0, 0) == 1.0);
static_assert(power(2.0, 10) == 1024.0);
static_assert(power(0.5, 2) == 0.25);

constexpr double absolute(double x) { return x < 0 ? -x : x; }
static_assert(absolute(-2.5) == 2.5);
static_assert(absolute(2.5) == 2.5);
static_assert(absolute(0.0) == 0.0);

// The float suffix, and the hexadecimal form of §5.13.4 whose value is
// exact by construction.
static_assert(1.5f == 1.5);
static_assert(0x1p4 == 16.0);
static_assert(0x1.8p3 == 12.0);
static_assert(0x1p-2 == 0.25);
