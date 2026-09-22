// Two bases, and the pointer adjustment converting to the second.
#include <cstdio>

struct Named {
    const char* name = "named";
    virtual const char* label() const { return name; }
    virtual ~Named() = default;
};

struct Counted {
    int count = 3;
    virtual int total() const { return count; }
    virtual ~Counted() = default;
};

struct Both : Named, Counted {
    int total() const override { return count * 100; }
    const char* label() const override { return "both"; }
};

int main() {
    Both b;
    Counted* c = &b;
    Named* n = &b;
    std::printf("%d %s %d\n", c->total(), n->label(), (void*)c != (void*)n);
    return 0;
}
