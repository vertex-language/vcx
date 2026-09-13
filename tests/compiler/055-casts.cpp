// 055-casts.cpp
// Tests static_cast and C-style casts:
// - float to int and int to float
// - narrowing and widening integer conversions
// - signed/unsigned conversions
// - enum class to underlying integer
// - derived to base pointer cast
// - C-style cast syntax

enum class Priority {
    Low = 10,
    Medium = 20,
    High = 30
};

struct Base {
    int b;
};

struct Derived : Base {
    int d;
};

int test_numeric_casts() {
    double pi = 3.14159;
    int int_pi = static_cast<int>(pi); // 3

    int x = 42;
    double d_x = static_cast<double>(x); // 42.0

    long long big = 0x12345678LL;
    short small = static_cast<short>(big); // 0x5678 = 22136

    unsigned int u = static_cast<unsigned int>(-1);
    int s = static_cast<int>(u); // -1

    return int_pi + (int)d_x + (small & 0xFF) + (s == -1 ? 1 : 0); // 3 + 42 + 120 + 1 = 166
}

int test_enum_cast() {
    Priority p = Priority::High;
    int val = static_cast<int>(p); // 30
    return val;
}

int test_pointer_cast() {
    Derived d;
    d.b = 77;
    d.d = 88;

    Base* bp = static_cast<Base*>(&d);
    return bp->b; // 77
}

int test_c_style_casts() {
    float f = 5.9f;
    int a = (int)f;      // 5
    short b = (short)1000; // 1000
    return a + b;        // 1005
}

int main() {
    int r1 = test_numeric_casts(); // 166
    int r2 = test_enum_cast();     // 30
    int r3 = test_pointer_cast();  // 77
    int r4 = test_c_style_casts(); // 1005

    // Total: 166 + 30 + 77 + 1005 = 1278
    // 1278 % 256 = 254
    return (r1 + r2 + r3 + r4) % 256;
}
