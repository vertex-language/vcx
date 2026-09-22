// A reference to an array keeps its size.
#include <cstdio>

template <int N>
int count(const int (&v)[N]) { return N; }

int main() {
    int a[7] = {};
    int (&r)[7] = a;
    r[6] = 9;
    std::printf("%d %d\n", count(a), a[6]);
    return 0;
}
