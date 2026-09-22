// if consteval picks a path by whether evaluation is at compile time.
#include <cstdio>

constexpr int where() {
    if consteval {
        return 1;
    } else {
        return 2;
    }
}

int main() {
    constexpr int compile_time = where();
    int run_time = where();
    std::printf("%d %d\n", compile_time, run_time);
    return 0;
}
