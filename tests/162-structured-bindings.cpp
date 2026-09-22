// Structured bindings over a struct, an array, and a reference.
#include <cstdio>

struct Result {
    int quotient;
    int remainder;
};

Result divide(int a, int b) { return {a / b, a % b}; }

int main() {
    auto [q, r] = divide(47, 5);
    int arr[3] = {7, 8, 9};
    auto& [x, y, z] = arr;
    y = 80;
    std::printf("%d %d %d %d %d\n", q, r, x, arr[1], z);
    return 0;
}
