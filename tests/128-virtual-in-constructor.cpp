// A virtual call in a constructor dispatches to the class under construction.
#include <cstdio>

struct Base {
    Base() { std::printf("%s\n", who()); }
    virtual const char* who() const { return "base"; }
    virtual ~Base() { std::printf("%s\n", who()); }
};

struct Derived : Base {
    Derived() { std::printf("%s\n", who()); }
    const char* who() const override { return "derived"; }
};

int main() {
    Derived d;
    return 0;
}
