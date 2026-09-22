// A table of function pointers, dispatched by index.
#include <cstdio>

int neg(int v) { return -v; }
int dbl(int v) { return v * 2; }
int sq(int v) { return v * v; }

using Fn = int (*)(int);

int main() {
    Fn table[] = {neg, dbl, sq};
    int v = 3;
    for (int i = 0; i < 6; ++i) v = table[i % 3](v);
    std::printf("%d\n", v);
    return 0;
}
