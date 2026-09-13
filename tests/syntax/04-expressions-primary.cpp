// §7.5 [expr.prim] -- everything an expression can start with.

int g;
int fn(int);
struct S { int m; int f(); static int sm; };

void primaries(int a) {
    // §7.5.1 [expr.prim.literal] and §7.5.2 [expr.prim.paren].
    1; 'c'; 1.5; true; nullptr; "s"; (a);

    // §7.5.3 [expr.prim.this] appears in 11-classes; here is id-expression.
    // §7.5.4 [expr.prim.id]: unqualified, qualified, and the operator forms.
    a;
    ::g;
    S::sm;
    ::S::sm;

    // operator-function-id, conversion-function-id, literal-operator-id,
    // and the destructor spelling -- all id-expressions by §7.5.4.1.
    operator+;
    operator new;
    operator"" _suffix;

    // §7.5.5 [expr.prim.lambda] has a file of its own; §7.5.6 is fold, in 05.
    // §7.5.7 [expr.prim.req]: a requires-expression is a primary expression.
    // §7.6.1.2 the parenthesized aggregate of an id.
    (void)sizeof a;
    (void)sizeof(int);
    (void)alignof(int);
    (void)noexcept(a + 1);
    (void)typeid(a);
    (void)typeid(int);
}
