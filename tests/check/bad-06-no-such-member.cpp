// §11.4/1 [class.mem] -- a member that was not declared.
struct S { int present; };
int f(S s) { return s.absent; }
