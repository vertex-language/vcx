// consteval functions run only at compile time; constinit fixes a global's initializer.
#include <cstdio>

consteval int digits(long long v) {
    int n = 1;
    while (v >= 10) {
        v /= 10;
        ++n;
    }
    return n;
}

constinit int width = digits(123456789);

int main() {
    std::printf("%d %d\n", width, digits(7));
    return 0;
}
