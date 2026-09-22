// A class template whose member functions are constrained.
#include <concepts>
#include <cstdio>

template <typename T>
struct Number {
    T v;
    T half() const
        requires std::floating_point<T>
    {
        return v / 2;
    }
    T half() const
        requires std::integral<T>
    {
        return v >> 1;
    }
    bool even() const
        requires std::integral<T>
    {
        return v % 2 == 0;
    }
};

int main() {
    Number<int> i{7};
    Number<double> d{7};
    std::printf("%d %g %d\n", i.half(), d.half(), i.even());
    return 0;
}
