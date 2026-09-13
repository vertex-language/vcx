// Does <type_traits> compile, and do its traits answer?
//
// The header is the library's exercise of the template machinery: variable
// templates and their partial specializations (is_pointer_v<T*>), default
// template arguments (enable_if<B, T = void>), packs (void_t<...>,
// conjunction<...>), array patterns binding a bound (extent<T[N]>), alias
// templates, and static_asserts that wait for an instantiation to be
// checked. The program asks a trait of each kind and adds the answers up
// through the two spellings the header offers: the class's ::value and the
// _v variable.

#include <type_traits>

struct S { int x; };

template <class T, std::enable_if_t<std::is_integral_v<T>, int> = 0>
int only_integral(T) { return 1; }

int main() {
    int n = 0;
    n += std::is_pointer_v<int*>;                         // 1
    n += std::is_pointer<int>::value;                     // 0
    n += std::is_same_v<std::remove_cv_t<const int>, int>; // 1
    n += std::extent_v<int[3][4]> + std::extent_v<int[3][4], 1>; // 7
    n += std::rank_v<int[1][2][3]>;                        // 3
    n += std::is_void_v<std::void_t<S, int>>;              // 1
    n += std::conjunction_v<std::is_integral<int>, std::is_signed<int>>; // 1
    n += std::is_class_v<S> * 10;                          // 10
    n += only_integral(4);                                 // 1
    n += std::is_array_v<int[]> + std::is_unbounded_array_v<int[]>; // 2
    return n;                                              // 27
}
