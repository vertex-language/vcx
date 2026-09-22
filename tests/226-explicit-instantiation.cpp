// Explicit instantiation, and a friend declared as a template.
#include <cstdio>

template <typename T>
class Pair;

template <typename T>
T sum(const Pair<T>& p);

template <typename T>
class Pair {
public:
    Pair(T a, T b) : a_(a), b_(b) {}
    friend T sum<>(const Pair<T>& p);

private:
    T a_, b_;
};

template <typename T>
T sum(const Pair<T>& p) { return p.a_ + p.b_; }

template class Pair<int>;
template int sum<int>(const Pair<int>&);

int main() {
    std::printf("%d %g\n", sum(Pair<int>(2, 3)), sum(Pair<double>(0.5, 0.25)));
    return 0;
}
