// §9.2.4 [dcl.typedef], §9.2.4 [dcl.typedef]/3, §13.7.8 [temp.alias]
// Valid typedefs, duplicate typedef to same type, using aliases, and alias templates.

typedef int IntAlias;
typedef int IntAlias; // §9.2.4/3 -- duplicate typedef to same type is well-formed

using FloatAlias = float;
template<typename T> using Ptr = T*;

struct S {
    using value_type = int;
    typedef value_type* pointer;
};

void test_aliases() {
    IntAlias a = 42;
    FloatAlias b = 3.14f;
    Ptr<int> p = &a;
    S::value_type v = 10;
    S::pointer sp = &v;
}
