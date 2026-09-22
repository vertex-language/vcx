// Pointer to const, const pointer, and both.
#include <cstdio>

int main() {
    int a = 1, b = 2;
    const int* p = &a;
    p = &b;
    int* const q = &a;
    *q = 5;
    const int* const r = &b;
    std::printf("%d %d %d\n", *p, a, *r);
    return 0;
}
