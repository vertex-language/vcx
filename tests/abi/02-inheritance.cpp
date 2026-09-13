// Where a base subobject sits inside its derived class, which is the part of
// layout a size does not reveal: two compilers can agree on how big a class
// is and disagree about where its base begins, and only a call through a
// base pointer ever finds out.

struct Empty {};
struct One { int a; };
struct Two { int a; int b; };
struct Char { char c; };
struct Wide { double d; };

// Single inheritance: the base first, then the derived members after it.
struct D1 : One { int d; };
struct D2 : Two { int d; };
struct D3 : Char { char d; };

// The base's own alignment moves the derived members, and the derived's
// alignment sizes the whole.
struct D4 : Char { int d; };
struct D5 : One { double d; };
struct D6 : Wide { char d; };

// Tail padding: whether a derived member may sit inside the padding the base
// left is the question every ABI answers differently, and the reason this
// case is here rather than assumed.
struct HasTailPadding { int i; char c; };
struct FillsTailPadding : HasTailPadding { char d; };

// A chain of them, so an offset is the sum of several rather than one.
struct Chain1 : One { int b; };
struct Chain2 : Chain1 { int c; };
struct Chain3 : Chain2 { int d; };

// §11.7/5 -- multiple bases are laid out in the order they are declared, and
// each is padded to its own alignment.
struct M1 : One, Two { int m; };
struct M2 : Char, One { int m; };
struct M3 : One, Char, Wide { int m; };

// §11.7/8 [class.derived] -- an empty base takes no space of its own, which
// is what makes a policy class free. It still has an address, so two of the
// same type cannot share one.
struct EBO1 : Empty { int m; };
struct EBO2 : Empty, One { int m; };
struct EmptyToo {};
struct EBO3 : Empty, EmptyToo { int m; };
struct EmptyChain : Empty {};
struct EBO4 : EmptyChain { int m; };

// A class with no members of its own is still as big as its base.
struct NoMembers : Two {};
struct NoMembersEmpty : Empty {};
