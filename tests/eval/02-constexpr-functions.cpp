// §9.2.6 [dcl.constexpr] and §7.7 -- a constexpr function is a function the
// evaluator can run, and running it is what this file does. Every assertion
// here is a whole program executed at compile time.

constexpr int square(int x) { return x * x; }
constexpr int add(int a, int b) { return a + b; }

static_assert(square(0) == 0);
static_assert(square(7) == 49);
static_assert(square(-7) == 49);
static_assert(add(square(3), square(4)) == 25);

// Recursion: the evaluator has to keep a call stack.
constexpr int fib(int n) {
    if (n < 2) return n;
    return fib(n - 1) + fib(n - 2);
}
static_assert(fib(0) == 0);
static_assert(fib(1) == 1);
static_assert(fib(10) == 55);
static_assert(fib(20) == 6765);

constexpr int gcd(int a, int b) {
    if (b == 0) return a;
    return gcd(b, a % b);
}
static_assert(gcd(12, 18) == 6);
static_assert(gcd(17, 5) == 1);
static_assert(gcd(270, 192) == 6);

// §7.7/5 -- a loop and a mutable local are allowed since C++14, which is
// most of what makes constexpr usable and all of what makes it hard.
constexpr int factorial(int n) {
    int r = 1;
    for (int i = 2; i <= n; ++i) r *= i;
    return r;
}
static_assert(factorial(0) == 1);
static_assert(factorial(1) == 1);
static_assert(factorial(5) == 120);
static_assert(factorial(10) == 3628800);

constexpr int sum_to(int n) {
    int s = 0;
    int i = 1;
    while (i <= n) { s += i; ++i; }
    return s;
}
static_assert(sum_to(0) == 0);
static_assert(sum_to(100) == 5050);

constexpr int count_down(int n) {
    int c = 0;
    do { --n; ++c; } while (n > 0);
    return c;
}
static_assert(count_down(5) == 5);
static_assert(count_down(1) == 1);

// A loop with a break and a continue, so the evaluator's control flow is
// exercised rather than only its arithmetic.
constexpr int first_multiple(int of, int above) {
    for (int i = above; ; ++i) {
        if (i % of != 0) continue;
        return i;
    }
}
static_assert(first_multiple(7, 50) == 56);

constexpr int digits(int n) {
    int d = 0;
    if (n == 0) return 1;
    if (n < 0) n = -n;
    while (n > 0) { n /= 10; ++d; }
    return d;
}
static_assert(digits(0) == 1);
static_assert(digits(9) == 1);
static_assert(digits(10) == 2);
static_assert(digits(-12345) == 5);

// Mutual recursion.
constexpr bool is_odd(int n);
constexpr bool is_even(int n) { return n == 0 ? true : is_odd(n - 1); }
constexpr bool is_odd(int n) { return n == 0 ? false : is_even(n - 1); }
static_assert(is_even(10));
static_assert(is_odd(7));

// §9.2.6/4 -- consteval is a constexpr function that may only be called in
// a constant expression, which is the only place this file calls anything.
consteval int always(int x) { return x + 1; }
static_assert(always(41) == 42);
