// §7.3 [conv] -- the standard conversion sequences that must be accepted
// without a diagnostic.
void takes_int(int);
void takes_double(double);
void takes_bool(bool);
void takes_cptr(const int*);
void takes_voidptr(const void*);

struct Base {}; struct Derived : Base {};
void takes_base_ptr(Base*);
void takes_base_ref(const Base&);

void conversions(int i, double d, float f, char c, bool b, int* p, Derived* dp) {
    takes_int(c);           // integral promotion
    takes_int(b);
    takes_double(i);        // integral -> floating
    takes_double(f);        // floating promotion
    takes_bool(i);          // boolean conversion
    takes_bool(p);
    takes_cptr(p);          // qualification conversion
    takes_voidptr(p);       // pointer conversion
    takes_base_ptr(dp);     // derived-to-base
    takes_base_ref(*dp);
    takes_int(d);           // narrowing is allowed outside a braced list
    (void)i; (void)d;
}

// §9.4.5 -- an explicit conversion is always allowed where an implicit one is.
int explicit_forms(double d, Base* b) {
    int a = static_cast<int>(d);
    Derived* dp = static_cast<Derived*>(b);
    const int* cp = const_cast<const int*>(&a);
    (void)dp; (void)cp;
    return a;
}
