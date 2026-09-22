// bool converts from any scalar, and to 0 or 1.
#include <cstdio>

int main() {
    bool a = 42;
    bool b = 0.0;
    bool c(nullptr);  // direct-initialization: copy-initialization from nullptr is ill-formed
    int n = a + a;
    std::printf("%d %d %d %d %zu\n", a, b, c, n, sizeof(bool));
    return 0;
}
