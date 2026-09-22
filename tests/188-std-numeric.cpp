// <numeric>: accumulate, iota, inner_product, gcd and lcm.
#include <cstdio>
#include <numeric>
#include <vector>

int main() {
    std::vector<int> v(10);
    std::iota(v.begin(), v.end(), 1);
    int sum = std::accumulate(v.begin(), v.end(), 0);
    long long prod = std::accumulate(v.begin(), v.end(), 1LL, [](long long a, int b) { return a * b; });
    int dot = std::inner_product(v.begin(), v.end(), v.begin(), 0);
    std::printf("%d %lld %d %d %d\n", sum, prod, dot, std::gcd(84, 36), std::lcm(4, 6));
    return 0;
}
