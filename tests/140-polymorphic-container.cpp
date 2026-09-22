// Owning a heterogeneous array of objects through base pointers.
#include <cstdio>

struct Op {
    virtual int apply(int v) const = 0;
    virtual ~Op() { ++destroyed; }
    static int destroyed;
};
int Op::destroyed = 0;

struct Add : Op {
    int n;
    Add(int x) : n(x) {}
    int apply(int v) const override { return v + n; }
};

struct Mul : Op {
    int n;
    Mul(int x) : n(x) {}
    int apply(int v) const override { return v * n; }
};

int main() {
    Op* ops[] = {new Add(3), new Mul(4), new Add(-2), new Mul(5)};
    int v = 1;
    for (Op* o : ops) v = o->apply(v);
    for (Op* o : ops) delete o;
    std::printf("%d %d\n", v, Op::destroyed);
    return 0;
}
