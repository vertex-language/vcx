// Does a name inside namespaces spell its scopes the way cl does?
//
// Innermost first, each a back-reference candidate. `outer::inner::f` puts
// `inner` in slot 1 and `outer` in slot 2, and a class in `outer` used as a
// parameter refers to `outer` by its digit rather than spelling it again.

namespace outer {
    struct Thing { int v; };
    void f(Thing) {}
    namespace inner {
        struct Thing { int w; };
        void f(Thing) {}
        void g(outer::Thing, Thing) {}
        int h(int) { return 0; }
    }
    void uses_inner(inner::Thing) {}
}

void global(outer::Thing, outer::inner::Thing) {}

namespace a { namespace b { namespace c { namespace d { void deep(int) {} } } } }
