// A derived member hides every base overload of the name, unless brought in.
#include <cstdio>

struct Base {
    const char* f(int) { return "base int"; }
    const char* f(double) { return "base double"; }
};

struct Hides : Base {
    const char* f(const char*) { return "hides string"; }
};

struct Unhides : Base {
    using Base::f;
    const char* f(const char*) { return "unhides string"; }
};

int main() {
    Hides h;
    Unhides u;
    std::printf("%s | %s %s\n", h.f("x"), u.f(1), u.f(2.0));
    return 0;
}
