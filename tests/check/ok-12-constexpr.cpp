// §7.7 [expr.const] -- a constant expression is checked where it is written,
// so a wrong one is a diagnostic and a right one is silence.
constexpr int square(int x) { return x * x; }

constexpr int factorial(int n) {
    int r = 1;
    for (int i = 2; i <= n; ++i) r *= i;
    return r;
}

constexpr int fib(int n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}

static_assert(square(7) == 49);
static_assert(factorial(5) == 120);
static_assert(fib(10) == 55);
static_assert(1 + 2 * 3 == 7);
static_assert((1 << 8) == 256);

constexpr int folded = square(3) + factorial(3);
int array[folded];

consteval int always_constant(int x) { return x + 1; }
constexpr int from_consteval = always_constant(1);

// §9.2.6 -- constinit says the initializer is constant without making the
// object one.
constinit int mutable_but_statically_initialized = square(2);
