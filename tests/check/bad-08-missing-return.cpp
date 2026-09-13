// §9.5.1 [dcl.fct.def.general]/2 -- flowing off the end of a value-returning
// function is undefined; vcx diagnoses the path rather than emitting it.
int f(int a) {
    if (a > 0) return 1;
}
