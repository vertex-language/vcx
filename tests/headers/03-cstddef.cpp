// Does <cstddef> compile, with size_t, ptrdiff_t and nullptr_t the
// target's?

#include <cstddef>

struct Pair { char a; int b; };

int main() {
    std::size_t n = sizeof(Pair);            // 8
    std::ptrdiff_t d = 10 - 3;               // 7
    std::nullptr_t none = nullptr;
    int *p = none;
    return (int)n + (int)d + (p == nullptr ? 1 : 0) + (int)offsetof(Pair, b) - 20;   // 8 + 7 + 1 + 4 - 20 = 0
}
