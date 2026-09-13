// §10 [module] and §9.5.4 [dcl.fct.def.coroutine] -- the two clauses whose
// grammar exists ahead of the semantics that will implement them.
//
// A module unit's grammar is a whole-file shape, so what a single file can
// hold is the export-declaration and the two import forms. The interface
// unit itself, its partitions, and the global-module fragment belong to a
// corpus that compiles more than one file, and are not here.

// §10.3 [module.import] -- an import is a declaration, and its operand is a
// module-name, a partition, or a header-name.
import already.declared;
import :partition;

// §10.2 [module.interface] -- export applies to one declaration or a block.
export int exported_variable;
export void exported_function();
export struct ExportedType { int m; };
export template <typename T> struct ExportedTemplate {};
export namespace ExportedNS { int inside; }
export using ExportedAlias = int;
export {
    int a;
    void b();
    struct C {};
}

// §9.5.4 [dcl.fct.def.coroutine] -- a function is a coroutine because its
// body holds one of these, which is why the grammar is expression-level and
// there is no keyword on the declaration.
struct Task { struct promise_type; };
struct Awaitable {};

Task co_returns() {
    co_return;
}

Task co_returns_value() {
    co_return 1;
}

Task co_awaits(Awaitable a) {
    co_await a;
    int v = co_await a;
    (void)v;
    co_return;
}

Task co_yields() {
    co_yield 1;
    co_yield 1 + 2;
    co_return;
}

Task in_control_flow(Awaitable a, int n) {
    for (int i = 0; i < n; ++i) {
        if (i) {
            co_yield i;
        } else {
            co_await a;
        }
    }
    co_return;
}

// co_await binds tighter than a binary operator and looser than a postfix
// one, §7.6.2.4, so this is `(co_await a) + 1` and parses as one expression.
Task precedence(Awaitable a) {
    (void)(co_await a);
    co_return;
}
