// §11.7.3 [class.virtual] -- a polymorphic class carries a pointer to its
// virtual table, and where that pointer sits is the first thing the two ABIs
// disagree about. Itanium puts it at offset zero and reuses a base's;
// Microsoft puts it at zero too but does not merge one from a second base,
// so a class with two polymorphic bases carries two.
//
// What is compared here is the layout that follows from the pointer, not the
// table itself. The table's contents -- which slot each override lands in --
// is what a call through a base pointer reads, and it needs a dump this
// compiler does not yet produce.

struct Plain { int a; };
struct Poly { virtual void f(); int a; };
struct PolyOnly { virtual void f(); };
struct PolyDtor { virtual ~PolyDtor(); int a; };
struct TwoVirtuals { virtual void f(); virtual void g(); int a; };

// A derived class inherits the pointer rather than adding one.
struct DerivedPoly : Poly { int b; };
struct OverridesOne : Poly { void f() override; int b; };
struct AddsAVirtual : Poly { virtual void g(); int b; };

// A polymorphic class derived from a plain one, and the reverse.
struct PolyFromPlain : Plain { virtual void f(); int b; };
struct PlainFromPoly : Poly { int b; };

// Two polymorphic bases: whether the second's pointer is merged away is the
// question, and it is asked rather than assumed.
struct PolyA { virtual void a(); int x; };
struct PolyB { virtual void b(); int y; };
struct TwoPolyBases : PolyA, PolyB { int z; };

// A polymorphic base and a plain one, in both orders.
struct PolyThenPlain : PolyA, Plain { int z; };
struct PlainThenPoly : Plain, PolyA { int z; };

// An empty polymorphic class is not empty: it has the pointer.
struct EmptyPoly { virtual void f(); };
struct DerivesEmptyPoly : EmptyPoly { int m; };

// The alignment a pointer imposes moves everything after it.
struct PolyWithChar { virtual void f(); char c; };
struct PolyWithDouble { virtual void f(); double d; };
struct PolyWithMixed { virtual void f(); char c; int i; double d; };
