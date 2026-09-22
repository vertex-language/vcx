// Padding and alignment inside a struct.
#include <cstddef>
#include <cstdio>

struct Mixed {
    char c;
    int i;
    short s;
    double d;
    char tail;
};

int main() {
    std::printf("%zu %zu %zu %zu %zu %zu\n", offsetof(Mixed, c), offsetof(Mixed, i), offsetof(Mixed, s),
                offsetof(Mixed, d), offsetof(Mixed, tail), sizeof(Mixed));
    return 0;
}
