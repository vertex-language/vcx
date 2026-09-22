// A non-type template parameter: a fixed-size array class.
#include <cstdio>

template <typename T, int N>
struct Array {
    T data[N];
    constexpr int size() const { return N; }
    T sum() const {
        T s{};
        for (int i = 0; i < N; ++i) s += data[i];
        return s;
    }
};

int main() {
    Array<int, 4> a{{1, 2, 3, 4}};
    Array<double, 2> b{{0.5, 1.5}};
    std::printf("%d %d %g\n", a.size(), a.sum(), b.sum());
    return 0;
}
