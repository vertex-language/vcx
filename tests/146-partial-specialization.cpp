// Partial specialization for pointers and for arrays.
#include <cstdio>

template <typename T>
struct Kind {
    static constexpr int v = 0;
};
template <typename T>
struct Kind<T*> {
    static constexpr int v = 1 + Kind<T>::v;
};
template <typename T, int N>
struct Kind<T[N]> {
    static constexpr int v = 100 + N;
};

int main() {
    std::printf("%d %d %d %d\n", Kind<int>::v, Kind<int*>::v, Kind<int***>::v, Kind<char[7]>::v);
    return 0;
}
