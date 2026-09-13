// §13.7.1 [temp.variable], §13.2/12 default template arguments, §13.7.4
// parameter packs and §7.5.6 fold expressions -- the machinery the type
// traits headers are written in, answered at compile time.
//
// Every assertion is paired with its negation for the same reason as in
// 08: a trait that defaults to true produces no diagnostic anywhere.

// A variable template, and its partial and explicit specializations.
template <class T> constexpr bool is_pointer_v = false;
template <class T> constexpr bool is_pointer_v<T *> = true;
template <class T> constexpr bool is_pointer_v<T *const> = true;
static_assert(is_pointer_v<int *>);
static_assert(is_pointer_v<int *const>);
static_assert(!is_pointer_v<int>);
static_assert(!is_pointer_v<int &>);

template <class T> constexpr int kind = 0;
template <> constexpr int kind<int> = 7;
static_assert(kind<int> == 7);
static_assert(kind<char> == 0);

// §13.10.3.6 -- a value parameter bound from an array bound in a pattern.
template <class T> constexpr unsigned rank_v = 0;
template <class T, unsigned long long N> constexpr unsigned rank_v<T[N]> = rank_v<T> + 1;
template <class T> constexpr unsigned rank_v<T[]> = rank_v<T> + 1;
static_assert(rank_v<int> == 0);
static_assert(rank_v<int[2]> == 1);
static_assert(rank_v<int[2][3]> == 2);
static_assert(rank_v<int[][3]> == 2);

template <class T, unsigned I = 0> struct extent { static constexpr unsigned long long value = 0; };
template <class T, unsigned long long N> struct extent<T[N], 0> { static constexpr unsigned long long value = N; };
template <class T, unsigned long long N, unsigned I> struct extent<T[N], I> { static constexpr unsigned long long value = extent<T, I - 1>::value; };
template <class T, unsigned I> struct extent<T[], I> { static constexpr unsigned long long value = extent<T, I - 1>::value; };
static_assert(extent<int[3][4]>::value == 3);   // the default I = 0
static_assert(extent<int[3][4], 1>::value == 4);
static_assert(extent<int[][4], 1>::value == 4);
static_assert(extent<int>::value == 0);

// §13.2/12 -- a default that reads the parameter before it.
template <class T, T v> struct integral_constant { static constexpr T value = v; };
template <bool B> using bool_constant = integral_constant<bool, B>;
template <class T> constexpr bool is_int_v = false;
template <> constexpr bool is_int_v<int> = true;
template <class T, bool = is_int_v<T>> struct sign_base { static constexpr bool is_signed = true; };
template <class T> struct sign_base<T, false> { static constexpr bool is_signed = false; };
template <class T> struct is_signed : bool_constant<sign_base<T>::is_signed> {};
static_assert(is_signed<int>::value);
static_assert(!is_signed<char>::value);

// §13.7.4 -- packs: absorbed, expanded, matched by a pattern's tail.
using true_type = integral_constant<bool, true>;
using false_type = integral_constant<bool, false>;
template <bool First, class F, class... Rest> struct conj_impl { using type = F; };
template <class True, class Next, class... Rest> struct conj_impl<true, True, Next, Rest...> {
    using type = typename conj_impl<static_cast<bool>(Next::value), Next, Rest...>::type;
};
template <class... Traits> struct conjunction : true_type {};
template <class First, class... Rest> struct conjunction<First, Rest...>
    : conj_impl<static_cast<bool>(First::value), First, Rest...>::type {};
template <class... Traits> constexpr bool conjunction_v = conjunction<Traits...>::value;
static_assert(conjunction_v<>);
static_assert(conjunction_v<true_type>);
static_assert(conjunction_v<true_type, true_type>);
static_assert(!conjunction_v<true_type, false_type, true_type>);
static_assert(!conjunction<false_type>::value);

// §7.5.6 -- folds over a pack, of types through a trait and of values.
template <class T, class U> constexpr bool is_same_v = false;
template <class T> constexpr bool is_same_v<T, T> = true;
template <class T, class... Ts> constexpr bool any_of_v = (is_same_v<T, Ts> || ...);
static_assert(any_of_v<int, char, int, long>);
static_assert(!any_of_v<int, char, long>);
static_assert(!any_of_v<int>);   // an empty pack under || is false
template <int... Ns> constexpr int sum_v = (Ns + ... + 0);
static_assert(sum_v<1, 2, 3> == 6);
static_assert(sum_v<> == 0);
template <int... Ns> constexpr bool all_positive = ((Ns > 0) && ...);
static_assert(all_positive<1, 2>);
static_assert(!all_positive<1, -2>);
static_assert(all_positive<>);   // and under && it is true

int main() { return 0; }
