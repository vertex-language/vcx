// §9.8.2/7 [namespace.def] -- the members of an inline namespace can be
// used as though they were members of the enclosing namespace: qualified
// lookup into the enclosing one finds them, through any depth of inline
// namespaces, and so does unqualified lookup from inside it. A standard
// library that versions itself this way -- libc++ puts everything in
// std::__1 -- is written against exactly this.
//
// §6.5.5.3/2 [namespace.qual] -- qualified lookup into a namespace that
// finds nothing declared there looks next in the namespaces its
// using-directives nominate.

namespace lib {
inline namespace v1 {
    int answer() { return 42; }
    template <class T, class U> inline constexpr bool same = false;
    template <class T> inline constexpr bool same<T, T> = true;
    inline namespace detail {
        struct Deep { int n; };
    }
}
// Reopened without the keyword: still inline.
namespace v1 {
    using size = decltype(sizeof(int));
}
    // Unqualified, from the enclosing namespace.
    inline int twice() { return 2 * answer(); }
}

namespace outer {
    namespace impl { int hidden = 7; }
    using namespace impl;
}

static_assert(lib::same<int, int>);
static_assert(!lib::same<int, char>);
static_assert(sizeof(lib::size) == sizeof(sizeof(int)));

int main() {
    lib::Deep d{lib::answer()};
    lib::v1::Deep e{lib::v1::detail::Deep{1}.n};
    return d.n + e.n + lib::twice() + outer::hidden;
}
