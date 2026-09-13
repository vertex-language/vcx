// The other half of 07: see a.cpp.

struct One { unsigned char c; };
struct Two { short s; };
struct Four { int i; };
struct Pair { short a, b; };

One make_one(int c) { One o; o.c = (unsigned char)c; return o; }
Two make_two(int s) { Two t; t.s = (short)s; return t; }
Four make_four(int i) { Four f; f.i = i; return f; }
Pair make_pair(int a, int b) { Pair p; p.a = (short)a; p.b = (short)b; return p; }
int sum_one(One o) { return o.c; }
int sum_two(Two t) { return t.s; }
int sum_four(Four f) { return f.i; }
int sum_pair(Pair p) { return p.a + p.b; }
