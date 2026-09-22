// A pointer to a pointer.
#include <cstdio>

void retarget(int** pp, int* to) { *pp = to; }

int main() {
    int a = 1, b = 2;
    int* p = &a;
    retarget(&p, &b);
    **(&p) += 40;
    std::printf("%d %d\n", *p, b);
    return 0;
}
