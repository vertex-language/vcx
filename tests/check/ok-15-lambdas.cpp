// §7.5.5 [expr.prim.lambda] -- a lambda is not a function. It is an object
// of a class the compiler makes up, whose public inline `operator()` has the
// lambda's parameters and its return type, and everything that makes one
// usable follows from that member existing: calling a lambda is a call to
// it, and passing one anywhere is passing an object.

// §7.5.5.1/4 -- with no trailing return type the return is deduced from the
// body, the way `auto` is; with one, it is written down.
int deduced_returns() {
    auto plus_one = [](int x) { return x + 1; };
    auto as_double = [](int x) -> double { return x; };
    auto nothing = []() { };
    nothing();
    return plus_one(1) + static_cast<int>(as_double(2));
}

// §7.5.5.3 [expr.prim.lambda.capture] -- by copy, by reference, and the
// init-capture, which declares a name that exists nowhere else.
int captures() {
    int a = 1;
    int b = 2;

    auto by_copy = [a](int x) { return a + x; };
    auto by_ref = [&b]() { b = 5; };
    auto by_default_copy = [=]() { return a + b; };
    auto by_default_ref = [&]() { return a + b; };
    auto init_capture = [n = 10]() { return n; };
    auto ref_init_capture = [&r = a]() { return r; };

    by_ref();
    return by_copy(1) + by_default_copy() + by_default_ref()
        + init_capture() + ref_init_capture() + b;
}

// §7.5.5.1/3 -- the call operator is const unless `mutable` says otherwise,
// which is what makes writing to a by-value capture legal here and not above.
int mutable_lambda() {
    int a = 1;
    auto counter = [a]() mutable { a = a + 1; return a; };
    return counter();
}

// §7.5.5.1/6 -- an `auto` parameter is an invented template parameter, so a
// generic lambda's body is dependent for the same reason a template's is.
int generic() {
    auto identity = [](auto x) { return x; };
    auto sum = [](auto x, auto y) { return x + y; };
    return identity(1) + static_cast<int>(sum(1.5, 2.5));
}

// A lambda is an expression, so it may be called where it is written, nested
// inside another, or passed to a template that deduces its type.
template <typename F> int apply(F f, int x) { return f(x); }

int as_a_value() {
    int immediate = [](int x) { return x * 2; }(21);
    auto nested = []() { return []() { return 7; }(); };
    return immediate + nested() + apply([](int y) { return y + 1; }, 1);
}

// The body is checked where it is written, which is what makes a lambda a
// place errors can be found rather than a hole they fall through.
int uses_its_body() {
    auto uses_a_member = [](int n) {
        int total = 0;
        for (int i = 0; i < n; ++i) total += i;
        return total;
    };
    return uses_a_member(5);
}
