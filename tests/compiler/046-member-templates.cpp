// Do member function templates become code, in the class they belong to?
//
// §13.7.3 -- a class's member can itself be a template, with a head of
// its own: a constructor template that takes anything convertible, a
// method template deduced from its argument, a static one with two
// parameters. Each instance is a member of the class -- it has `this`,
// its name is the class's for a constructor -- and its object-file name
// says which arguments made it (`??$?0N@P@@QEAA@AEBN@Z` is P::P<double>).
//
// With them come the pieces the library writes such templates with: a
// forwarding reference `U &&` deduced from an lvalue as `int &`
// (§13.10.3.2/3), `forward<U>(u)` called with its arguments written out
// and qualified, a parameter in a non-deduced context (`remove_reference_t
// <T> &`), and a template parameter nothing deduces that takes its
// default -- the enable_if idiom, whose failure to substitute is one
// candidate fewer rather than an error.

template <class T> struct remove_reference { using type = T; };
template <class T> struct remove_reference<T&> { using type = T; };
template <class T> struct remove_reference<T&&> { using type = T; };
template <class T> using remove_reference_t = typename remove_reference<T>::type;

namespace lib {
template <class T> constexpr T&& forward(remove_reference_t<T>& arg) noexcept { return static_cast<T&&>(arg); }
template <class T> constexpr T&& forward(remove_reference_t<T>&& arg) noexcept { return static_cast<T&&>(arg); }
template <class T> constexpr remove_reference_t<T>&& move(T&& arg) noexcept { return static_cast<remove_reference_t<T>&&>(arg); }
}

template <bool B, class T = void> struct enable_if {};
template <class T> struct enable_if<true, T> { using type = T; };
template <bool B, class T = void> using enable_if_t = typename enable_if<B, T>::type;
template <class T> constexpr bool is_int_v = false;
template <> constexpr bool is_int_v<int> = true;

struct P {
    int a;
    template <class U> P(const U& x) : a((int)x) {}
    template <class U> int add(U v) const { return a + (int)v; }
    template <class U, class V> static int both(U u, V v) { return (int)u + (int)v; }
};

template <class T> struct Q {
    T t;
    template <class U> Q(const U& x) : t((T)x) {}
    template <class U> T get(U v) const { return t + (T)v; }
};

struct Pair {
    int first, second;
    template <class U, class V> Pair(U&& u, V&& v) : first(lib::forward<U>(u)), second(lib::forward<V>(v)) {}
};

template <class T, enable_if_t<is_int_v<T>, int> = 0>
int only_int(T v) { return v * 2; }

template <class T> void swap(T& l, T& r) { T tmp = lib::move(l); l = lib::move(r); r = lib::move(tmp); }

int main() {
    P p(2.5);                            // P<double>: a = 2
    Q<int> q(1);                         // Q<int>::Q<int>
    int x = 4;
    Pair pr(x, 3);                       // U = int&, V = int
    int a = 3, b = 4;
    swap(a, b);                          // a = 4, b = 3
    return p.add(1.5) + P::both(1, 2L) + q.get('a') + pr.first + pr.second + only_int(5) + a * 10 + b;
    // 3 + 3 + 98 + 4 + 3 + 10 + 43 = 164
}
