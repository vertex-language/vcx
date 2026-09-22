// Default member initializers, and constructors that override them.
#include <cstdio>

struct Config {
    int width = 80;
    int height = 24;
    bool color = true;
    Config() = default;
    Config(int w) : width(w) {}
};

int main() {
    Config a, b(120);
    std::printf("%d %d %d %d %d\n", a.width, a.height, a.color, b.width, b.height);
    return 0;
}
