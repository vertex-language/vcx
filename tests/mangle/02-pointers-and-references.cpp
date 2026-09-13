// Do pointers, references and their cv-qualifiers spell the way cl spells them?
//
// A 64-bit pointer carries an `E`; a pointer to a function does not. The
// pointee's cv goes after that letter and the pointer's own replaces the
// `P`. An array parameter is a pointer (§9.3.4.6/5), and top-level const on
// a parameter is not part of the function type at all, so `k(const int)`
// and `k(int)` would be one function -- the file declares only one of them.

void p(int *) {}
void pc(const int *) {}
void pv(volatile int *) {}
void pcv(const volatile int *) {}
void cp(int *const) {}
void pp(int **) {}
void pcp(const int *const *) {}
void r(int &) {}
void rc(const int &) {}
void rr(int &&) {}
void rrc(const int &&) {}
void fp(void (*)(int)) {}
void fpr(int (*)(double, char)) {}
void fr(void (&)(int)) {}
void arr(int[3]) {}
void arr2(int[3][4]) {}
void parr(int (*)[3]) {}
void rarr(int (&)[3]) {}
void rarr2(const int (&)[2][5]) {}
void k(const int) {}
void cpp(char *, const char *, char *const, const char *const) {}
void vp(void *, const void *) {}
int *ret_p() { return nullptr; }
const int &ret_r(const int &x) { return x; }
