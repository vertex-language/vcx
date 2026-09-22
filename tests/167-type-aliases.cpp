// typedef and using, including an alias template.
#include <cstdio>

typedef unsigned long long u64;
using Callback = int (*)(int);
template <typename T>
using Pair = T[2];

int inc(int v) { return v + 1; }

int main() {
    u64 big = 1ull << 40;
    Callback cb = inc;
    Pair<int> p = {3, 4};
    std::printf("%llu %d %d\n", big, cb(41), p[0] * p[1]);
    return 0;
}
