// Does a template become code?
//
// §13.9 -- nothing runs a template; what runs is a specialization,
// instantiated where the program needs it with the arguments that use
// supplied. A function template called with an int and with a double is
// two functions in the object, and a class template named with two
// arguments is two classes with two layouts and two sets of members --
// each a COMDAT, since every unit that uses one may make its own copy.

template <typename T> T twice(T x) { return x + x; }

template <typename T> struct Box {
    T v;
    Box(T x) : v(x) {}
    T get() const { return v; }
    void set(const T &x) { v = x; }
    bool holds(T x) const { return v == x; }
};

template <typename T> Box<T> boxed(T x) { return Box<T>(x); }

template <typename A, typename B> A first(A a, B) { return a; }

struct Pair { int lo, hi; };

int main() {
    Box<int> bi(20);
    Box<double> bd(0.25);
    Box<Pair> bp(Pair{3, 4});
    bi.set(bi.get() + 1);              // 21
    bd.set(bd.get() * 2);              // 0.5
    Box<long long> big = boxed(1000000000000LL);
    int wide = (int)(big.get() / 1000000000);   // 1000
    return twice(3) + (int)twice(2.5) + bi.get() + (int)(bd.get() * 4) + bp.get().lo + bp.get().hi
        + boxed(4).get() + first(1, 'c') + (int)first(2.0, bi) + (bi.holds(21) ? 10 : 0) + wide - 1000;
    // 6 + 5 + 21 + 2 + 3 + 4 + 4 + 1 + 2 + 10 + 1000 - 1000 = 58
}
