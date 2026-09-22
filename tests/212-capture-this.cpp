// Capturing this by reference, and *this by copy.
#include <cstdio>

struct Counter {
    int n = 0;
    auto by_ref() {
        return [this] { return ++n; };
    }
    auto by_copy() {
        return [*this]() mutable { return ++n; };
    }
};

int main() {
    Counter c;
    auto r = c.by_ref();
    r();
    r();
    auto k = c.by_copy();
    k();
    k();
    k();
    std::printf("%d %d\n", c.n, k());
    return 0;
}
