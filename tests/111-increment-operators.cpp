// Prefix and postfix ++ as member operators.
#include <cstdio>

struct Counter {
    int v = 0;
    Counter& operator++() {
        ++v;
        return *this;
    }
    Counter operator++(int) {
        Counter old = *this;
        ++v;
        return old;
    }
};

int main() {
    Counter c;
    Counter a = c++;
    Counter b = ++c;
    std::printf("%d %d %d\n", a.v, b.v, c.v);
    return 0;
}
