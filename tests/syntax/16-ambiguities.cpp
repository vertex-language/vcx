// The places where two productions match the same tokens, and the rule that
// picks one. These are the lines a C++ parser gets wrong, so each is here
// with the paragraph that decides it.

struct S { S(); S(int); int m; };
using T = int;
int x;
int f(int);

// §13.3/3 [temp.names] -- `>>` closes two template-argument-lists rather
// than shifting, and `>=` likewise splits.
template <typename A> struct Tmpl { using type = A; };
Tmpl<Tmpl<int>> two_lists;
Tmpl<Tmpl<Tmpl<int>>> three_lists;

// §5.4/3 [lex.pptoken] -- but `<::` is `<` then `::` and not `<:` (the
// digraph for `[`), when what follows makes a nested-name-specifier.
namespace N { struct Inner {}; }
Tmpl<::N::Inner> digraph_exception;

// §7.6.1.9/2 -- `sizeof` binds to a unary-expression, so parentheses may
// belong to the operand or to a type-id, and the two are different.
auto s1 = sizeof(int);      // type-id
auto s2 = sizeof x;         // unary-expression
auto s3 = sizeof(x);        // parenthesized expression, not a type
auto s4 = sizeof(T);        // type-id again -- T is a type
auto s5 = sizeof(int*);
auto s6 = sizeof(int (*)(int));

// §7.6.3/2 [expr.cast] -- a parenthesized name is a cast if the name is a
// type and a parenthesized expression otherwise, and the following token
// does not disambiguate it.
auto c1 = (T)x;
auto c2 = (T)+x;            // cast of unary plus, not a binary add
auto c3 = (x) + x;          // x is an object, so this is addition
auto c4 = (int)(x);
auto c5 = (int*)0;
auto c6 = (int (*)(int))0;

// The parser has no type table, so it cannot ask the question §7.6.3/2
// answers -- whether the name is a type. What it can ask is whether the
// parentheses could hold a type-id at all, and these could not: a type-id
// has no conditional in it and no arithmetic. Each of these is therefore a
// parenthesized expression followed by a binary operator, not a cast of the
// unary one.
int fn2(int, int);
auto c7 = fn2(1, 2) + (fn2(3, 4) ? 1 : 0) + x;
auto c8 = (x + x) * x;
auto c9 = (fn(x)) - x;

// §8.9/2 [stmt.ambig] -- in a statement, anything that can be a declaration
// is one. The `(x)` around the name does not change that.
void statements() {
    T (a);                  // declares a, not a cast of x to T
    T *b;                   // declares a pointer, not a multiplication
    T &c = x;               // declares a reference, not a bitwise and
    (void)a; (void)b; (void)c;
}

// §9.3.4.6/2 [dcl.ambig.res] -- the same rule at namespace scope, where the
// vexing parse lives. Every line below declares a function.
S vexing1();
S vexing2(S);
S vexing3(S (*)());

// §9.2.9.6 -- an elaborated-type-specifier names the class even where a
// variable of the same name is in scope, which is what `struct` is for.
struct Shadowed {};
int Shadowed;
struct Shadowed shadowed_object;

// §13.8.3.2 [temp.res] -- a dependent qualified name is not a type unless
// `typename` says so, and a dependent template is not one unless `template`
// does. Neither is deducible from the tokens.
template <typename A> struct Dependent {
    typename A::member_type as_a_type;
    void as_an_expression() { A::member_type; }
    void as_a_template() { A::template member<int>(); }
    void through_this() { this->template member<int>(); }
};

// §7.5.5/3 -- a lambda-introducer and an attribute-specifier both begin
// `[[`, and only the second bracket pair decides which.
auto lam = [](int v) { return v; };
auto nested_lam = [](int v) { return [v] { return v; }; };
[[maybe_unused]] auto after_attribute = 1;

// §7.6.1.2/1 -- a subscript and an attribute both use brackets, and a
// lambda may be the subscript.
int arr[4];
auto sub = arr[0];
