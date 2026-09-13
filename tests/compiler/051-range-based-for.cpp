// 051-range-based-for.cpp
// Tests range-based for loops over arrays:
// - by value (auto, explicit type)
// - by reference (auto&, auto&&)
// - by const reference (const auto&)
// - braced initializer list ({1, 2, 3})
// - init-statement with continue and break
// - nested ranged loops
// - ranged loop over struct array

struct Point {
    int x;
    int y;
};

int test_by_value() {
    int a[5] = {1, 2, 3, 4, 5};
    int s = 0;
    for (int v : a) {
        s += v;
    }
    for (auto v : a) {
        s += v;
    }
    return s;
}

int test_by_ref() {
    int a[4] = {10, 20, 30, 40};
    for (auto& v : a) {
        v += 1;
    }
    int s = 0;
    for (const auto& v : a) {
        s += v;
    }
    return s;
}

int test_init_list() {
    int a[] = {10, 20, 30};
    int s = 0;
    for (auto v : a) {
        s += v;
    }
    return s;
}

int test_init_stmt() {
    int a[4] = {1, 2, 3, 4};
    int total = 0;
    for (int multiplier = 10; auto v : a) {
        if (v == 2) continue;
        if (v == 4) break;
        total += v * multiplier;
    }
    return total;
}

int test_nested() {
    int a[3] = {1, 2, 3};
    int b[2] = {10, 20};
    int total = 0;
    for (auto x : a) {
        for (auto y : b) {
            total += x * y;
        }
    }
    return total;
}

int test_struct_array() {
    Point pts[3] = {{1, 2}, {3, 4}, {5, 6}};
    int sum = 0;
    for (const auto& p : pts) {
        sum += p.x + p.y;
    }
    return sum;
}

int main() {
    int r1 = test_by_value();
    int r2 = test_by_ref();
    int r3 = test_init_list();
    int r4 = test_init_stmt();
    int r5 = test_nested();
    int r6 = test_struct_array();

    return (r1 + r2 + r3 + r4 + r5 + r6) % 256;
}
