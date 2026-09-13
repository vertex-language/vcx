// §6.5.5 [class.qual] -- a name qualified by a class that does not declare it.
struct S { int present; };
int f() { return S::absent; }
