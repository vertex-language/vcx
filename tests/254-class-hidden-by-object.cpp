// [basic.scope.hiding]/2: a class and an object (or function) of the
// same name, declared in either order; the object hides the class, and
// an elaborated-type-specifier still finds it. Darwin's <sys/time.h> has
// `struct timezone` before <_time.h>'s `extern long timezone`.
#include <cstdio>
struct S { int a; };
extern long S;
long S = 7;
long V = 2;
struct V { int c; };
struct T { int b; };
int T(int x) { return x + 1; }
int main() {
  struct S s = {3};
  struct T t = {4};
  struct V v = {5};
  std::printf("%ld %d %d %ld %d\n", S, s.a, T(t.b), V, v.c);
  return 0;
}
