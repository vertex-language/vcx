// Does a lambda make a closure that captures, calls and converts?
//
// §7.5.5 [expr.prim.lambda] -- a lambda-expression is a prvalue of a
// closure type with an operator() that is the body. §7.5.5.3 -- a capture
// by copy is a member holding a copy made where the lambda is written
// (/10), by reference a member referring to the entity (/12); a
// capture-default captures what the body uses (/7-8); an init-capture
// declares a member from its initializer (/6). §7.5.5.2/8 -- a closure
// with no captures converts to a pointer to a function with the same
// parameters and result. §7.5.5.1/3 -- operator() is const unless the
// lambda is mutable, so a copy is modifiable only then.

int apply(int (*f)(int), int x) { return f(x); }
int twice(int (*f)(int, int), int a, int b) { return f(a, b) * 2; }

struct Tag {
    int n;
    Tag(int k) : n(k) {}
    int get() const { return n; }
};

int main() {
    int r = 0;

    // By copy: the value at the point of the lambda, not later.
    int k = 3;
    auto add = [k](int x) { return x + k; };
    k = 100;
    if (add(4) == 7) r += 1;

    // By reference: the entity itself, read and written.
    int count = 0;
    auto bump = [&count](int by) { count += by; return count; };
    bump(2);
    bump(3);
    if (count == 5 && bump(0) == 5) r += 1;

    // Capture-defaults: [=] copies what is used, [&] refers to it.
    int a = 10, b = 20;
    auto sum = [=]() { return a + b; };
    auto swapish = [&]() { int t = a; a = b; b = t; };
    swapish();
    if (sum() == 30 && a == 20 && b == 10) r += 1;

    // An init-capture, by value and by reference.
    auto scaled = [m = k * 2](int x) { return x * m; };
    auto alias = [&c = count]() { c += 10; return c; };
    if (scaled(1) == 200 && alias() == 15 && count == 15) r += 1;

    // A class captured by copy, used through its members.
    Tag t(7);
    auto tagged = [t]() { return t.get(); };
    if (tagged() == 7) r += 1;

    // mutable: the copy is the closure's own to change, call to call.
    auto counter = [n = 0]() mutable { return ++n; };
    counter();
    counter();
    if (counter() == 3) r += 1;

    // No captures: a function pointer, by conversion and by initialization.
    auto plain = [](int x) { return x * 2; };
    int (*fp)(int) = plain;
    auto both = [](int x, int y) { return x - y; };
    if (apply(plain, 5) == 10 && fp(6) == 12 && twice(both, 9, 4) == 10) r += 1;

    // A lambda inside a lambda, capturing the outer's capture.
    int base = 1000;
    auto outer = [base](int x) {
        auto inner = [base, x](int y) { return base + x + y; };
        return inner(1);
    };
    if (outer(10) == 1011) r += 1;

    // Void result, and a call for its side effect alone.
    int hits = 0;
    auto poke = [&hits]() { ++hits; };
    poke();
    poke();
    if (hits == 2) r += 1;

    return r; // nine checks
}
