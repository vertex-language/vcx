// A template template parameter.
#include <cstdio>

template <typename T>
struct Box {
    T v;
    T get() const { return v; }
};

template <template <typename> class Holder, typename T>
T unwrap(const Holder<T>& h) { return h.get(); }

int main() {
    Box<int> b{41};
    std::printf("%d\n", unwrap(b) + 1);
    return 0;
}
