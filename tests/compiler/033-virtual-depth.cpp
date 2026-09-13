// The table at each level of a hierarchy: an override at the bottom, a
// function inherited unchanged from the middle, and a call qualified to
// bypass dispatch altogether (§11.7.3/12).
struct A {
    virtual int f() { return 1; }
    virtual int g() { return 10; }
    virtual int h() { return 100; }
};
struct B : A {
    int f() override { return 2; }
    virtual int extra() { return 1000; }
};
struct C : B {
    int g() override { return 30; }
    int extra() override { return 3000; }
};

int viaA(A* a) { return a->f() + a->g() + a->h(); }
int viaB(B* b) { return b->extra(); }

int main() {
    C c;
    B b;
    A a;
    int n = viaA(&c) + viaA(&b) + viaA(&a);
    n = n + viaB(&c) + viaB(&b);
    n = n + c.A::f();
    return n % 251;
}
