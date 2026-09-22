// A covariant return type narrows an override's result.
#include <cstdio>

struct Animal {
    virtual Animal* clone() const { return new Animal(*this); }
    virtual const char* kind() const { return "animal"; }
    virtual ~Animal() = default;
};

struct Cat : Animal {
    Cat* clone() const override { return new Cat(*this); }
    const char* kind() const override { return "cat"; }
    int lives = 9;
};

int main() {
    Cat c;
    Cat* copy = c.clone();
    Animal* a = &c;
    Animal* other = a->clone();
    std::printf("%d %s\n", copy->lives, other->kind());
    delete copy;
    delete other;
    return 0;
}
