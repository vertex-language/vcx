// Init-captures: a new variable in the closure, and a move into it.
#include <cstdio>
#include <memory>

int main() {
    int x = 10;
    auto f = [y = x * 2, z = 3]() { return y + z; };
    auto p = std::make_unique<int>(42);
    auto g = [q = std::move(p)]() { return *q + 1; };
    int n = 0;
    auto h = [&counter = n](int k) { counter += k; };
    h(4);
    h(5);
    std::printf("%d %d %d %d\n", f(), g(), p == nullptr, n);
    return 0;
}
