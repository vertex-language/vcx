// A struct too large for registers, passed and returned by value.
#include <cstdio>

struct Big {
    long long v[8];
};

Big fill(long long start) {
    Big b;
    for (int i = 0; i < 8; ++i) b.v[i] = start + i;
    return b;
}

long long sum(Big b) {
    long long s = 0;
    for (long long x : b.v) s += x;
    return s;
}

int main() {
    std::printf("%lld\n", sum(fill(100)));
    return 0;
}
