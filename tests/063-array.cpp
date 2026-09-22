// A local array: initialized, indexed, summed.
#include <cstdio>

int main() {
    int v[6] = {3, 1, 4, 1, 5};
    int sum = 0;
    for (int i = 0; i < 6; ++i) sum += v[i] * (i + 1);
    std::printf("%d %d %zu\n", sum, v[5], sizeof v / sizeof v[0]);
    return 0;
}
