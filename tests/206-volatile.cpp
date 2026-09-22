// volatile accesses happen as written, each one.
#include <cstdio>

volatile int flag = 0;

int spin_reads(volatile const int* p, int n) {
    int sum = 0;
    for (int i = 0; i < n; ++i) sum += *p;
    return sum;
}

int main() {
    volatile int v = 3;
    v = v + 1;
    v += 1;
    flag = v;
    std::printf("%d %d\n", spin_reads(&flag, 10), (int)v);
    return 0;
}
