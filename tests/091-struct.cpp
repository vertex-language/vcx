// A struct: members, aggregate initialization, member access.
#include <cstdio>

struct Point {
    int x, y;
};

int main() {
    Point p = {3, 4};
    Point q{.x = -1, .y = 2};
    p.x += q.x;
    std::printf("%d %d %zu\n", p.x, p.y + q.y, sizeof(Point));
    return 0;
}
