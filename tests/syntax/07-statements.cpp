// §8 [stmt.stmt] -- every statement, and the condition grammar that three
// of them share.

int fn(int);
struct Res { int ok; int v; explicit operator bool() const; };
Res make();

void statements(int a) {
    // §8.2 labelled: an identifier label, and the two case forms of §8.5.3.
    // §8.3 expression-statement, including the empty one.
    ;
    a;
    goto later;
later: ;

    // §8.4 compound-statement nests without limit.
    { { { a; } } }

    // §8.5 selection, with the C++17 init-statement and the C++23 label rule
    // that lets a label end a block.
    if (a) a; else a;
    if (int x = fn(a); x) a;
    if (int x = fn(a); x) a; else a;
    if constexpr (sizeof(int) == 4) a;
    if constexpr (true) a; else a;

    // §8.5.2/2 [stmt.if] -- `if consteval` takes no condition and therefore
    // no parentheses, and both of its branches are compound-statements.
    if consteval { a; }
    if consteval { a; } else { a; }
    if !consteval { a; }
    if !consteval { a; } else { a; }

    switch (a) {
        case 0:
        case 1: a; break;
        case 2 ... 4: a; break;             // GNU case range, tolerated
        default: break;
    }
    switch (int x = fn(a); x) { default: break; }

    // §8.6 iteration. A condition may declare, in all three loops that have
    // one, and a for-init may be a declaration or an expression.
    while (a) a;
    while (int x = fn(a)) (void)x;
    do a; while (a);
    for (;;) break;
    for (int i = 0; i < 3; ++i) a;
    for (int i = 0, j = 1; i < j; ++i, --j) a;
    for (a = 0; a; ) break;

    // §8.6.5 [stmt.ranged] -- the ranged loop, in every declaration form
    // and with the init-statement C++20 added.
    int arr[3] = {1, 2, 3};
    for (int v : arr) (void)v;
    for (auto v : arr) (void)v;
    for (auto& v : arr) v = 0;
    for (const auto& v : arr) (void)v;
    for (auto&& v : arr) (void)v;
    for (auto v : {1, 2, 3}) (void)v;
    for (int i = 0; auto v : arr) { (void)i; (void)v; }
    for (auto v : arr) { if (v) continue; else break; }

    // The `:` of a conditional in a three-clause loop's condition is not the
    // one that makes a loop ranged, which is the only thing that tells the
    // two apart before the declaration is parsed.
    for (int i = 0; i < (a ? 1 : 2); ++i) (void)i;

    // §8.7 jump.
    for (;;) { continue; }
    for (;;) { break; }
    return;
}

int returns() {
    return 0;
}

// §8.9 [stmt.ambig] -- the declaration/expression ambiguity, resolved toward
// a declaration whenever the tokens allow one.
void ambiguity(int a) {
    int (x2);           // a declaration of x2, not a parenthesized expression
    (void)a; (void)x2;

    // The other half of §8.9, and the half that decides the vexing parse:
    // `int x1(0);` is a declaration with a parenthesized initializer, because
    // `0` cannot begin a parameter-declaration -- where `S s(T());` declares
    // a function, because `T()` can.
    int x1(0);
    (void)x1;
}

// §8.8 [stmt.dcl] -- a declaration is a statement, which is why the block
// above could hold one at all.
void declarations() {
    int a = 0;
    static int s = 0;
    extern int e;
    const int c = 0;
    using A = int;
    typedef int B;
    struct Local { int m; };
    enum LocalE { LE };
    (void)a; (void)s; (void)c;
}
