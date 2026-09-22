// Copying a derived object into a base object slices it.
#include <cstdio>

struct Base {
    int b = 1;
    virtual int get() const { return b; }
    virtual ~Base() = default;
};

struct Derived : Base {
    int d = 2;
    int get() const override { return b + d * 10; }
};

int main() {
    Derived d;
    Base sliced = d;
    const Base& ref = d;
    std::printf("%d %d\n", sliced.get(), ref.get());
    return 0;
}
