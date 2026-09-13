// §8.5/2 [stmt.pre] -- a condition is contextually converted to bool, and a
// class with no conversion to bool cannot be.
struct Opaque { int m; };
void f(Opaque o) {
    if (o) { }
}
