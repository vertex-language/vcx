// override, final on a function, and final on a class.
#include <cstdio>

struct A {
    virtual const char* name() const { return "A"; }
    virtual int id() const { return 1; }
    virtual ~A() = default;
};

struct B : A {
    const char* name() const override { return "B"; }
    int id() const final { return 2; }
};

struct C final : B {
    const char* name() const override { return "C"; }
};

int main() {
    C c;
    const A& a = c;
    std::printf("%s %d\n", a.name(), a.id());
    return 0;
}
