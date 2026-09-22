// A for loop with a declaration, a condition and a step.
#include <cstdio>

int main() {
    long long sum = 0;
    for (int i = 1; i <= 100; ++i) sum += i * i;
    std::printf("%lld\n", sum);
    return 0;
}
