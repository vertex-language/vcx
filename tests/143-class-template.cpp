// A class template instantiated for two types.
#include <cstdio>

template <typename T>
class Pair {
public:
    Pair(T a, T b) : a_(a), b_(b) {}
    T sum() const { return a_ + b_; }
    void swap() {
        T t = a_;
        a_ = b_;
        b_ = t;
    }
    T first() const { return a_; }

private:
    T a_, b_;
};

int main() {
    Pair<int> p(1, 2);
    p.swap();
    Pair<double> q(0.5, 0.25);
    std::printf("%d %d %g\n", p.first(), p.sum(), q.sum());
    return 0;
}
