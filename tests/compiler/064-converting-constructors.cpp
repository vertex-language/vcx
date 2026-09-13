// Does an argument reach a class parameter through a converting constructor?
//
// §12.2.2.5 [over.match.conv], §12.2.4.2.3 [over.ics.user] -- an implicit
// conversion sequence may run one constructor: `f(0)` for `f(Zero)` is
// `f(Zero(0))`, and the temporary is built by the caller. The constructor
// may be a template whose parameter is deduced from the argument. Along
// the way, §11.8.4 hidden friends found only by argument-dependent lookup,
// and §12.2.2.3/3's rewritten candidates: `a != b` as `!(a == b)` and
// `a == b` as `b == a` when only the one operator== is declared.

struct Meters {
    int v;
    Meters(int n) : v(n) {}
};

int twice(Meters m) { return m.v * 2; }
int peek(const Meters& m) { return m.v; }

struct Tagged {
    int tag;
    template <typename T> Tagged(T t) : tag((int)sizeof(T) * 10 + (int)t) {}
};

int tagOf(Tagged t) { return t.tag; }

namespace geo {
    struct Zero {
        template <typename T> constexpr Zero(T) noexcept {}
    };

    struct Ord {
        signed char v;
        // Hidden friends: not members, not visible to ordinary lookup
        // outside the class, found because an Ord is an operand.
        friend bool operator<(const Ord a, Zero) { return a.v < 0; }
        friend bool operator>(Zero, const Ord a) { return a < 0; }
        friend bool operator==(const Ord a, Zero) { return a.v == 0; }
    };
}

struct Pt {
    int x, y;
    bool operator==(const Pt& o) const { return x == o.x && y == o.y; }
};

int main() {
    int r = 0;

    // by value and by const reference, through Meters(int)
    if (twice(21) == 42) r += 1;
    if (peek(7) == 7) r += 1;

    // through a constructor template: T deduced as char, then as int
    if (tagOf('A') == 10 + 65) r += 1;
    if (tagOf(3) == 40 + 3) r += 1;

    // hidden friends with a literal 0 converted to Zero
    geo::Ord neg{-1};
    geo::Ord eq{0};
    if (neg < 0) r += 1;
    if (0 > neg) r += 1;
    if (eq == 0) r += 1;

    // rewritten candidates on a class with only operator==
    Pt a{1, 2};
    Pt b{1, 3};
    if (a != b) r += 1;
    if (!(a != a)) r += 1;
    // and `eq != 0` -- !(eq == 0) through the hidden friend
    if (neg != 0) r += 1;
    if (!(eq != 0)) r += 1;

    return r; // eleven checks
}
