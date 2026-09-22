// A lambda that calls itself, through an explicit object parameter.
#include <cstdio>

int main() {
    auto fib = [](this auto const& self, int n) -> long long {
        return n < 2 ? n : self(n - 1) + self(n - 2);
    };
    auto gcd = [](this auto self, int a, int b) -> int { return b == 0 ? a : self(b, a % b); };
    std::printf("%lld %d\n", fib(30), gcd(1071, 462));
    return 0;
}
