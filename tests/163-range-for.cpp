// Range-based for over arrays, braced lists, and a class with begin and end.
#include <cstdio>
#include <initializer_list>

struct Countdown {
    int from;
    struct It {
        int v;
        int operator*() const { return v; }
        It& operator++() {
            --v;
            return *this;
        }
        bool operator!=(const It& o) const { return v != o.v; }
    };
    It begin() const { return {from}; }
    It end() const { return {0}; }
};

int main() {
    int arr[] = {1, 2, 3};
    for (int& v : arr) v *= 10;
    for (int v : arr) std::printf("%d ", v);
    for (auto v : {4, 5}) std::printf("%d ", v);
    for (int v : Countdown{3}) std::printf("%d ", v);
    std::printf("\n");
    return 0;
}
