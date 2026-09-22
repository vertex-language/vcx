// An insertion sort over an array: loops, indexing, swaps.
#include <cstdio>

void sort(int* v, int n) {
    for (int i = 1; i < n; ++i) {
        int x = v[i], j = i - 1;
        while (j >= 0 && v[j] > x) {
            v[j + 1] = v[j];
            --j;
        }
        v[j + 1] = x;
    }
}

int main() {
    int v[] = {9, -3, 7, 0, 7, 12, -8, 1};
    sort(v, 8);
    for (int x : v) std::printf("%d ", x);
    std::printf("\n");
    return 0;
}
