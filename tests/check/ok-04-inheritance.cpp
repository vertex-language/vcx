struct Base {
    int b = 1;
    virtual int f() { return 1; }
    virtual ~Base() {}
};

struct Middle : Base {
    int m = 2;
    int f() override { return 2; }
};

struct Derived : Middle {
    int d = 3;
    int f() override { return Middle::f() + d; }
};

int through_base(Base* p) { return p->f(); }

int use() {
    Derived d;
    Base* p = &d;
    return through_base(p) + d.b + d.m;
}
