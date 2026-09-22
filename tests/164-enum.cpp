// Unscoped enums convert to int; scoped enums do not.
#include <cstdio>

enum Color { Red, Green = 5, Blue };
enum class Mode : unsigned char { Off, On = 200 };

const char* name(Mode m) { return m == Mode::On ? "on" : "off"; }

int main() {
    int c = Blue;
    Mode m = Mode::On;
    std::printf("%d %d %d %s %zu\n", Red, c, static_cast<int>(m), name(m), sizeof(Mode));
    return 0;
}
