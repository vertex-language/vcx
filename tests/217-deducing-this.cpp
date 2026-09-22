// Deducing this: one member function for const and non-const, lvalue and rvalue.
#include <cstdio>
#include <utility>

struct Box {
    int v = 1;
    template <typename Self>
    auto&& value(this Self&& self) {
        return std::forward<Self>(self).v;
    }
    const char* kind(this const Box&) { return "const&"; }
    const char* kind(this Box&&) { return "&&"; }
};

int main() {
    Box b;
    b.value() = 10;
    const Box& cb = b;
    std::printf("%d %s %s\n", cb.value(), cb.kind(), Box{}.kind());
    return 0;
}
