// §11.4.5 -- a constructor runs when the object is created, and the
// mem-initializer-list is how its members get their first values.
struct Point {
    int x;
    int y;
    Point(int a, int b) : x(a), y(b) {}
    int sum() { return x + y; }
};

struct Counter {
    int n;
    Counter() : n(10) {}
    void bump() { n = n + 1; }
};

int main() {
    Point p(3, 4);
    Counter c;
    c.bump();
    c.bump();
    return p.sum() + c.n;
}
