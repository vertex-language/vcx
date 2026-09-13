// Is a class template's member template its own template, and does an
// alias template's spelling take part in matching a partial
// specialization?
//
// §13.7.3 [temp.mem]: a template can be declared within a class or class
// template; `allocator<T>::rebind<U>` is a template of its own, with its
// own parameters, in every specialization of allocator. §13.7.8/2
// [temp.alias]: a specialization of an alias template is equivalent to
// the type it names -- `void_t<X>` is void whenever X is a type -- and
// §13.10.3.6/1 [temp.deduct.type] makes it a non-deduced context, so the
// classic `has_type<T, void_t<typename T::type>>` decides on the strength
// of the substitution alone: well-formed for a T with the member, void,
// the partial specialization matches; ill-formed otherwise, and it does
// not. §13.7.2/1 [temp.local]: inside a specialization the template's
// name with its own arguments is the class itself, not a second one.
//
// The trait arguments below use the builtin cl has as an expression,
// __is_nothrow_constructible, with a pack that expands into it (§13.7.4).

template <class T, class U> struct same { static constexpr bool value = false; };
template <class T> struct same<T, T> { static constexpr bool value = true; };

// Member class template, of a plain class and of a class template.
struct Plain { template <class U> struct Box { U u; using type = U*; }; };
static_assert(same<Plain::Box<int>::type, int*>::value);
static_assert(sizeof(Plain::Box<double>) == sizeof(double));

template <class T> struct allocator {
    T val;
    template <class U> struct rebind { using other = allocator<U>; };
    // The template's name with its own arguments names this class.
    using self = allocator<T>;
};
static_assert(same<allocator<int>::rebind<long>::other, allocator<long>>::value);
static_assert(same<allocator<int>::self, allocator<int>>::value);
static_assert(sizeof(allocator<int>::rebind<long>::other) == sizeof(long));

// Member alias template, reached with `template` through a dependent base.
template <class A> struct allocator_traits {
    template <class U> using rebind_alloc = typename A::template rebind<U>::other;
};
template <class A, class T> using rebind_t = typename allocator_traits<A>::template rebind_alloc<T>;
static_assert(same<allocator_traits<allocator<int>>::rebind_alloc<char>, allocator<char>>::value);
static_assert(same<rebind_t<allocator<int>, short>, allocator<short>>::value);

// void_t detection through a partial specialization.
template <class...> using void_t = void;
struct Yes { using type = int; };
struct No {};
template <class T, class = void> struct has_type { static constexpr bool value = false; };
template <class T> struct has_type<T, void_t<typename T::type>> { static constexpr bool value = true; };
static_assert(has_type<Yes>::value);
static_assert(!has_type<No>::value);
static_assert(!has_type<int>::value);

// The same shape with the detected class itself a template argument:
// what _Is_default_allocator asks of allocator<T>::from_primary.
template <class T> struct alloc2 { using from_primary = alloc2; };
template <class A, class = void> struct is_default : same<int, long> {};
template <class T> struct is_default<alloc2<T>, void_t<typename alloc2<T>::from_primary>> : same<int, int> {};
static_assert(is_default<alloc2<int>>::value);
static_assert(!is_default<int>::value);
static_assert(!is_default<Yes>::value);

// A trait's arguments: a pack expands into the argument list, and the
// trait in template-argument position is an expression, not a type.
template <class T, T v> struct integral_constant { static constexpr T value = v; };
template <bool B> using bool_constant = integral_constant<bool, B>;
template <class T, class... A>
struct nothrow_constructible : bool_constant<__is_nothrow_constructible(T, A...)> {};
static_assert(nothrow_constructible<int, int>::value);
static_assert(nothrow_constructible<int>::value);
static_assert(!nothrow_constructible<int, Yes>::value);
static_assert(!nothrow_constructible<Yes, int>::value);
static_assert(nothrow_constructible<Yes, Yes>::value);

// A function template declared and not defined still takes part in
// overload resolution by its substituted signature.
template <class T> T* declared_only(T&);
static_assert(same<decltype(declared_only(*(int*)0)), int*>::value);
