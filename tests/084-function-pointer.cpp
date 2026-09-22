// Calling through a function pointer.
#include <cstdio>

int add(int a, int b) { return a + b; }
int mul(int a, int b) { return a * b; }

int apply(int (*f)(int, int), int a, int b) { return f(a, b); }

int main() {
    int (*op)(int, int) = add;
    std::printf("%d ", op(3, 4));
    op = &mul;
    std::printf("%d %d\n", (*op)(3, 4), apply(add, 10, 20));
    return 0;
}
