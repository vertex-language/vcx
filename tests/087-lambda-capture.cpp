// Captures by value and by reference.
#include <cstdio>

int main() {
    int base = 10, total = 0;
    auto add_base = [base](int v) { return v + base; };
    auto accumulate = [&total](int v) { total += v; };
    base = 1000;
    for (int i = 0; i < 4; ++i) accumulate(add_base(i));
    std::printf("%d\n", total);
    return 0;
}
