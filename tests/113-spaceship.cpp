// operator<=> and the comparisons rewritten from it.
#include <compare>
#include <cstdio>

struct Version {
    int major, minor;
    auto operator<=>(const Version&) const = default;
};

int main() {
    Version a{1, 4}, b{1, 10}, c{1, 4};
    std::printf("%d %d %d %d %d\n", a < b, a == c, b > c, a != b, (a <=> b) < 0);
    return 0;
}
