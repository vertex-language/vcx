// std::optional: present, absent, value_or.
#include <cstdio>
#include <optional>

std::optional<int> parse_digit(char c) {
    if (c >= '0' && c <= '9') return c - '0';
    return std::nullopt;
}

int main() {
    auto a = parse_digit('7');
    auto b = parse_digit('x');
    std::printf("%d %d %d %d\n", a.has_value(), *a, b.has_value(), b.value_or(-1));
    return 0;
}
