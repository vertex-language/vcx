// Temporaries are destroyed at the end of the full expression.
#include <cstdio>

struct T {
    int v;
    T(int x) : v(x) { std::printf("make %d\n", v); }
    ~T() { std::printf("end %d\n", v); }
};

int value(const T& t) { return t.v; }

int main() {
    std::printf("in the expression %d\n", value(T(1)) + 1);
    int a = value(T(2));
    std::printf("after the statement %d\n", a);
    const T& kept = T(3);
    std::printf("kept %d\n", kept.v);
    return 0;
}
