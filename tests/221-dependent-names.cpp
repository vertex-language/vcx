// typename and template in dependent names, and default template arguments.
#include <cstdio>

struct Traits {
    using value_type = long;
    template <int N>
    static constexpr int scaled = N * 10;
    template <typename T>
    static T make(int v) { return T(v) * 2; }
};

template <typename T = Traits, int Bias = 5>
typename T::value_type compute(int v) {
    typename T::value_type r = T::template make<typename T::value_type>(v);
    return r + T::template scaled<3> + Bias;
}

int main() {
    std::printf("%ld %ld\n", compute(4), compute<Traits, 0>(1));
    return 0;
}
