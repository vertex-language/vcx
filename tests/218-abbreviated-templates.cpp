// Abbreviated function templates: auto parameters, constrained and not.
#include <concepts>
#include <cstdio>

auto add(auto a, auto b) { return a + b; }
void show(std::integral auto v) { std::printf("integral %lld\n", (long long)v); }
void show(std::floating_point auto v) { std::printf("floating %g\n", (double)v); }

int main() {
    std::printf("%d %g\n", add(2, 3), add(1.5, 2));
    show(7);
    show('a');
    show(2.5f);
    return 0;
}
