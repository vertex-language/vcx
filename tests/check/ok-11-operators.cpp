// §12.4 [over.oper] -- an overloaded operator is a function, and the call
// that uses it goes through the same resolution as any other.
struct Vec {
    int x, y;
    Vec operator+(const Vec& o) const { return Vec{x + o.x, y + o.y}; }
    Vec operator-() const { return Vec{-x, -y}; }
    Vec& operator+=(const Vec& o) { x += o.x; y += o.y; return *this; }
    bool operator==(const Vec& o) const { return x == o.x && y == o.y; }
    int& operator[](int i) { return i == 0 ? x : y; }
    int operator()(int s) const { return x * s + y; }
    Vec& operator++() { ++x; ++y; return *this; }
    Vec operator++(int) { Vec c = *this; ++x; ++y; return c; }
    explicit operator bool() const { return x || y; }
};

Vec operator*(const Vec& v, int s) { return Vec{v.x * s, v.y * s}; }
Vec operator*(int s, const Vec& v) { return v * s; }

int use() {
    Vec a{1, 2};
    Vec b{3, 4};
    Vec c = a + b;
    c += a;
    c = -c;
    c = c * 2;
    c = 2 * c;
    ++c;
    c++;
    if (c) { }
    if (c == a) { }
    return c[0] + c(2);
}
