// A global initialized by a function call, before main.
#include <cstdio>

int compute() { return 6 * 7; }
int answer = compute();
int twice = answer * 2;

int main() {
    std::printf("%d %d\n", answer, twice);
    return 0;
}
