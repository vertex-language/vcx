// A mutable member can change inside a const member function: a cache.
#include <cstdio>

class Fib {
public:
    long long get(int n) const {
        if (n < 2) return n;
        if (cache_[n]) {
            ++hits_;
            return cache_[n];
        }
        return cache_[n] = get(n - 1) + get(n - 2);
    }
    int hits() const { return hits_; }

private:
    mutable long long cache_[91] = {};
    mutable int hits_ = 0;
};

int main() {
    const Fib f;
    std::printf("%lld %d\n", f.get(90), f.hits());
    return 0;
}
