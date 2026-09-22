// const and non-const overloads of a member function.
#include <cstdio>

class Box {
public:
    int& get() { return v_; }
    const int& get() const { return v_; }
    const char* which() { return "mutable"; }
    const char* which() const { return "const"; }

private:
    int v_ = 1;
};

int main() {
    Box b;
    const Box& cb = b;
    b.get() = 42;
    std::printf("%d %s %s\n", cb.get(), b.which(), cb.which());
    return 0;
}
