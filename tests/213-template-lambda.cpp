// A lambda with a template parameter list.
#include <cstdio>

int main() {
    auto size_of = []<typename T>(T) { return sizeof(T); };
    auto first = []<typename T, int N>(T (&arr)[N]) { return arr[0] * N; };
    int v[4] = {3, 1, 2, 5};
    std::printf("%zu %zu %d\n", size_of('c'), size_of(1.0), first(v));
    return 0;
}
