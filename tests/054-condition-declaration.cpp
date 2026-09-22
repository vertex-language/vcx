// A declaration as a while condition.
#include <cstdio>

int next(int& state) { return state > 0 ? state-- : 0; }

int main() {
    int state = 5, sum = 0;
    while (int v = next(state)) sum += v;
    std::printf("%d\n", sum);
    return 0;
}
