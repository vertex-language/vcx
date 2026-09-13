// Do pointers to members point, and static members live once?
//
// §7.6.2.2/3 [expr.unary.op] -- `&C::m` on a non-static member is a
// pointer to member. §7.6.4 [expr.mptr.oper] -- `obj.*pm` and `p->*pm`
// reach the member, and a pointer to member function is called with the
// object bound. §7.3.13 -- a null pointer constant converts to a null
// member pointer, which compares unequal to any real one. Under the
// Microsoft convention (tests/abi has the sizes) a data member pointer
// is the member's offset in four bytes, null being -1 since zero is an
// offset, and a function member pointer is the function's address.
// §11.4.9.3 [class.static.data] -- a static member is one object; an
// inline one initialized in the class is defined by that (§11.4.9.3/4).
// `step` says inline because a plain `static const int` initialized in the
// class is only declared there, and taking its address below is an
// odr-use that needs a definition somewhere (§6.3/10): cl lets that go,
// and ld64 does not. `limit` is constexpr and so inline already.

struct S {
    int a;
    int b;
    int get() const { return a + b; }
    int twice() const { return 2 * a; }
    int scale(int k) { a *= k; return a; }
};

struct Counter {
    static int count;
    static int made() { return count; }
    Counter() { ++count; }
    ~Counter() { --count; }
};
int Counter::count = 0;

struct Config {
    static constexpr int limit = 8;
    static inline const int step = 2;
};

int apply(S& s, int (S::*f)(int), int k) { return (s.*f)(k); }

int main() {
    int r = 0;

    S s{3, 4};
    S* p = &s;
    int S::*pm = &S::b;
    int (S::*pf)() const = &S::get;
    if (s.*pm == 4 && p->*pm == 4 && (s.*pf)() == 7 && (p->*pf)() == 7) r += 1;

    // Reassigned, and written through.
    pf = &S::twice;
    pm = &S::a;
    s.*pm = 10;
    if ((s.*pf)() == 20 && s.a == 10 && apply(s, &S::scale, 3) == 30) r += 1;

    // Null member pointers.
    int S::*none = nullptr;
    int S::*zero = 0;
    if (none == nullptr && zero == none && pm != none && sizeof(pm) == 4 && sizeof(pf) == 8) r += 1;

    // A static member counts objects across scopes; a static function reads it.
    {
        Counter a, b;
        Counter c;
        if (Counter::made() == 3) r += 1;
    }
    if (Counter::count == 0) r += 1;

    // A const static member initialized in the class has an address.
    const int* ps = &Config::step;
    if (Config::limit * Config::step == 16 && *ps == 2) r += 1;

    return r; // six checks
}
