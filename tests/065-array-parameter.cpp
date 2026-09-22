// An array passed to a function decays to a pointer.
#include <cstdio>

int sum(const int* v, int n) {
    int s = 0;
    while (n--) s += *v++;
    return s;
}

int main() {
    int v[] = {10, 20, 30, 40};
    std::printf("%d %d\n", sum(v, 4), sum(v + 2, 2));
    return 0;
}
