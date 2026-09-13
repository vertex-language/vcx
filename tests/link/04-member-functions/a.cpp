// Do the two compilers agree on how a member function is called?
//
// `this` first, then the arguments; a const member; a static member with
// no `this` at all; a constructor called from the other side on storage
// this side owns; and a member function returning a class with a
// constructor, whose hidden result pointer comes *after* `this` under the
// Microsoft convention.

struct Made {
    int n;
    Made(int k) : n(k) {}
};

struct Counter {
    int count;
    int step;
    Counter(int start, int step);
    void bump();
    int get() const;
    static int twice(int v);
    Made snapshot() const;
};

int main() {
    Counter c(10, 5);
    c.bump();
    c.bump();
    Made m = c.snapshot();
    return c.get() + Counter::twice(3) + m.n - 40;   // 20 + 6 + 20 - 40 = 6
}
