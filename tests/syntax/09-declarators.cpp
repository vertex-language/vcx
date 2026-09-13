// §9.3 [dcl.decl] -- the declarator grammar, which reads outward from the
// name and is the part of C++ that C already made hard.

int x;
int* p;
int** pp;
int& r = x;
int&& rr = 0;
const int* pc;
int* const cp = &x;
const int* const cpc = &x;
int* volatile vp;

int a[4];
int a2[2][3];
int* pa[4];        // array of pointers
int (*ap)[4];      // pointer to array

int f();
int g(int);
int h(int, double);
int v(int, ...);
int nop(void);
int (*fp)(int);            // pointer to function
int (*fpa[4])(int);        // array of pointers to function
int (*(*fpp)(int))(double);// pointer to function returning pointer to function
int& fr();                 // function returning a reference
int (&far())[4];           // function returning a reference to an array

// §9.3.4.6 [dcl.fct] -- the trailing return type, and the specifiers that
// only exist after the parameter list.
auto tr() -> int;
auto trp(int) -> int*;
auto trf(int) -> int (*)(int);
int cf() noexcept;
int nf() noexcept(true);
[[nodiscard]] int af();

// §11.4 [class.mem] -- the cv- and ref-qualifiers a member declarator adds,
// and the pointer-to-member declarator of §9.3.4.4.
struct S {
    int m;
    int f();
    int cfn() const;
    int vfn() volatile;
    int cvfn() const volatile;
    int lfn() &;
    int rfn() &&;
    int clfn() const&;
    virtual int pure() = 0;
    virtual int impure() { return 0; }
    virtual int overridden() final { return 0; }
};
int S::*pm = &S::m;
int (S::*pmf)() = &S::f;
int (S::*pmfc)() const = &S::cfn;

// §9.4 [dcl.init] -- every initializer form the declarator can carry.
int i1 = 0;
int i2{0};
int i3 = {0};
int i4{};
int arr1[] = {1, 2, 3};
int arr2[3] = {1};
int arr3[3] = {};
struct Agg { int a; int b; };
Agg g1 = {1, 2};
Agg g2{1, 2};
Agg g3 = {.a = 1, .b = 2};    // §9.4.5 designated initializers
Agg g4{.b = 2};

// §9.3.4.7 [dcl.fct.default] and §9.3.4.6/3, the parameter with no name.
int deflt(int a = 1, double b = 2.0, const char* c = "s");
int unnamed(int, double, char);
int mixed(int a, double = 1.0);

// §9.5.1 [dcl.fct.def.general] -- a definition is a declarator and a body,
// and the two bodies that are not compound statements.
struct D {
    D() = default;
    D(const D&) = delete;
    ~D() = default;
};
