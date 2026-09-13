// Do the back-reference tables behave like cl's past the tenth entry?
//
// Ten slots for names and ten for parameter types, and the eleventh of
// either is spelled in full. A type written as one letter never takes a
// slot; a two-letter one -- `_J`, `_N` -- does, which is why the second
// `long long` in a list is a digit.

struct A {}; struct B {}; struct C {}; struct D {}; struct E {};
struct F {}; struct G {}; struct H {}; struct I {}; struct J {}; struct K {};

void names(A, B, C, D, E, F, G, H, I, J, K, A, B, K) {}
void types(A *, B *, C *, D *, E *, F *, G *, H *, I *, J *, K *, A *, K *, A *) {}
void twice(long long, long long, bool, bool, long long) {}
void mixed(int, A, int, A, const A &, const A &, A *, A *, A) {}
void refs(int &, int &, const int &, const int &, int *, int *) {}
