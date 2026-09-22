// Tag dispatch on iterator categories.
#include <cstdio>
#include <iterator>
#include <list>
#include <vector>

template <typename It>
int distance_impl(It a, It b, std::random_access_iterator_tag) {
    std::printf("random access: ");
    return (int)(b - a);
}

template <typename It>
int distance_impl(It a, It b, std::input_iterator_tag) {
    std::printf("walked: ");
    int n = 0;
    for (; a != b; ++a) ++n;
    return n;
}

template <typename It>
int my_distance(It a, It b) {
    return distance_impl(a, b, typename std::iterator_traits<It>::iterator_category{});
}

int main() {
    std::vector<int> v(7);
    std::list<int> l(4);
    std::printf("%d\n", my_distance(v.begin(), v.end()));
    std::printf("%d\n", my_distance(l.begin(), l.end()));
    return 0;
}
