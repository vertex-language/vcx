struct Pair { int a, b; };
struct Wide { long long v[3]; };
struct Made {
    int n;
    Made(int k) : n(k) {}
};

int sum_pair(Pair p) { return p.a + p.b; }
Pair make_pair(int a, int b) { Pair p; p.a = a; p.b = b; return p; }
long long sum_wide(Wide w) { return w.v[0] + w.v[1] + w.v[2]; }
Wide make_wide(long long a) { Wide w; w.v[0] = a; w.v[1] = a; w.v[2] = a; return w; }
Made make_made(int k) { return Made(k); }
int read_made(Made m) { return m.n; }
