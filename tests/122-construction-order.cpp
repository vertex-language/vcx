// Bases are constructed before members, members before the body; destruction reverses it.
#include <cstdio>

struct Part {
    const char* n;
    Part(const char* s) : n(s) { std::printf("+%s ", n); }
    ~Part() { std::printf("-%s ", n); }
};

struct Base {
    Part p{"base-member"};
    Base() { std::printf("+base "); }
    ~Base() { std::printf("-base "); }
};

struct Derived : Base {
    Part a{"a"};
    Part b{"b"};
    Derived() { std::printf("+derived\n"); }
    ~Derived() { std::printf("-derived "); }
};

int main() {
    {
        Derived d;
    }
    std::printf("\n");
    return 0;
}
