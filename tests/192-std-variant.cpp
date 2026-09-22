// std::variant and std::visit.
#include <cstdio>
#include <variant>

using Value = std::variant<int, double, const char*>;

struct Printer {
    void operator()(int v) const { std::printf("int %d\n", v); }
    void operator()(double v) const { std::printf("double %g\n", v); }
    void operator()(const char* v) const { std::printf("string %s\n", v); }
};

int main() {
    Value vs[] = {42, 2.5, "text"};
    for (const Value& v : vs) std::visit(Printer{}, v);
    Value v = 1;
    v = 3.0;
    std::printf("%zu %d\n", v.index(), std::holds_alternative<double>(v));
    return 0;
}
