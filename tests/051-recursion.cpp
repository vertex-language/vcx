// A recursive function.
#include <cstdio>

long long fact(int n) { return n <= 1 ? 1 : n * fact(n - 1); }
int fib(int n) { return n < 2 ? n : fib(n - 1) + fib(n - 2); }

int main() {
    std::printf("%lld %d\n", fact(20), fib(25));
    return 0;
}
