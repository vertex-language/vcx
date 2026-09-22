// Every compound assignment operator.
#include <cstdio>

int main() {
    int v = 100;
    v += 5;  std::printf("%d ", v);
    v -= 3;  std::printf("%d ", v);
    v *= 2;  std::printf("%d ", v);
    v /= 7;  std::printf("%d ", v);
    v %= 5;  std::printf("%d ", v);
    v <<= 4; std::printf("%d ", v);
    v >>= 1; std::printf("%d ", v);
    v |= 3;  std::printf("%d ", v);
    v &= 6;  std::printf("%d ", v);
    v ^= 15; std::printf("%d\n", v);
    return 0;
}
