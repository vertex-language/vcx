// std::expected: a value or an error, and the monadic operations over it.
#include <cstdio>
#include <expected>

enum class Err { Empty, NotDigit, Overflow };

std::expected<int, Err> parse(const char* s) {
    if (!*s) return std::unexpected(Err::Empty);
    int v = 0;
    for (; *s; ++s) {
        if (*s < '0' || *s > '9') return std::unexpected(Err::NotDigit);
        if (v > 100000) return std::unexpected(Err::Overflow);
        v = v * 10 + (*s - '0');
    }
    return v;
}

int main() {
    for (const char* s : {"123", "", "12x", "99999999"}) {
        auto r = parse(s).transform([](int v) { return v * 2; });
        if (r) std::printf("ok %d\n", *r);
        else std::printf("error %d\n", (int)r.error());
    }
    std::printf("%d\n", parse("x").value_or(-1));
    return 0;
}
