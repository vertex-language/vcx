void f(int);
void f(double);
void f(int, int);
void f(const char*);

void calls() {
    f(1);           // exact
    f(1.0);         // exact
    f('c');         // promotion to int beats conversion to double
    f(1.0f);        // promotion to double
    f(1, 2);        // arity
    f("literal");
}

int g(int a) { return a; }
int g(int a, int b) { return a + b; }
int uses = g(1) + g(1, 2);
