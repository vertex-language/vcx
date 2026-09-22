// The more constrained overload wins when one concept subsumes another.
#include <cstdio>

template <typename T>
concept Animal = requires(T t) { t.legs(); };

template <typename T>
concept Bird = Animal<T> && requires(T t) { t.wings(); };

struct Dog {
    int legs() const { return 4; }
};
struct Crow {
    int legs() const { return 2; }
    int wings() const { return 2; }
};

const char* kind(const Animal auto&) { return "animal"; }
const char* kind(const Bird auto&) { return "bird"; }

int main() {
    std::printf("%s %s\n", kind(Dog{}), kind(Crow{}));
    return 0;
}
