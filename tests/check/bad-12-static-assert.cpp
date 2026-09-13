// §9.1 [dcl.pre]/6 -- a static_assert whose condition is false, with and
// without the message that replaces the default one.
static_assert(2 + 2 == 5, "math is broken");
static_assert(sizeof(char) == 2);
