// 059-multiple-inheritance.cpp
// Tests multiple inheritance without virtual functions:
// - Object layout and member offsets for secondary bases
// - Pointer adjustments during upcasting to secondary base
// - Constructor initialization order (bases in declaration order, then members)
// - Destructor execution order (members, then bases in reverse declaration order)

int order = 0;
int order_log[10];

struct BaseA {
    int a;
    BaseA(int a) : a(a) {
        order_log[order] = 1;
        order = order + 1;
    }
    ~BaseA() {
        order_log[order] = 10;
        order = order + 1;
    }
    int get_a() { return a; }
};

struct BaseB {
    int b;
    BaseB(int b) : b(b) {
        order_log[order] = 2;
        order = order + 1;
    }
    ~BaseB() {
        order_log[order] = 20;
        order = order + 1;
    }
    int get_b() { return b; }
};

struct Derived : BaseA, BaseB {
    int c;
    Derived(int a, int b, int c) : BaseA(a), BaseB(b), c(c) {
        order_log[order] = 3;
        order = order + 1;
    }
    ~Derived() {
        order_log[order] = 30;
        order = order + 1;
    }
    int sum() {
        return a + b + c;
    }
};

int test_offsets_and_casts() {
    Derived d(10, 20, 30);
    int direct_sum = d.sum(); // 60

    BaseA* pa = &d;
    BaseB* pb = &d;

    int va = pa->get_a(); // 10
    int vb = pb->get_b(); // 20

    // Cast pointer check
    return direct_sum + va + vb; // 60 + 10 + 20 = 90
}

int main() {
    int r1 = test_offsets_and_casts(); // 90

    // After d is destroyed, destructor order should have logged:
    // constructors: 1, 2, 3
    // destructors: 30, 20, 10
    int ctor_ok = (order_log[0] == 1 && order_log[1] == 2 && order_log[2] == 3);
    int dtor_ok = (order_log[3] == 30 && order_log[4] == 20 && order_log[5] == 10);

    int r2 = 0;
    if (ctor_ok) r2 += 15;
    if (dtor_ok) r2 += 25;

    // Total: 90 + 15 + 25 = 130
    return r1 + r2;
}
