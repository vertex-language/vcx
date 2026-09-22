// A thread_local variable, in the one thread there is.
#include <cstdio>

thread_local int calls = 0;

int next_id() { return ++calls; }

struct Tls {
    int v = 7;
};
thread_local Tls obj;

int main() {
    next_id();
    next_id();
    obj.v += next_id();
    std::printf("%d %d\n", calls, obj.v);
    return 0;
}
