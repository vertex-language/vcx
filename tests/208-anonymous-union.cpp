// A struct with an anonymous union: its members are the struct's.
#include <cstdio>

struct Value {
    enum Kind { Int, Real } kind;
    union {
        int i;
        double d;
    };
};

double as_double(const Value& v) { return v.kind == Value::Int ? v.i : v.d; }

int main() {
    Value a{Value::Int, {}};
    a.i = 7;
    Value b{Value::Real, {}};
    b.d = 2.25;
    std::printf("%g %g %zu\n", as_double(a), as_double(b), sizeof(Value));
    return 0;
}
