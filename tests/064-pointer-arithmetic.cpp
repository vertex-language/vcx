// Pointer arithmetic: add, subtract, compare, difference.
#include <cstdio>

int main() {
    int v[10];
    for (int i = 0; i < 10; ++i) v[i] = i * i;
    int* p = v + 3;
    int* q = &v[8];
    std::printf("%d %d %td %d\n", *p, *(q - 1), q - p, p < q);
    return 0;
}
