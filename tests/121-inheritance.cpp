// A derived class inherits members and adds its own.
#include <cstdio>

struct Animal {
    int legs = 4;
    int weight() const { return legs * 10; }
};

struct Bird : Animal {
    Bird() { legs = 2; }
    int wings = 2;
};

int main() {
    Bird b;
    std::printf("%d %d %d\n", b.legs, b.wings, b.weight());
    return 0;
}
