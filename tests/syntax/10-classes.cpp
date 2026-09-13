// §11 [class] -- the class head, the member specification, and the four
// things that only exist inside one.

// §11.1 [class.pre]: the three class-keys, and the head that may be empty.
struct S1 {};
class C1 {};
union U1 { int i; float f; };
struct { int anon; } anon_var;
union { int a; float b; } anon_union_var;

// §11.4 [class.mem] -- members, and the access-specifiers of §11.8.
class Members {
public:
    int pub;
    void pub_fn();
protected:
    int prot;
private:
    int priv;
public:
    // Static, mutable, and the member types.
    static int count;
    static constexpr int limit = 10;
    mutable int cache;
    using Alias = int;
    typedef int Typedef;
    struct Nested { int m; };
    enum E { A, B };
    enum class EC { X, Y };

    // §11.4.9 [class.bit] -- bit-fields, including the unnamed one.
    unsigned flag : 1;
    unsigned pair : 2;
    unsigned : 0;
    unsigned rest : 5;

    // §11.4.1/1 -- a member with a default member initializer.
    int inited = 1;
    int braced{2};
};
int Members::count = 0;

// §11.4.5 [class.ctor], §11.4.7 [class.dtor], §11.4.8 [class.conv].
struct Special {
    Special();
    Special(int);
    Special(const Special&);
    Special(Special&&);
    Special& operator=(const Special&);
    Special& operator=(Special&&);
    ~Special();
    explicit Special(double);
    explicit(true) Special(char);       // §12.4.2, conditional explicit
    operator int() const;
    explicit operator bool() const;

    // §11.4.5/1 -- the mem-initializer-list is part of the definition.
    Special(int a, int b) : x(a), y(b) {}
    Special(int a, int b, int) : x{a}, y{b} {}

    int x, y;
};

// §11.4.6 [class.copy] by its defaulted and deleted spellings, and the
// C++20 defaulted comparison of §11.11.
struct Defaults {
    Defaults() = default;
    Defaults(const Defaults&) = delete;
    Defaults& operator=(const Defaults&) = default;
    ~Defaults() = default;
    bool operator==(const Defaults&) const = default;
    auto operator<=>(const Defaults&) const = default;
};

// §11.7 [class.derived] -- base-clause, access, and virtual bases.
struct B1 {}; struct B2 {}; struct B3 {};
struct D1 : B1 {};
struct D2 : public B1 {};
struct D3 : private B1 {};
struct D4 : protected B1 {};
struct D5 : virtual B1 {};
struct D6 : public virtual B1 {};
struct D7 : virtual public B1 {};
struct D8 : B1, B2, B3 {};
struct D9 : public B1, private B2, protected virtual B3 {};
class D10 : B1 {};              // private by default for `class`
struct D11 final : B1 {};

// §11.7.3 [class.virtual] -- the specifiers on a virtual member.
struct V {
    virtual void a();
    virtual void b() = 0;
    virtual ~V();
};
struct VD : V {
    void a() override;
    void b() override final;
    ~VD() override;
};

// §11.8.4 [class.friend] -- a friend is a declaration that grants access.
class Friendly {
    friend class Other;
    friend struct AlsoOther;
    friend int free_function(Friendly&);
    friend int inline_friend(Friendly&) { return 0; }
    int secret;
};

// §11.9 [class.local] -- a class declared inside a function.
void local() {
    struct Local {
        int m;
        int f() { return m; }
    };
    Local l{0};
    (void)l;
}
