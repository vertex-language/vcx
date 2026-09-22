// Mixing signed and unsigned converts to unsigned.
#include <cstdio>

int main() {
    int neg = -1;
    unsigned one = 1u;
    std::printf("%d\n", neg < one);          // -1 becomes UINT_MAX
    std::printf("%d\n", neg < (int)one);
    long long wide = neg;
    std::printf("%lld\n", wide + one);       // long long can hold every unsigned
    return 0;
}
