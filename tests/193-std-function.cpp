// std::function holds any callable with the right signature.
#include <cstdio>
#include <functional>

int triple(int v) { return v * 3; }

struct Offset {
    int k;
    int operator()(int v) const { return v + k; }
};

int main() {
    std::function<int(int)> fs[] = {triple, Offset{100}, [](int v) { return -v; }};
    int v = 2;
    for (auto& f : fs) v = f(v);
    std::function<int(int)> empty;
    std::printf("%d %d\n", v, static_cast<bool>(empty));
    return 0;
}
