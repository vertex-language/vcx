// Returning from inside nested loops.
#include <cstdio>

int find(const int* v, int n, int want) {
    for (int i = 0; i < n; ++i)
        for (int k = 0; k < 3; ++k)
            if (v[i] + k == want) return i * 10 + k;
    return -1;
}

int main() {
    int v[] = {5, 9, 20, 31};
    std::printf("%d %d\n", find(v, 4, 22), find(v, 4, 100));
    return 0;
}
