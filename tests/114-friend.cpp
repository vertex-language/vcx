// A friend function reaches private members.
#include <cstdio>

class Secret {
public:
    explicit Secret(int v) : v_(v) {}
    friend int reveal(const Secret& s);
    friend class Inspector;

private:
    int v_;
};

int reveal(const Secret& s) { return s.v_; }

class Inspector {
public:
    static int doubled(const Secret& s) { return s.v_ * 2; }
};

int main() {
    Secret s(21);
    std::printf("%d %d\n", reveal(s), Inspector::doubled(s));
    return 0;
}
