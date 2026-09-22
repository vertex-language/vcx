// Private inheritance, with a using-declaration to expose one member.
#include <cstdio>

struct Engine {
    int start() { return 1; }
    int rpm() const { return 900; }
};

class Car : private Engine {
public:
    using Engine::rpm;
    int go() { return start() * 60; }
};

int main() {
    Car c;
    std::printf("%d %d\n", c.go(), c.rpm());
    return 0;
}
