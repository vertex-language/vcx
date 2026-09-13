// §7.6.10 [expr.eq]/1 -- the built-in == takes arithmetic, enumeration
// and pointer operands. A class with no operator== of its own, and none
// found by §12.2.2.3's rewriting, cannot be compared: this is the
// diagnostic a requires-expression asking `a == b` is answered by, so it
// must be one.
struct NoCmp { int v; };
bool same(NoCmp a, NoCmp b) { return a == b; }
