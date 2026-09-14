// Is `auto*` deduced by matching the pointer, not by wrapping it?
//
// §9.2.9.7.2 [dcl.type.auto.deduct] -- a placeholder is deduced as a
// template parameter would be: `auto* p = q` with q an S* matches P = auto*
// against A = S*, so auto is S and p is an S*. Substituting the whole
// initializer type for auto made p an S**, and every `auto* x =
// static_cast<T*>(...)` a conversion error.

struct S { int x; };

int main() {
    S s{5};
    S* q = &s;
    auto* p = q;
    const auto* c = q;
    auto* d = static_cast<S*>(q);
    S* arr[2] = {q, q};
    int n = 0;
    for (auto* e : arr)
        n += e->x;
    return p->x + c->x + d->x + n;   // 5 + 5 + 5 + 10 = 25
}
