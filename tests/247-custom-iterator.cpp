// A custom random-access iterator used with std::stable_sort, std::lower_bound and reverse iteration.
#include <algorithm>
#include <cstddef>
#include <cstdio>
#include <iterator>

struct Rec {
    int key;
    char tag;
};

class Table {
public:
    struct iterator {
        using iterator_category = std::random_access_iterator_tag;
        using value_type = Rec;
        using difference_type = std::ptrdiff_t;
        using pointer = Rec*;
        using reference = Rec&;
        Rec* p;
        Rec& operator*() const { return *p; }
        Rec* operator->() const { return p; }
        Rec& operator[](difference_type n) const { return p[n]; }
        iterator& operator++() { ++p; return *this; }
        iterator operator++(int) { iterator t = *this; ++p; return t; }
        iterator& operator--() { --p; return *this; }
        iterator operator--(int) { iterator t = *this; --p; return t; }
        iterator& operator+=(difference_type n) { p += n; return *this; }
        iterator& operator-=(difference_type n) { p -= n; return *this; }
        friend iterator operator+(iterator i, difference_type n) { return {i.p + n}; }
        friend iterator operator+(difference_type n, iterator i) { return {i.p + n}; }
        friend iterator operator-(iterator i, difference_type n) { return {i.p - n}; }
        friend difference_type operator-(iterator a, iterator b) { return a.p - b.p; }
        friend auto operator<=>(iterator a, iterator b) = default;
    };
    iterator begin() { return {rows_}; }
    iterator end() { return {rows_ + 8}; }

private:
    Rec rows_[8] = {{3, 'a'}, {1, 'b'}, {3, 'c'}, {2, 'd'}, {1, 'e'}, {3, 'f'}, {2, 'g'}, {0, 'h'}};
};

int main() {
    Table t;
    std::stable_sort(t.begin(), t.end(), [](const Rec& a, const Rec& b) { return a.key < b.key; });
    for (const Rec& r : t) std::printf("%c", r.tag);
    auto it = std::lower_bound(t.begin(), t.end(), 2, [](const Rec& r, int k) { return r.key < k; });
    std::printf(" %td ", it - t.begin());
    for (auto r = std::make_reverse_iterator(t.end()); r != std::make_reverse_iterator(t.begin()); ++r)
        std::printf("%d", r->key);
    std::printf("\n");
    return 0;
}
