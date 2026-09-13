// Do overloaded operators compile and execute correctly?
//
// §12.4 [over.match.oper] -- binary, unary, subscript, call, increment,
// compound assignment, and user-defined conversion operators.

struct Vec2 {
    int x, y;

    Vec2 operator+(const Vec2& o) const { return {x + o.x, y + o.y}; }
    Vec2 operator-(const Vec2& o) const { return {x - o.x, y - o.y}; }
    Vec2& operator+=(const Vec2& o) {
        x += o.x;
        y += o.y;
        return *this;
    }
    bool operator==(const Vec2& o) const { return x == o.x && y == o.y; }
    bool operator!=(const Vec2& o) const { return !(*this == o); }
    Vec2 operator-() const { return {-x, -y}; }

    Vec2& operator++() {
        ++x;
        ++y;
        return *this;
    }
    Vec2 operator++(int) {
        Vec2 old = *this;
        ++x;
        ++y;
        return old;
    }

    int operator[](int idx) const {
        return idx == 0 ? x : y;
    }

    int operator()(int scale) const {
        return (x + y) * scale;
    }
};

int main() {
    Vec2 a{10, 20};
    Vec2 b{3, 4};

    Vec2 c = a + b;
    Vec2 d = a - b;
    c += d;

    Vec2 neg = -b;

    Vec2 post = b++;
    Vec2 pre = ++b;

    int x_val = c[0];
    int y_val = c[1];

    int call_val = c(2);

    bool eq = (a == a);
    bool neq = (a != b);

    int result = (c.x == 20 && c.y == 40 ? 1 : 0) * 100
               + (post.x == 3 && post.y == 4 ? 1 : 0) * 20
               + (pre.x == 5 && pre.y == 6 ? 1 : 0) * 10
               + (neg.x == -3 && neg.y == -4 ? 1 : 0) * 5
               + (x_val == 20 && y_val == 40 ? 1 : 0) * 2
               + (call_val == 120 ? 1 : 0)
               + (eq && neq ? 1 : 0);

    return result;
}
