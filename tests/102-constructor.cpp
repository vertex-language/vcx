// Constructors: default, parameterized, and a member initializer list.
#include <cstdio>

class Rect {
public:
    Rect() : w_(1), h_(1) {}
    Rect(int w, int h) : w_(w), h_(h) {}
    int area() const { return w_ * h_; }

private:
    int w_, h_;
};

int main() {
    Rect a;
    Rect b(3, 4);
    Rect c{5, 6};
    std::printf("%d %d %d\n", a.area(), b.area(), c.area());
    return 0;
}
