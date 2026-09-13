// Does a class travel by value -- copied in, copied out?
//
// §11.4.5.3 -- the copy constructor makes the new object from the old one.
// Both of these have trivial copies, so the object is memcpy'd, but how the
// bytes travel is the ABI's decision: a class that fits a register goes in
// one on x64, a bigger one goes by hidden pointer, and a returned one comes
// back through a caller-supplied slot. The corpus does not know which; it
// knows only that the values arrive.

struct Small { int a, b; };
struct Big { long long v[4]; };

Small make(int a, int b) { Small s; s.a = a; s.b = b; return s; }
int sum(Small s) { return s.a + s.b; }

Big widen(Small s) { Big g; g.v[0] = s.a; g.v[1] = s.b; g.v[2] = 0; g.v[3] = 1; return g; }
long long total(Big g) { return g.v[0] + g.v[1] + g.v[2] + g.v[3]; }

int main() {
    Small s = make(3, 4);
    Small t = s;
    t.a = 10;
    return sum(s) + sum(t) + (int)total(widen(s));   // 7 + 14 + 8
}
