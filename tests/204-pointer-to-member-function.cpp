// Pointers to member functions, including virtual ones, called with .* and ->*.
#include <cstdio>

struct Base {
    int k = 2;
    int twice(int v) const { return v * k; }
    virtual int op(int v) const { return v + k; }
    virtual ~Base() = default;
};

struct Derived : Base {
    int op(int v) const override { return v * 100; }
};

int main() {
    int (Base::*plain)(int) const = &Base::twice;
    int (Base::*virt)(int) const = &Base::op;
    Base b;
    Derived d;
    const Base* pd = &d;
    std::printf("%d %d %d %d\n", (b.*plain)(5), (b.*virt)(5), (pd->*virt)(5), (d.*plain)(7));
    return 0;
}
