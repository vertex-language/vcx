// Does a requires-expression discriminate?
//
// §7.5.7 [expr.prim.req]. A requires-expression is satisfied when each
// requirement is well-formed under the binding of the template parameters,
// which means forming the expression -- operator lookup, member lookup,
// overload resolution -- and asking whether anything went wrong. Every
// concept below has to answer differently for two types, because a
// constraint satisfied by everything is indistinguishable from none.
//
// No built-in trait is used: cl has no __is_same or __is_integral as an
// expression, so the concepts this file needs are written out.

template <typename T, typename U> inline constexpr bool same_v = false;
template <typename T> inline constexpr bool same_v<T, T> = true;
template <typename T, typename U> concept same_as = same_v<T, U>;

template <typename T> inline constexpr bool integral_v = false;
template <> inline constexpr bool integral_v<int> = true;
template <> inline constexpr bool integral_v<long> = true;
template <> inline constexpr bool integral_v<char> = true;
template <typename T> concept integral = integral_v<T>;

struct NoPlus {};
struct Cmp { bool operator==(const Cmp&) const; };
struct NoCmp {};
struct HasType { using value_type = int; };
struct Callable { int operator()(int) const; };
struct Thrower { int f(); };
struct Safe { int f() noexcept; };
struct Counter { Counter& operator++(); Counter operator++(int); };

// §7.5.7.2 [expr.prim.req.simple] -- the expression is valid.
template <typename T> concept Addable = requires(T a, T b) { a + b; };
static_assert(Addable<int>);
static_assert(Addable<double>);
static_assert(!Addable<NoPlus>);

template <typename T> concept Deref = requires(T p) { *p; };
static_assert(Deref<int*>);
static_assert(!Deref<int>);

template <typename T> concept Incr = requires(T x) { ++x; x++; };
static_assert(Incr<int>);
static_assert(Incr<char*>);
static_assert(Incr<Counter>);
static_assert(!Incr<NoPlus>);

// §7.5.7.3 [expr.prim.req.type] -- the name denotes a type.
template <typename T> concept HasValueType = requires { typename T::value_type; };
static_assert(HasValueType<HasType>);
static_assert(!HasValueType<int>);
static_assert(!HasValueType<NoPlus>);

// §7.5.7.4 [expr.prim.req.compound] -- the expression is valid, and its
// decltype satisfies the type-constraint with itself as the first argument.
template <typename T> concept EqComparable =
    requires(const T a, const T b) { { a == b } -> same_as<bool>; };
static_assert(EqComparable<int>);
static_assert(EqComparable<Cmp>);
static_assert(!EqComparable<NoCmp>);

template <typename T> concept IntCallable = requires(T f) { { f(1) } -> integral; };
static_assert(IntCallable<Callable>);
static_assert(!IntCallable<int>);

// -- and is noexcept when asked.
template <typename T> concept NothrowF = requires(T t) { { t.f() } noexcept; };
static_assert(NothrowF<Safe>);
static_assert(!NothrowF<Thrower>);

// §7.5.7.5 [expr.prim.req.nested] -- a constraint inside the body.
template <typename T> concept BigAddable = Addable<T> && requires { requires sizeof(T) >= 4; };
static_assert(BigAddable<int>);
static_assert(!BigAddable<char>);
static_assert(!BigAddable<NoPlus>);

// §13.5.3 [temp.constr.decl] -- a requires-expression as the constraint
// of a requires-clause, choosing between two overloads.
template <typename T> requires requires(T a) { a + a; }
constexpr int pick(T) { return 1; }
template <typename T> requires (!requires(T a) { a + a; })
constexpr int pick(T) { return 2; }
static_assert(pick(1) == 1);
static_assert(pick(NoPlus{}) == 2);

// §7.5.7/1 -- a requires-expression is a prvalue of type bool wherever
// an expression may stand, not only in a constraint.
template <typename T> constexpr bool addable_flag = requires(T a) { a + a; };
static_assert(addable_flag<int>);
static_assert(!addable_flag<NoPlus>);
