// Does a class come back from a call the way the ABI says it does?
//
// Three classes, three conventions on x64 Windows. A plain eight-byte
// struct comes back in RAX. A struct too big for a register comes back
// through storage the caller supplies. And a class with a constructor of
// its own comes back through a hidden pointer whatever its size -- and for
// a member function that pointer is the *second* argument, after `this`,
// which is the case a compiler gets wrong by treating the result slot as
// always-first. All three are exercised through a member function.

struct Pair { int a, b; };
struct Wide { long long v[3]; };
struct Made {
    int n;
    Made(int k) : n(k) {}
};

struct Factory {
    int base;
    Factory(int b) : base(b) {}
    Pair pair() const { Pair p; p.a = base; p.b = base + 1; return p; }
    Wide wide() const { Wide w; w.v[0] = base; w.v[1] = 2; w.v[2] = 3; return w; }
    Made made() const { return Made(base * 10); }
    static Pair fixed() { Pair p; p.a = 1; p.b = 2; return p; }
};

Made free_made(int k) { return Made(k + 1); }

int main() {
    Factory f(5);
    Pair p = f.pair();          // 5, 6
    Wide w = f.wide();          // 5, 2, 3
    Made m = f.made();          // 50
    Pair q = Factory::fixed();  // 1, 2
    Made fm = free_made(6);     // 7
    return p.a + p.b + (int)(w.v[0] + w.v[1] + w.v[2]) + m.n + q.a + q.b + fm.n - 50;  // 11 + 10 + 50 + 3 + 7 - 50 = 31
}
