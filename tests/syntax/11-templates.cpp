// §13 [temp] -- the template-parameter-list, the template-id, and the four
// kinds of declaration a template can be.

// §13.2 [temp.param] -- every parameter kind, and the defaults on each.
template <typename T> struct TypeParam;
template <class T> struct ClassParam;
template <typename T = int> struct DefaultedType;
template <int N> struct NonType;
template <int N = 4> struct DefaultedNonType;
template <auto V> struct DeducedNonType;
template <typename T, T V> struct DependentNonType;
template <template <typename> class TT> struct TemplateParam;
template <template <typename> typename TT> struct TemplateParam2;
template <template <typename> class TT = TypeParam> struct DefaultedTemplate;
template <typename... Ts> struct TypePack;
template <int... Ns> struct NonTypePack;
template <template <typename> class... TTs> struct TemplatePack;
template <typename T, int N, template <typename> class TT, typename... Rest>
struct Everything;

// §13.7 [temp.decls] -- the four things a template declaration can declare.
template <typename T> struct Class { T v; };
template <typename T> void function(T);
template <typename T> T variable = T();
template <typename T> using Alias = Class<T>;

// A member template, and a member of a class template defined out of line.
template <typename T> struct Outer {
    template <typename U> struct Inner;
    template <typename U> void member(U);
    template <typename U> using MemberAlias = U;
    void plain();
    static T s;
};
template <typename T> void Outer<T>::plain() {}
template <typename T> template <typename U> void Outer<T>::member(U) {}
template <typename T> T Outer<T>::s = T();

// §13.7.6 [temp.spec.partial] and §13.9.4 [temp.expl.spec].
template <typename T, typename U> struct Partial {};
template <typename T> struct Partial<T, int> {};
template <typename T> struct Partial<T*, T*> {};
template <typename T, typename U> struct Partial<T*, U> {};
template <> struct Partial<char, char> {};
template <> void function<int>(int) {}

// §13.9.2 [temp.explicit] -- explicit instantiation, both directions.
template struct Class<double>;
extern template struct Class<float>;
template void function<double>(double);

// §13.3 [temp.names] -- a template-id in every position that takes a type,
// and the disambiguators of §13.8 that a dependent name needs.
Class<int> ci;
Class<Class<int>> nested;          // §13.3/3: `>>` closes two lists
Class<int(*)(int)> fnptr;
Class<int[4]> arrayarg;

template <typename T> struct Dependent {
    typename T::type member;                 // §13.8.3.2, `typename`
    typename Outer<T>::template Inner<int>* p;
    void f() { this->template member<int>(); }
    using U = typename T::template Rebind<int>;
};

// §13.10 [temp.fct] -- deduction on a call, and the explicit form.
template <typename T> T identity(T);
template <typename T, typename U> auto pair_of(T, U) -> T;

// §13.7.4 [temp.variadic] -- packs, and every place one can be expanded.
template <typename... Ts> struct Pack {
    static constexpr int size = sizeof...(Ts);
    using tuple = Class<Pack<Ts...>>;
};
template <typename... Ts> void expand(Ts... ts);
template <typename... Ts> void expand_refs(Ts&&... ts);
template <typename... Ts> struct Bases : Ts... {};
template <typename... Ts> void fold_left(Ts... ts) { (void)(... + ts); }
template <typename... Ts> void fold_right(Ts... ts) { (void)(ts + ...); }
template <typename... Ts> void fold_init_l(Ts... ts) { (void)(0 + ... + ts); }
template <typename... Ts> void fold_init_r(Ts... ts) { (void)(ts + ... + 0); }
template <typename... Ts> void fold_comma(Ts... ts) { ((void)ts, ...); }

// §13.7.2.3 [temp.friend] and §13.7.5, the deduction guides of §12.4.1.8.
template <typename T> struct Guided { Guided(T); };
Guided(int) -> Guided<long>;
template <typename T> Guided(T*) -> Guided<T>;
