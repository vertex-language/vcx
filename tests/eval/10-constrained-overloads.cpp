// §13.5.4 [temp.constr.order] -- two functions of one signature told
// apart by their requires-clauses: the unsatisfied one is not a
// candidate, and among the satisfied a constrained one beats an
// unconstrained one. A constraint that is ill-formed for the arguments
// is unsatisfied, and says nothing (§13.5.2/3). Partial specializations
// match their pattern exactly (§13.10.3.6): `T &` takes only a
// reference, `const T` only a const type.
//
// The evaluator does not call member functions yet, so the member
// overloads -- and the implicit object parameter that ranks them -- are
// exercised at run time in tests/compiler/047 rather than here.

template <class T> concept Big = sizeof(T) > 2;

template <class T> constexpr int f(T) { return 1; }
template <class T> requires Big<T> constexpr int f(T) { return 2; }
static_assert(f('c') == 1);
static_assert(f(1) == 2);

// A constraint whose expression has no meaning for the argument: not
// an error, just not this overload.
template <class T> constexpr int g(T) { return 1; }
template <class T> requires (sizeof(typename T::type) > 0) constexpr int g(T) { return 2; }
struct HasType { using type = int; };
static_assert(g(1) == 1);
static_assert(g(HasType{}) == 2);

// Patterns are exact.
template <class T> struct is_ref { static constexpr bool value = false; };
template <class T> struct is_ref<T&> { static constexpr bool value = true; };
template <class T> struct is_ref<T&&> { static constexpr bool value = true; };
template <class T> struct is_const { static constexpr bool value = false; };
template <class T> struct is_const<const T> { static constexpr bool value = true; };
static_assert(!is_ref<int>::value);
static_assert(is_ref<int&>::value);
static_assert(is_ref<int&&>::value);
static_assert(!is_const<int>::value);
static_assert(is_const<const int>::value);
static_assert(!is_const<const int&>::value);

int main() { return 0; }
