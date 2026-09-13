// §13.10.3/1 -- a candidate whose deduction fails is not viable, so a
// pointer parameter with a non-pointer argument leaves nothing to call.
template <typename T> T dereference(T* p) { return *p; }
int f() { int a = 1; return dereference(a); }
