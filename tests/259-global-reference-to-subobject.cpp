// A reference at namespace scope bound to a global, to a member of one, to an
// element of an array, to another reference, and to a base subobject: each
// reads what it was bound to and writes through to it.
#include <cstdio>

struct Inner { long a; long b; };
struct Outer { const char* name; Inner inner; int list[3]; };
struct Left { long l = 7; };
struct Right { long r = 8; };
struct Both : Left, Right { long own = 9; };

namespace {
Outer outer = {"outer", {1, 2}, {10, 20, 30}};
Both both;
long plain = 5;
}

long& toPlain = plain;
const Inner& toMember = outer.inner;
long& toDeep = outer.inner.b;
int& toElement = outer.list[1];
long& toReference = toDeep;
Right& toBase = both;
long made() { return 41; }
const long& toTemporary = made();

int main() {
    printf("%ld %ld %ld %d %ld %ld %ld\n", toPlain, toMember.a, toDeep, toElement, toReference, toBase.r, toTemporary);
    toPlain = 50;
    toDeep = 60;
    toElement = 70;
    toBase.r = 80;
    printf("%ld %ld %d %ld %ld\n", plain, outer.inner.b, outer.list[1], both.r, toMember.b);
    printf("%d\n", &toMember == &outer.inner && &toDeep == &toReference);
    return 0;
}
