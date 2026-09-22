// <type_traits>: a handful of queries and transformations.
#include <cstdio>
#include <type_traits>

struct S {};
struct D : S {};

int main() {
    std::printf("%d %d %d %d %d\n", std::is_same_v<int, int>, std::is_pointer_v<int*>,
                std::is_base_of_v<S, D>, std::is_same_v<std::remove_const_t<const int>, int>,
                std::is_same_v<std::decay_t<int[3]>, int*>);
    std::printf("%d %d\n", std::is_trivially_copyable_v<S>, std::is_signed_v<unsigned>);
    return 0;
}
