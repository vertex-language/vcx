// Do non-member overloaded operators compile and execute correctly?
//
// §12.4 [over.match.oper] -- non-member binary and unary operators,
// commutative operations (scalar + class, class + scalar),
// chained stream-like operators (<<, >>), comparison operators,
// and prefix/postfix operators.

struct Vec2 {
    int x;
    int y;
};

// Non-member binary operators
Vec2 operator+(const Vec2& a, const Vec2& b) {
    return Vec2{a.x + b.x, a.y + b.y};
}

Vec2 operator-(const Vec2& a, const Vec2& b) {
    return Vec2{a.x - b.x, a.y - b.y};
}

// Commutative scalar multiplication
Vec2 operator*(const Vec2& v, int s) {
    return Vec2{v.x * s, v.y * s};
}

Vec2 operator*(int s, const Vec2& v) {
    return Vec2{v.x * s, v.y * s};
}

// Non-member unary operator
Vec2 operator-(const Vec2& v) {
    return Vec2{-v.x, -v.y};
}

// Non-member comparisons
bool operator==(const Vec2& a, const Vec2& b) {
    return a.x == b.x && a.y == b.y;
}

bool operator!=(const Vec2& a, const Vec2& b) {
    return !(a == b);
}

// Chained stream-like insertion
struct Buffer {
    int data[8];
    int count;
};

Buffer& operator<<(Buffer& b, int val) {
    if (b.count < 8) {
        b.data[b.count] = val;
        b.count = b.count + 1;
    }
    return b;
}

Buffer& operator<<(Buffer& b, const Vec2& v) {
    b << v.x << v.y;
    return b;
}

int main() {
    Vec2 a{10, 20};
    Vec2 b{3, 5};

    Vec2 sum = a + b;            // (13, 25)
    Vec2 diff = a - b;           // (7, 15)
    Vec2 s1 = a * 2;             // (20, 40)
    Vec2 s2 = 3 * b;             // (9, 15)
    Vec2 neg = -a;               // (-10, -20)

    bool eq1 = (sum == Vec2{13, 25});
    bool eq2 = (diff == Vec2{7, 15});
    bool eq3 = (s1 == Vec2{20, 40});
    bool eq4 = (s2 == Vec2{9, 15});
    bool eq5 = (neg == Vec2{-10, -20});
    bool neq = (a != b);

    Buffer buf{{0, 0, 0, 0, 0, 0, 0, 0}, 0};
    buf << 42 << a; // puts 42, 10, 20 into buf

    int r = 0;
    if (eq1 && eq2 && eq3 && eq4 && eq5 && neq) {
        r += 10;
    }
    if (buf.count == 3 && buf.data[0] == 42 && buf.data[1] == 10 && buf.data[2] == 20) {
        r += 32;
    }
    return r; // Expected: 42
}
