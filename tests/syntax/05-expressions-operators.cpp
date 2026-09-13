// §7.6 [expr.compound] -- the operators, in the order the clause gives them,
// which is also the order they bind in.

struct S { int m; int f(int); S* operator->(); };
int fn(int);
int arr[4];

void unary(int a, double d, S* p, S& r) {
    // §7.6.1 postfix
    arr[0]; 0[arr];
    fn(a);
    int(a); (int)a;
    p->m; r.m;
    a++; a--;
    dynamic_cast<S*>(p);
    static_cast<double>(a);
    reinterpret_cast<char*>(p);
    const_cast<S*>(p);

    // §7.6.2 unary
    ++a; --a;
    +a; -a; !a; ~a;
    *p; &a;
    sizeof(int); sizeof a;
    alignof(int);
    new int; new int[4]; delete p; delete[] arr;
    noexcept(a);

    // §7.6.4 pointer-to-member, §7.6.5 through §7.6.14: the binary chain.
    int S::*pm = &S::m;
    r.*pm; p->*pm;

    a * a; a / a; a % a;
    a + a; a - a;
    a << 1; a >> 1;
    a <=> a;                    // §7.6.8 three-way comparison
    a < a; a > a; a <= a; a >= a;
    a == a; a != a;
    a & a; a ^ a; a | a;
    a && a; a || a;

    // §7.6.16 conditional, §7.6.19 assignment, §7.6.20 comma.
    a ? a : a;
    a = a; a += a; a -= a; a *= a; a /= a; a %= a;
    a <<= 1; a >>= 1; a &= a; a ^= a; a |= a;
    a, a;

    // §7.6.1.4 [expr.type.conv] -- the functional notation, in both forms.
    // In expression-statement position the paren form is the ambiguity of
    // §8.9/2 and is read as a declaration, so it is written where an
    // expression is the only reading.
    (void)S{0};
    (void)S(0);
    (void)int(a);
    (void)int{a};

    // A cast to a compound type needs a name for the type, which is what
    // makes the alias below part of the operator grammar rather than decoration.
    (void)d;
    using P = int*;
    P q = P();
    (void)q;

    // The alternative tokens of §5.5, which are the same operators.
    a and a; a or a; a not_eq a; compl a; not a;
    a bitand a; a bitor a; a xor a;
    a and_eq a; a or_eq a; a xor_eq a;
}

// §7.6.6/7.6.7 -- the operators are also declarable, so the whole table
// appears a second time as declarator-ids.
struct T {
    T operator+(T) const; T operator-(T) const;
    T operator*(T) const; T operator/(T) const; T operator%(T) const;
    T operator^(T) const; T operator&(T) const; T operator|(T) const;
    T operator~() const;  bool operator!() const;
    T& operator=(const T&); bool operator<(T) const; bool operator>(T) const;
    T& operator+=(T); T& operator-=(T); T& operator*=(T); T& operator/=(T);
    T& operator%=(T); T& operator^=(T); T& operator&=(T); T& operator|=(T);
    T& operator<<=(int); T& operator>>=(int);
    bool operator==(T) const; bool operator!=(T) const;
    bool operator<=(T) const; bool operator>=(T) const;
    auto operator<=>(T) const;
    bool operator&&(T) const; bool operator||(T) const;
    T operator<<(int) const; T operator>>(int) const;
    T& operator++(); T operator++(int);
    T& operator--(); T operator--(int);
    T* operator->(); T& operator*();
    T& operator[](int);
    int operator()(int) const;
    T& operator,(T&);
    operator bool() const;
    explicit operator int() const;
    static void* operator new(unsigned long long);
    static void operator delete(void*);
};
