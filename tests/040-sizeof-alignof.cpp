// sizeof and alignof the scalar types.
#include <cstdio>

int main() {
    std::printf("%zu %zu %zu %zu %zu %zu %zu\n", sizeof(char), sizeof(short), sizeof(int),
                sizeof(long long), sizeof(float), sizeof(double), sizeof(void*));
    std::printf("%zu %zu %zu\n", alignof(char), alignof(int), alignof(double));
    return 0;
}
