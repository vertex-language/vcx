// §11.7.2 [class.mi] -- a virtual base is shared by every path that reaches
// it, which is what makes its offset unknown when the base is compiled.
//
// A non-virtual base's offset is fixed once and for all. A virtual base's
// depends on the *complete* object, so a class deriving from it cannot know
// where it is: the offset is read at run time out of a table, and the
// pointer to that table is the {vbptr}. This file asks where the shared
// subobject went and what the table says about reaching it -- the second
// being the subtlest thing to get wrong, because an object can be the right
// size with every member at the right offset and a cast to the virtual base
// still land somewhere else.

struct B { int b; };
struct C { int c; };
struct Wide { double d; };
struct Empty {};

// One virtual base: the pointer first, then the members, then the base last.
struct One : virtual B { int v; };
struct OneChar : virtual B { char v; };
struct OneWide : virtual Wide { int v; };
struct NoMembers : virtual B {};

// Two, in declaration order after everything else.
struct Two : virtual B, virtual C { int v; };
struct TwoWide : virtual B, virtual Wide { int v; };

// A non-virtual base beside a virtual one, in both orders. The pointer goes
// after the non-virtual bases and before this class's own members.
struct Mixed : B, virtual C { int v; };
struct MixedOther : virtual C, B { int v; };
struct TwoNonVirtual : B, C, virtual Wide { int v; };

// A polymorphic class with a virtual base carries both pointers, and the
// order between them is the question.
struct PolyV : virtual B { virtual void f(); int p; };
struct PolyVTwo : virtual B, virtual C { virtual void f(); int p; };
struct PolyBase { virtual void g(); int q; };
struct PolyAndVirtual : PolyBase, virtual B { int m; };

// The diamond. Both Left and Right derive virtually from B, so Diamond has
// one B -- and two virtual base pointers, one per base, neither of them its
// own.
struct Left : virtual B { int l; };
struct Right : virtual B { int r; };
struct Diamond : Left, Right { int d; };
struct DiamondNoMembers : Left, Right {};

// A class deriving from one that already has a virtual base adds no pointer
// of its own; it uses the one it inherited.
struct DerivesOne : One { int e; };
struct DerivesDiamond : Diamond { int e; };

// A virtual base that is empty still has an address.
struct VirtualEmpty : virtual Empty { int m; };

// §11.7/1 -- the same class reached once virtually and once not is two
// subobjects, not one: only the virtual paths share. `struct X : B, virtual
// B` is ill-formed (a direct base named twice), so the non-virtual path goes
// through another class.
struct HasB : B {};
struct BothWays : HasB, virtual B { int m; };
