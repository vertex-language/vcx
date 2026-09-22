// Output parameters through pointers.
#include <cstdio>

void divmod(int a, int b, int* q, int* r) {
    *q = a / b;
    *r = a % b;
}

int main() {
    int q, r;
    divmod(47, 5, &q, &r);
    std::printf("%d %d\n", q, r);
    return 0;
}
