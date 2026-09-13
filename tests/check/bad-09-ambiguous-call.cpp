// §12.2.4 [over.match.best]/2 -- neither candidate is better than the other,
// because each wins on one argument.
void f(int, double);
void f(double, int);
void g() { f(1, 2); }
