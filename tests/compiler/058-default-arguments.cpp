// 058-default-arguments.cpp
// Tests default arguments in C++:
// - Free functions with single and multiple default arguments
// - Forward declaration supplying default arguments
// - Member functions with default arguments
// - Constructors with default arguments

int add(int a, int b = 10, int c = 5) {
    return a + b + c;
}

int mul(int a, int b = 2);
int mul(int a, int b) {
    return a * b;
}

struct Point {
    int x;
    int y;
    Point(int x = 1, int y = 2) : x(x), y(y) {}

    int offset(int dx = 10, int dy = 20) {
        return (x + dx) + (y + dy);
    }
};

int main() {
    int r1 = add(100);          // 100 + 10 + 5 = 115
    int r2 = add(100, 20);      // 100 + 20 + 5 = 125
    int r3 = add(100, 20, 30);  // 100 + 20 + 30 = 150
    int r4 = mul(7);            // 7 * 2 = 14
    int r5 = mul(7, 3);         // 7 * 3 = 21

    Point p_def;                // x = 1, y = 2
    Point p_one(10);            // x = 10, y = 2
    Point p_two(10, 20);        // x = 10, y = 20

    int r6 = p_def.x + p_def.y; // 3
    int r7 = p_one.x + p_one.y; // 12
    int r8 = p_two.x + p_two.y; // 30

    int r9 = p_def.offset();    // (1 + 10) + (2 + 20) = 33
    int r10 = p_def.offset(5);  // (1 + 5) + (2 + 20) = 28

    // Total: 115 + 125 + 150 + 14 + 21 + 3 + 12 + 30 + 33 + 28 = 531
    // 531 % 256 = 19
    return (r1 + r2 + r3 + r4 + r5 + r6 + r7 + r8 + r9 + r10) % 256;
}
