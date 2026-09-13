// §13.5 [temp.constr] and §13.7.9 [temp.concept] -- concepts, requirements,
// and the four places a constraint can be written.

template <typename T> struct Class;

// §13.7.9 -- a concept is a named constraint-expression.
template <typename T> concept Always = true;
template <typename T> concept Sized = sizeof(T) > 0;
template <typename T, typename U> concept Same = __is_same(T, U);
template <typename... Ts> concept AllAlways = (Always<Ts> && ...);

// §13.5.2 [temp.constr.op] -- conjunction, disjunction, and the atomic
// constraint that is neither.
template <typename T> concept Conjunction = Always<T> && Sized<T>;
template <typename T> concept Disjunction = Always<T> || Sized<T>;
template <typename T> concept Mixed = (Always<T> || Sized<T>) && Always<T>;
template <typename T> concept Negated = !Always<T>;

// §7.5.7 [expr.prim.req] -- the four requirement kinds.
template <typename T> concept Requirements = requires(T a, T b) {
    a + b;                              // simple-requirement
    typename T::type;                   // type-requirement
    { a + b };                          // compound-requirement
    { a + b } -> Always;                //   with a return-type-requirement
    { a + b } noexcept;                 //   with noexcept
    { a + b } noexcept -> Same<T>;      //   with both
    requires Always<T>;                 // nested-requirement
    requires sizeof(T) > 0;
};

// A requires-expression need not take a parameter list, and it is an
// expression, so it can appear wherever one can.
template <typename T> concept NoParams = requires { typename T::type; };
template <typename T> constexpr bool value = requires { typename T::type; };

// §13.5.3 [temp.constr.decl] -- the four spellings of a constrained
// declaration, which mean the same thing.
template <typename T> requires Always<T> void a(T);
template <Always T> void b(T);
void c(Always auto);
template <typename T> void d(T) requires Always<T>;

// A requires-clause may itself be a requires-expression, which is where the
// doubled keyword comes from.
template <typename T> requires requires(T x) { x + x; } void e(T);

// Constrained placeholders: §9.2.9.7 [dcl.spec.auto].
Always auto f1() { return 0; }
auto f2(Always auto x) { return x; }
Always auto f3 = 0;
Always auto& f4 = f3;
void f5() {
    Always auto v = 0;
    Always auto& r = v;
    const Always auto c = 0;
    (void)r; (void)c;
}
template <Always auto V> struct ConstrainedNonType;

// Constrained partial specializations and member functions.
template <typename T> struct Constrained {
    void plain();
    void only() requires Always<T>;
    template <typename U> requires Same<T, U> void both(U);
};
template <typename T> requires Sized<T> struct Specialized {};
