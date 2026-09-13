// 060-type-aliases.cpp
// Tests type aliases in C++:
// - typedef for primitive types, pointers, arrays
// - using type aliases
// - nested type aliases inside classes (using / typedef)
// - alias templates: template<typename T> using
// - using aliases in functions, templates, and casts

typedef unsigned char byte_t;
typedef int (*BinaryFn)(int, int);
typedef int IntArray4[4];

using Index = long long;
template<typename T> using Ptr = T*;
template<typename T> using Ref = T&;

int add(int a, int b) {
    return a + b;
}

struct Point {
    using coord_t = int;
    typedef coord_t value_type;

    coord_t x;
    coord_t y;

    coord_t sum() {
        return x + y;
    }
};

template<typename T>
struct Box {
    using value_type = T;
    using pointer = Ptr<T>;

    value_type data;

    pointer get_address() {
        return &data;
    }
};

int test_typedefs() {
    byte_t b = 250;
    b = b + 5; // 255

    IntArray4 arr;
    arr[0] = 10;
    arr[1] = 20;
    arr[2] = 30;
    arr[3] = 40;

    BinaryFn fn = add;
    int sum = fn(arr[0], arr[3]); // 10 + 40 = 50

    return (int)b + sum; // 255 + 50 = 305
}

int test_using_and_templates() {
    Index idx = 1000;
    int target = 42;
    Ptr<int> p = &target;
    *p = 55;

    Point pt;
    pt.x = 12;
    pt.y = 18;
    Point::coord_t s = pt.sum(); // 30

    Box<int> box;
    box.data = 77;
    Box<int>::pointer bptr = box.get_address();

    return (int)idx + target + s + *bptr; // 1000 + 55 + 30 + 77 = 1162
}

int main() {
    int r1 = test_typedefs();           // 305
    int r2 = test_using_and_templates(); // 1162

    // Total: 305 + 1162 = 1467
    // 1467 % 256 = 187
    return (r1 + r2) % 256;
}
