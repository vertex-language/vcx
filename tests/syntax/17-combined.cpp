// The combinations. Nothing here introduces a production the earlier files
// have not already covered on its own; what it does is put them inside each
// other, which is where a parser that handles each in isolation stops.

template <typename T> concept Addable = requires(T a) { a + a; };
template <typename T> struct Trait { using type = T; static constexpr int n = 1; };

// A template whose parameter list holds a default that is a template-id
// holding a template-id, and whose constraint is a fold over a pack.
template <typename T = Trait<Trait<int>>::type,
          int N = Trait<int>::n,
          typename... Rest>
    requires (Addable<T> && ... && Addable<Rest>)
struct Deep {
    // A member template, constrained, returning a trailing type that is a
    // dependent template-id, with a default argument that is a lambda call.
    template <typename U>
        requires Addable<U>
    auto member(U u, int k = [] { return 1; }()) const noexcept(noexcept(u + u))
        -> typename Trait<U>::type;

    // A pointer to a member function of a dependent type, defaulted.
    typename Trait<T>::type (Deep::*pmf)(T) const = nullptr;

    // A bit-field whose width is a constant expression with a `>` in it,
    // parenthesized so the template-argument rule does not claim it.
    unsigned field : (sizeof(T) > 1 ? 4 : 2);

    // Nested class with its own base list, its own template, and an
    // in-class initializer that is a braced list of lambda results.
    template <typename U> struct Inner : Trait<U>, Trait<T> {
        int pair[2] = {[] { return 1; }(), [] { return 2; }()};
        auto operator<=>(const Inner&) const = default;
    };
};

// Out-of-line definition of the member template of a constrained class
// template: three template-parameter-lists and a requires-clause on each.
template <typename T, int N, typename... Rest>
    requires (Addable<T> && ... && Addable<Rest>)
template <typename U>
    requires Addable<U>
auto Deep<T, N, Rest...>::member(U u, int k) const noexcept(noexcept(u + u))
    -> typename Trait<U>::type { return u + k; }

// A partial specialization whose arguments are themselves template-ids, with
// a nested pack expansion in the base list.
template <typename... Ts> struct Bases : Trait<Ts>... {
    using Trait<Ts>::type...;
};
template <typename T> struct Deep<Trait<T>, 1> {};

// A generic lambda taking a pack, expanded inside a fold inside an
// initializer inside a default argument.
auto folded = []<typename... Us>(Us... us) {
    return (... + static_cast<int>(sizeof(Us)));
};
void with_default(int n = []<typename... Us>(Us... us) { return (0 + ... + us); }(1, 2)) {}

// A function-try-block on a function whose return type is a dependent
// template-id and whose body holds a lambda that throws.
template <typename T>
typename Trait<T>::type guarded(T v) try {
    auto inner = [v] { if (v) throw 1; return v; };
    return inner();
} catch (...) {
    return T();
}

// Every disambiguator at once: a dependent name that is a type, holding a
// dependent template that is called, inside a decltype in a trailing return.
// The operand would be `typename Trait<U>::type{} + u` if §7.6.1.4's braced
// functional cast parsed -- see 05 -- so it is spelled with a declared
// object instead, which leaves the `typename` and the `template` in place.
template <typename T> struct Chain {
    template <typename U> static auto go(U u)
        -> decltype(u + u);
    template <typename U> static typename Trait<U>::type also(U u);
    void use() { Chain::template go<T>(T()); }
};
