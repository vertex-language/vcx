// §7.7 [expr.const] -- the operators, on the type every one of them is
// defined for first.
static_assert(1 + 2 == 3);
static_assert(10 - 4 == 6);
static_assert(3 * 7 == 21);
static_assert(20 / 4 == 5);
static_assert(20 % 6 == 2);

// §7.6/4 -- the operators bind the way they are written, and parentheses
// are the only thing that changes it.
static_assert(1 + 2 * 3 == 7);
static_assert((1 + 2) * 3 == 9);
static_assert(2 + 3 * 4 - 5 == 9);
static_assert(100 / 10 / 2 == 5);
static_assert(2 * 3 % 4 == 2);

// §7.6.5/4 -- integer division truncates toward zero, and the remainder
// takes the sign of the dividend. Both were implementation-defined in C89
// and are not now.
static_assert(7 / 2 == 3);
static_assert(-7 / 2 == -3);
static_assert(7 / -2 == -3);
static_assert(-7 / -2 == 3);
static_assert(7 % 2 == 1);
static_assert(-7 % 2 == -1);
static_assert(7 % -2 == 1);
static_assert(-7 % -2 == -1);

// §7.6.2.2 -- unary operators, and the double negation that is not a no-op
// for the parser.
static_assert(-(-5) == 5);
static_assert(+5 == 5);
static_assert(- -5 == 5);

// §7.6.7 [expr.shift] -- the left shift is defined on the value, and the
// right shift of a negative signed value is arithmetic since C++20.
static_assert((1 << 0) == 1);
static_assert((1 << 8) == 256);
static_assert((256 >> 4) == 16);
static_assert((-8 >> 1) == -4);
static_assert((1u << 31) == 2147483648u);

// §7.6.10 through §7.6.12 -- the bitwise operators.
static_assert((0xF0 & 0xAA) == 0xA0);
static_assert((0x0F | 0xF0) == 0xFF);
static_assert((0xFF ^ 0x0F) == 0xF0);
static_assert((~0) == -1);
static_assert((~0u) == 4294967295u);

// §7.6.13 and §7.6.14 -- && and || are sequenced and short-circuit, which
// a constant evaluator has to honour: the right operand of a false && is
// not evaluated, so a division by zero there is not an error.
static_assert(true && true);
static_assert(!(true && false));
static_assert(false || true);
static_assert(!(false || false));
static_assert(!false);
static_assert(false || (1 == 1));

// §7.6.16 -- the conditional, including nested.
static_assert((1 ? 10 : 20) == 10);
static_assert((0 ? 10 : 20) == 20);
static_assert((1 ? 1 ? 1 : 2 : 3) == 1);

// §7.6.6 -- signed wraparound is not UB in a constant expression when it
// does not occur; these stay inside the range.
static_assert(2147483647 - 1 == 2147483646);
static_assert(0x7fffffff + 1 == -2147483648);
