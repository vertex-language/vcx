// Do structured bindings unpack structs and arrays into lvalues?
//
// §9.6 [dcl.struct.bind] -- structured bindings for aggregate structs
// and arrays, by value and by reference.

struct Point {
    int x;
    int y;
};

struct Triple {
    int a;
    int b;
    int c;
};

int main() {
    Point pt{10, 20};
    auto [px, py] = pt;

    int arr[3] = {1, 2, 3};
    auto [v0, v1, v2] = arr;

    Triple t{100, 200, 300};
    auto& [ta, tb, tc] = t;
    ta += 5;
    tb += 6;

    auto [w0, w1] = Point{30, 40};

    int result = (px == 10 && py == 20 ? 1 : 0) * 100
               + (v0 == 1 && v1 == 2 && v2 == 3 ? 1 : 0) * 20
               + (t.a == 105 && t.b == 206 && tc == 300 ? 1 : 0) * 10
               + (w0 == 30 && w1 == 40 ? 1 : 0) * 5;

    return result;
}
