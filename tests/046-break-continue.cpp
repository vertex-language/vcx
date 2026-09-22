// break leaves the innermost loop; continue skips to its next iteration.
#include <cstdio>

int main() {
    int sum = 0;
    for (int i = 0; i < 100; ++i) {
        if (i % 3 == 0) continue;
        if (i > 20) break;
        sum += i;
    }
    std::printf("%d\n", sum);
    return 0;
}
