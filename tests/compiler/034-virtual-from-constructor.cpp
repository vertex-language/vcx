// §11.9.5/4 -- a virtual call made while a constructor runs reaches the
// function of the class *being constructed*, not the most-derived one,
// because the derived part does not exist yet. Each constructor installs
// its own table before its body runs, which is what makes that true.
struct Base {
    int seen;
    Base() : seen(0) { seen = tag(); }
    virtual int tag() { return 1; }
};

struct Derived : Base {
    int later;
    Derived() : Base() { later = tag(); }
    int tag() override { return 2; }
};

int main() {
    Derived d;
    return d.seen * 10 + d.later;
}
