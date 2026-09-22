// Character and escape-sequence literals.
#include <cstdio>

int main() {
    std::printf("%d %d %d %d %d %d\n", 'A', '\n', '\t', '\\', '\'', '\x7f');
    std::printf("%d %d\n", '\0', '\101');
    return 0;
}
