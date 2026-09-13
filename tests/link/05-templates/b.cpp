template <typename T> T twice(T x) { return x + x; }

template <typename T> struct Box {
    T v;
    Box(T x) : v(x) {}
    T get() const { return v; }
    void set(T x) { v = x; }
};

int use_box(Box<int> &b) {
    int was = b.get();
    b.set(was + 1);
    return was;
}

double twice_double(double x) { return twice(x); }
int twice_int_from_b(int x) { return twice(x); }
