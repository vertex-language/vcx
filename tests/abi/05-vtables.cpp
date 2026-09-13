// §11.7.3 [class.virtual] -- which slot each virtual function lands in, and
// which definition that slot holds.
//
// This is the half of the ABI a layout does not show. A wrong offset reads
// the wrong field; a wrong slot calls the wrong function, and neither is
// visible in a class's size. The slot numbers below are cl's, and vcx has to
// arrive at the same ones from the same declarations.

// Slots are numbered in declaration order, and a derived class's own
// virtuals are appended after its base's.
struct A { virtual void f(); virtual void g(); virtual void h(); };
struct AddsOne : A { virtual void extra(); };
struct AddsTwo : A { virtual void one(); virtual void two(); };

// An override keeps its introducing class's slot, and the slot then holds
// the derived definition -- which is the whole mechanism.
struct OverridesFirst : A { void f() override; };
struct OverridesLast : A { void h() override; };
struct OverridesAll : A { void f() override; void g() override; void h() override; };

// Overriding and adding at once, in both orders.
struct BothWays : A { void g() override; virtual void added(); };

// Three deep: a slot introduced at the top, overridden in the middle, and
// overridden again at the bottom.
struct Mid : A { void f() override; virtual void midOnly(); };
struct Low : Mid { void f() override; void midOnly() override; };

// The `virtual` keyword is redundant on an override and is usually left off.
// A member that overrides one is virtual either way, §11.7.3/2.
struct NoKeyword : A { void f(); };
struct WithKeyword : A { virtual void f(); };

// A destructor takes a slot like anything else, and every class's overrides
// the base's however differently it is spelled.
struct HasDtor { virtual void f(); virtual ~HasDtor(); };
struct InheritsDtor : HasDtor { void f() override; };
struct OwnDtor : HasDtor { ~OwnDtor() override; };

// §11.7.3/4 -- a pure virtual still occupies a slot; what changes is what is
// in it, not whether it is there.
struct Abstract { virtual void pure() = 0; virtual void concrete(); };
struct Implements : Abstract { void pure() override; };
struct StillAbstract : Abstract { void concrete() override; };

// An overload of the name a slot holds is not an override, so it takes a
// slot of its own -- the signature is what decides, not the name.
struct Overloaded { virtual void f(int); virtual void f(double); };
struct PicksOne : Overloaded { void f(int) override; };

// A const member function and a non-const one of the same name are two
// different virtuals, for the same reason.
struct ConstQualified { virtual void f(); virtual void f() const; };

// Multiple inheritance: one table per polymorphic base. What the derived
// class introduces goes in the first table; the second holds only what its
// base declared, reached through an adjusted `this`.
struct P1 { virtual void p(); int x; };
struct P2 { virtual void q(); int y; };
struct P3 { virtual void r(); int z; };
struct TwoBases : P1, P2 { void p() override; void q() override; virtual void own(); };
struct ThreeBases : P1, P2, P3 { void q() override; };

// A polymorphic base beside a plain one, in both orders -- the plain one
// contributes no table however it is ordered.
struct Plain { int m; };
struct PolyFirst : P1, Plain { void p() override; };
struct PlainFirst : Plain, P1 { void p() override; };
