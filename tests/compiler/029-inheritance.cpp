// §11.7 -- a base's members are reached at the base subobject's offset plus
// their own, which is the arithmetic tests/abi checks and this exercises.
struct Base { int b; };
struct Middle : Base { int m; };
struct Derived : Middle { int d; };

int readBase(Base* p) { return p->b; }

int main() {
    Derived x;
    x.b = 1;
    x.m = 2;
    x.d = 3;

    Middle* asMiddle = &x;
    Base* asBase = &x;

    return x.b + x.m * 10 + x.d * 100
        + asMiddle->b * 1000
        + readBase(asBase) * 10000;
}
