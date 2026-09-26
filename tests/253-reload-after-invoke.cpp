// A member read, an assignment to the object through a call, and the
// member read again, with destructors live: the calls are invokes, and
// the second read must see what the assignment stored.
#include <cstdio>
struct S {
  int tag; int *p;
  S(int t, int *q) : tag(t), p(q) {}
  S(const S &o) : tag(o.tag), p(o.p) {}
  S &operator=(const S &o) { tag = o.tag; p = o.p; return *this; }
  ~S() {}
};
static int one = 1, two = 2;
static S make(int n) { return S(n, n == 1 ? &one : &two); }
static int get(int *p) { return *p; }
int main() {
  {
    S a = make(1);
    S b = a;
    std::printf("b holds %d\n", get(b.p));
    b = make(2);
    std::printf("b now %d\n", get(b.p));
  }
  return 0;
}
