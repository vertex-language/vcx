// A concept constrains a template.
#include <concepts>
#include <cstdio>

template <typename T>
concept Numeric = std::integral<T> || std::floating_point<T>;

template <Numeric T>
T twice(T v) { return v + v; }

template <typename T>
concept HasSize = requires(T t) { t.size(); };

struct Bag {
    int size() const { return 3; }
};

template <HasSize T>
int measure(const T& t) { return t.size() * 10; }

int main() {
    std::printf("%d %g %d\n", twice(4), twice(1.5), measure(Bag{}));
    return 0;
}
