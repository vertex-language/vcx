// A pure virtual function makes a class abstract.
#include <cstdio>

struct Stream {
    virtual int next() = 0;
    int sum(int n) {
        int s = 0;
        while (n--) s += next();
        return s;
    }
    virtual ~Stream() {}
};

struct Evens : Stream {
    int v = 0;
    int next() override { return v += 2; }
};

int main() {
    Evens e;
    Stream& s = e;
    std::printf("%d\n", s.sum(5));
    return 0;
}
