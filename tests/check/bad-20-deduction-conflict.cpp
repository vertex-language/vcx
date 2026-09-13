// §13.10.3/5 [temp.deduct] -- two deductions for one parameter must agree.
// `T` cannot be both int and double, so the call has no viable candidate.
template <typename T> T pick(T a, T b) { return a; }
int f() { return pick(1, 2.0); }
