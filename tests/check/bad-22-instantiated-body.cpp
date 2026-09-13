// §13.8/1 [temp.res] -- the half of a template body that does *not* depend
// on the parameters is checked where it is written, not at instantiation.
// So an error among the non-dependent constructs is reported once, here.
template <typename T>
T uses_an_undeclared_name(T x) {
    return x + no_such_variable;
}
