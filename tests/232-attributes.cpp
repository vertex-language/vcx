// Standard attributes: nodiscard on a function and a type, maybe_unused, likely and unlikely.
#include <cstdio>

[[nodiscard]] int compute(int v) { return v * 3; }

struct [[nodiscard]] Status {
    int code;
};
Status check(int v) { return {v % 2}; }

int classify(int v) {
    if (v > 0) [[likely]]
        return 1;
    else [[unlikely]]
        return -1;
}

int main() {
    [[maybe_unused]] int unused = 5;
    int r = compute(4);
    Status s = check(3);
    std::printf("%d %d %d %d\n", r, s.code, classify(10), classify(-2));
    return 0;
}
