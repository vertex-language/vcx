// A user-defined copy constructor and copy assignment.
#include <cstdio>

struct Tracked {
    int v;
    Tracked(int x) : v(x) {}
    Tracked(const Tracked& o) : v(o.v + 100) { std::printf("copy %d\n", o.v); }
    Tracked& operator=(const Tracked& o) {
        v = o.v + 1000;
        std::printf("assign %d\n", o.v);
        return *this;
    }
};

int main() {
    Tracked a(1);
    Tracked b = a;
    Tracked c(2);
    c = a;
    std::printf("%d %d %d\n", a.v, b.v, c.v);
    return 0;
}
