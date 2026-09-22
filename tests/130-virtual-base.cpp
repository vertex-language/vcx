// A virtual base is shared by the two paths to it.
#include <cstdio>

struct Root {
    int v = 0;
    Root() { std::printf("root\n"); }
};

struct Left : virtual Root {
    void inc() { ++v; }
};

struct Right : virtual Root {
    void add(int n) { v += n; }
};

struct Diamond : Left, Right {};

int main() {
    Diamond d;
    d.inc();
    d.add(10);
    Left& l = d;
    Right& r = d;
    std::printf("%d %d %d\n", d.v, l.v, r.v);
    return 0;
}
