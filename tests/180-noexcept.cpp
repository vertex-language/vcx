// The noexcept operator and specifier.
#include <cstdio>

void safe() noexcept {}
void unsafe() {}
template <typename T>
void maybe() noexcept(sizeof(T) < 4) {}

int main() {
    std::printf("%d %d %d %d\n", noexcept(safe()), noexcept(unsafe()), noexcept(maybe<char>()),
                noexcept(maybe<double>()));
    return 0;
}
