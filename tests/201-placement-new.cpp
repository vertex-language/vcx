// Placement new constructs into storage the program owns; the destructor is called by hand.
#include <cstdio>
#include <new>

struct Point {
    int x, y;
    Point(int a, int b) : x(a), y(b) { std::printf("construct %d %d\n", x, y); }
    ~Point() { std::printf("destroy %d %d\n", x, y); }
};

int main() {
    alignas(Point) unsigned char buf[2 * sizeof(Point)];
    Point* a = new (buf) Point(1, 2);
    Point* b = new (buf + sizeof(Point)) Point(3, 4);
    std::printf("%d\n", a->x + b->y);
    b->~Point();
    a->~Point();
    return 0;
}
