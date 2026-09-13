// Does <new> compile, and does placement new construct where it is told?
//
// The header declares the allocation functions the library calls and
// defines the placement forms inline: `new (p) T` is a call to the
// operator that returns its argument, and the constructor runs on that
// storage. The runtime's exception types come with it, through
// vcruntime_exception.h, and are declared but not used here: throwing is
// not lowered yet.

#include <new>

struct Counter {
    int n;
    Counter(int start) : n(start) {}
    int next() { return ++n; }
};

int main() {
    alignas(Counter) unsigned char storage[sizeof(Counter)];
    Counter* c = new (storage) Counter(40);
    c->next();
    c->next();
    return c->n;   // 42
}
