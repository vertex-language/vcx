// A mutable lambda changes its own copy of a capture.
#include <cstdio>

int main() {
    int n = 0;
    auto next = [n]() mutable { return ++n; };
    next();
    next();
    std::printf("%d %d\n", next(), n);
    return 0;
}
