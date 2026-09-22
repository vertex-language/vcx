// std::move and std::forward: perfect forwarding picks the right overload.
#include <cstdio>
#include <utility>

const char* take(int&) { return "lvalue"; }
const char* take(int&&) { return "rvalue"; }

template <typename T>
const char* relay(T&& v) { return take(std::forward<T>(v)); }

int main() {
    int x = 1;
    std::printf("%s %s %s\n", relay(x), relay(2), relay(std::move(x)));
    return 0;
}
