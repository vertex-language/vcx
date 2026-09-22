// dynamic_cast to a derived class succeeds only for the right dynamic type.
#include <cstdio>

struct Base {
    virtual ~Base() = default;
};
struct A : Base {
    int a = 1;
};
struct B : Base {
    int b = 2;
};

int probe(Base* p) {
    if (A* a = dynamic_cast<A*>(p)) return a->a;
    if (B* b = dynamic_cast<B*>(p)) return b->b * 10;
    return -1;
}

int main() {
    A a;
    B b;
    Base base;
    std::printf("%d %d %d\n", probe(&a), probe(&b), probe(&base));
    return 0;
}
