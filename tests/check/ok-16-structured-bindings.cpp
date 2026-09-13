// Structured bindings (§9.6 [dcl.struct.bind]).

struct Point {
    int x;
    int y;
};

struct Tuple3 {
    int a;
    double b;
    char c;
};

void test_struct() {
    Point p{10, 20};
    auto [x, y] = p;
    auto& [rx, ry] = p;
    const auto [cx, cy] = p;
    const auto& [crx, cry] = p;
    rx = 30;
}

void test_array() {
    int arr[3] = {1, 2, 3};
    auto [a, b, c] = arr;
    auto& [ra, rb, rc] = arr;
    ra = 10;
}

void test_nested() {
    Tuple3 t{1, 2.0, 'a'};
    auto [a, b, c] = t;
}
