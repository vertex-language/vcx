// §7.5.5 [expr.prim.lambda] -- the capture list, the parameter list nobody
// writes, and the three places a lambda's grammar hides a whole declaration.

int global;

void captures(int a, int b) {
    // §7.5.5.3 [expr.prim.lambda.capture]: every capture form.
    auto c1 = [] { return 0; };
    auto c2 = [a] { return a; };
    auto c3 = [&a] { return a; };
    auto c4 = [=] { return a + b; };
    auto c5 = [&] { return a + b; };
    auto c6 = [=, &a] { return a + b; };
    auto c7 = [&, a] { return a + b; };
    auto c8 = [a, &b] { return a + b; };

    // An init-capture is a declaration: §7.5.5.3/6.
    auto c9  = [x = a + b] { return x; };
    auto c10 = [&r = a] { return r; };
    auto c11 = [x = a, &y = b] { return x + y; };

    (void)c1; (void)c2; (void)c3; (void)c4; (void)c5;
    (void)c6; (void)c7; (void)c8; (void)c9; (void)c10; (void)c11;
}

void bodies() {
    // The declarator half: parameters, trailing return, and the specifiers
    // that sit between them.
    auto d1 = [](int x) { return x; };
    auto d2 = [](int x) -> double { return x; };
    auto d3 = [](int x) mutable { return ++x; };
    auto d4 = [](int x) noexcept { return x; };
    auto d5 = [](int x) constexpr { return x; };
    auto d6 = [](int x) mutable noexcept -> int { return x; };
    auto d7 = []() static { return 0; };            // C++23 [expr.prim.lambda]

    // A generic lambda is a template, spelled two ways: §7.5.5.1/6.
    auto g1 = [](auto x) { return x; };
    auto g2 = []<typename T>(T x) { return x; };
    auto g3 = []<typename T>(T x) -> T { return x; };
    auto g4 = [](auto&&... xs) { return sizeof...(xs); };

    // An attribute may appear on the call operator, and the whole thing is
    // an expression, so it can be called where it is written.
    auto a1 = []() [[nodiscard]] { return 0; };
    [] { return 0; }();
    [](int x) { return x; }(1);

    (void)d1; (void)d2; (void)d3; (void)d4; (void)d5; (void)d6; (void)d7;
    (void)g1; (void)g2; (void)g3; (void)g4; (void)a1;
}

// Nested, and in default arguments and initializers -- the places where a
// lambda's braces sit inside another construct's.
int nested = [] { return [] { return 1; }(); }();
void deflt(int f = [] { return 1; }());
struct Member { int v = [] { return 2; }(); };
