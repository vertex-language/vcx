// A virtual function dispatches on the dynamic type.
#include <cstdio>

struct Shape {
    virtual double area() const { return 0; }
    virtual ~Shape() = default;
};

struct Square : Shape {
    double s;
    Square(double x) : s(x) {}
    double area() const override { return s * s; }
};

struct Circle : Shape {
    double r;
    Circle(double x) : r(x) {}
    double area() const override { return 3.0 * r * r; }
};

double total(const Shape* const* shapes, int n) {
    double t = 0;
    for (int i = 0; i < n; ++i) t += shapes[i]->area();
    return t;
}

int main() {
    Square a(2);
    Circle b(1);
    Shape c;
    const Shape* all[] = {&a, &b, &c};
    std::printf("%g\n", total(all, 3));
    return 0;
}
