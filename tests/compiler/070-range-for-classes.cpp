// Does a range-based for run over a class the way §8.6.5 rewrites it?
//
// §8.6.5/1 [stmt.ranged] -- `for (decl : range)` over a class is
// `auto&& r = range; auto b = r.begin(), e = r.end(); for (; b != e; ++b)
// { decl = *b; ... }`, with begin and end found as members or else by
// argument-dependent lookup, and the iterator's !=, ++ and * being
// whatever it declares -- or the built-ins, when begin() returns a
// pointer. §12.2.2.3/3 -- an iterator with only operator== is compared
// with `!(b == e)`. §6.5.4 [basic.lookup.argdep] -- `begin(r)` finds
// ns::begin for an r declared in ns, which ordinary lookup does not see.

struct Range {
    int a[3];
    int* begin() { return a; }
    int* end() { return a + 3; }
};

struct Iter {
    const int* p;
    bool operator!=(const Iter& o) const { return p != o.p; }
    Iter& operator++() { ++p; return *this; }
    const int& operator*() const { return *p; }
};

struct Seq {
    int d[4];
    Iter begin() const { return Iter{d}; }
    Iter end() const { return Iter{d + 4}; }
};

namespace ns {
    struct Evens { int upto; };
    struct EvenIt {
        int v;
        int operator*() const { return v; }
        EvenIt& operator++() { v += 2; return *this; }
        bool operator==(const EvenIt& o) const { return v == o.v; }
    };
    EvenIt begin(const Evens&) { return EvenIt{0}; }
    EvenIt end(const Evens& e) { return EvenIt{e.upto}; }
}

int made = 0;
struct Counted {
    int n;
    Counted(int k) : n(k) { ++made; }
    ~Counted() { --made; }
};
struct Items {
    Counted c[2];
    Items() : c{1, 2} {}
    const Counted* begin() const { return c; }
    const Counted* end() const { return c + 2; }
};
Items items() { return Items(); }

int main() {
    int r = 0;

    // A pointer iterator from a member begin/end, by reference.
    Range rg{{10, 20, 30}};
    int s = 0;
    for (int& v : rg) { v += 1; s += v; }
    if (s == 63 && rg.a[0] == 11) r += 1;

    // A class iterator with != and a reference-returning *.
    Seq sq{{1, 2, 3, 4}};
    s = 0;
    for (auto v : sq) s += v;
    for (const int& v : sq) s += v * 10;
    if (s == 110) r += 1;

    // ADL begin/end, == only, * by value.
    ns::Evens ev{10};
    s = 0;
    for (int v : ev) s += v;
    if (s == 20) r += 1;

    // break and continue leave and re-enter the loop's own scope.
    s = 0;
    for (int v : sq) {
        if (v == 2) continue;
        if (v == 4) break;
        s += v;
    }
    if (s == 4) r += 1;

    // A temporary range lives for the whole statement (§6.7.7/6) and
    // its elements are destroyed with it, afterwards.
    s = 0;
    for (const Counted& c : items()) { s += c.n; if (made != 2) s = -1; }
    if (s == 3 && made == 0) r += 1;

    return r; // five checks
}
