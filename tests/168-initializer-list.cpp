// A constructor taking std::initializer_list.
#include <cstdio>
#include <initializer_list>

class Stats {
public:
    Stats(std::initializer_list<int> v) {
        for (int x : v) {
            sum_ += x;
            ++n_;
        }
    }
    int sum() const { return sum_; }
    int count() const { return n_; }

private:
    int sum_ = 0, n_ = 0;
};

int main() {
    Stats s{4, 8, 15, 16, 23, 42};
    std::printf("%d %d\n", s.sum(), s.count());
    return 0;
}
