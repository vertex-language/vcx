// §13.10.3 [temp.deduct] and §13.8 [temp.res] -- the two rules that let a
// template be written once and used with anything.
//
// Deduction reads the template's parameters off the call's arguments, and
// substitution puts them back into the signature, so a call to a template
// has a type. Two-phase lookup is the other half: an expression in the body
// that depends on a parameter is not checked here at all, because whether
// `x + x` is well-formed is a question about the T that has not arrived.

// §13.10.3.2 [temp.deduct.call] -- deduction from the arguments.
template <typename T> T identity(T x) { return x; }
template <typename T> T pick(T a, T b) { return a < b ? b : a; }
template <typename A, typename B> A first(A a, B b) { return a; }

int deduced_from_arguments() {
    int i = identity(5);
    double d = identity(1.5);
    int m = pick(1, 2);
    int fst = first(1, 2.5);
    return i + m + fst + static_cast<int>(d);
}

// §13.10.3.2/2 -- a by-value parameter drops the argument's reference and
// top-level qualification, so T is int in every one of these; a reference
// parameter keeps the argument's identity, so T is int there too.
template <typename T> T by_value(T x) { return x; }
template <typename T> T by_const_ref(const T& x) { return x; }
template <typename T> T* address_of(T& x) { return &x; }

int adjustments() {
    int a = 1;
    const int c = 2;
    int& r = a;
    return by_value(a) + by_value(c) + by_value(r) + by_const_ref(a) + *address_of(a);
}

// Deduction through a pointer, and through an array that decays to one.
template <typename T> T dereference(T* p) { return *p; }
template <typename T> T head(T* p) { return p[0]; }

int through_pointers() {
    int a = 1;
    int arr[3] = {1, 2, 3};
    return dereference(&a) + head(arr);
}

// §13.10.2 [temp.arg.explicit] -- a parameter that appears nowhere in the
// parameter list can only be supplied by writing it out.
template <typename T> T zero() { return T(); }
int explicit_arguments() { return zero<int>() + static_cast<int>(zero<double>()); }

// §13.8/1 [temp.res] -- the body is checked twice. Nothing below depends on
// anything but T, so nothing below is checked until a call supplies one --
// which is what lets a template use operators, members and calls that no
// type in this file has.
template <typename T> T twice(T x) { return x + x; }
template <typename T> bool less(T a, T b) { return a < b; }
template <typename T> auto member_of(T t) { return t.value; }
template <typename T> auto method_of(T t) { return t.method(); }
template <typename T> auto arrow_of(T* p) { return p->value; }
template <typename T> auto subscript_of(T c) { return c[0]; }
template <typename T> auto indirect(T p) { return *p; }

struct Holder {
    int value;
    int method() const { return value; }
};

int uses_the_bodies() {
    Holder h{7};
    int arr[2] = {1, 2};
    int n = 3;
    return twice(2)
        + (less(1, 2) ? 1 : 0)
        + member_of(h)
        + method_of(h)
        + arrow_of(&h)
        + subscript_of(arr)
        + indirect(&n);
}

// A template calling a template, where the inner call's result is dependent
// too and the outer body still must not be checked.
template <typename T> T relay(T x) { return identity(x) + identity(x); }
int nested() { return relay(4); }

// A constrained template deduces the same way an unconstrained one does.
template <typename T> concept Numeric = __is_arithmetic(T);
template <Numeric T> T doubled(T x) { return x + x; }
int constrained() { return doubled(3) + static_cast<int>(doubled(1.5)); }
