// A constructor that delegates to another.
#include <cstdio>

class Date {
public:
    Date(int y, int m, int d) : y_(y), m_(m), d_(d) { std::printf("full\n"); }
    Date(int y) : Date(y, 1, 1) { std::printf("year only\n"); }
    int code() const { return y_ * 10000 + m_ * 100 + d_; }

private:
    int y_, m_, d_;
};

int main() {
    Date d(2026);
    std::printf("%d\n", d.code());
    return 0;
}
