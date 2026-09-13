// §7.6.2.3 [expr.pre.incr]/1 -- the built-in ++ takes an arithmetic
// operand other than bool, or a pointer. A class without operator++ has
// no increment.
struct Counter { int n; };
void bump(Counter c) { ++c; }
