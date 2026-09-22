// typeid compares dynamic types.
#include <cstdio>
#include <typeinfo>

struct Base {
    virtual ~Base() = default;
};
struct Derived : Base {};

int main() {
    Derived d;
    Base& b = d;
    Base plain;
    std::printf("%d %d %d\n", typeid(b) == typeid(Derived), typeid(plain) == typeid(Derived),
                typeid(int) == typeid(int));
    return 0;
}
