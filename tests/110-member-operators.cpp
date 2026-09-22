// Operators as members: +=, [], unary -, and the call operator.
#include <cstdio>

class Vec3 {
public:
    Vec3(int x, int y, int z) : v{x, y, z} {}
    Vec3& operator+=(const Vec3& o) {
        for (int i = 0; i < 3; ++i) v[i] += o.v[i];
        return *this;
    }
    int& operator[](int i) { return v[i]; }
    Vec3 operator-() const { return {-v[0], -v[1], -v[2]}; }
    int operator()(int a, int b, int c) const { return v[0] * a + v[1] * b + v[2] * c; }

private:
    int v[3];
};

int main() {
    Vec3 a(1, 2, 3);
    a += Vec3(10, 20, 30);
    a[1] = 5;
    Vec3 n = -a;
    std::printf("%d %d %d %d\n", a[0], a[1], n[2], a(1, 1, 1));
    return 0;
}
