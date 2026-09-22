// Default arguments fill in trailing parameters.
#include <cstdio>

int volume(int w, int h = 2, int d = 3) { return w * h * d; }

int main() {
    std::printf("%d %d %d\n", volume(1), volume(1, 5), volume(1, 5, 7));
    return 0;
}
