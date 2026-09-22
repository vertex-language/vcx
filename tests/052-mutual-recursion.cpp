// Two functions that call each other, through a forward declaration.
#include <cstdio>

bool is_odd(unsigned n);
bool is_even(unsigned n) { return n == 0 ? true : is_odd(n - 1); }
bool is_odd(unsigned n) { return n == 0 ? false : is_even(n - 1); }

int main() {
    std::printf("%d %d %d\n", is_even(10), is_odd(7), is_even(99));
    return 0;
}
